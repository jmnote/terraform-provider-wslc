package wslc

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-log/tflogtest"
)

type runnerFunc func(context.Context, ...string) (Result, error)

func (f runnerFunc) Run(ctx context.Context, args ...string) (Result, error) { return f(ctx, args...) }

func TestCreateArguments(t *testing.T) {
	timeout := 0
	opts := CreateContainerOptions{Image: "nginx", Name: "test", DNS: []string{"1.1.1.1", "8.8.8.8"}, Env: []string{"A=a b", "B=secret"}, Labels: map[string]string{"z": "last", "a": "first"}, StopTimeout: &timeout, Command: []string{"sh", "-c", "echo hi"}, Publish: []string{"127.0.0.1:8080:80/tcp"}}
	want := []string{"create", "--dns", "1.1.1.1", "--dns", "8.8.8.8", "--env", "A=a b", "--env", "B=secret", "--label", "a=first", "--label", "z=last", "--name", "test", "--publish", "127.0.0.1:8080:80/tcp", "--stop-timeout", "0", "nginx", "sh", "-c", "echo hi"}
	c := NewClient(runnerFunc(func(_ context.Context, args ...string) (Result, error) {
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("args = %#v; want %#v", args, want)
		}
		return Result{Stdout: []byte("id123\r\n")}, nil
	}))
	id, err := c.CreateContainer(context.Background(), opts)
	if err != nil || id != "id123" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestCreateMinimalArguments(t *testing.T) {
	c := NewClient(runnerFunc(func(_ context.Context, args ...string) (Result, error) {
		if !reflect.DeepEqual(args, []string{"create", "nginx"}) {
			t.Fatalf("unset options must not become flags: %v", args)
		}
		return Result{Stdout: []byte("id")}, nil
	}))
	if _, err := c.CreateContainer(context.Background(), CreateContainerOptions{Image: "nginx"}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMissingExecutable(t *testing.T) {
	c := NewClient(runnerFunc(func(context.Context, ...string) (Result, error) {
		return Result{ExitCode: -1}, ErrExecutableNotFound
	}))
	if _, err := c.CreateContainer(context.Background(), CreateContainerOptions{Image: "nginx"}); !errors.Is(err, ErrExecutableNotFound) {
		t.Fatalf("missing executable diagnostic lost: %v", err)
	}
}

func TestCreateDiagnosticsAndLogsExcludeSecrets(t *testing.T) {
	const secret = "dummy-secret-with-spaces"
	for _, failed := range []bool{false, true} {
		var log bytes.Buffer
		ctx := tflogtest.RootLogger(context.Background(), &log)
		c := NewClient(runnerFunc(func(_ context.Context, args ...string) (Result, error) {
			if !strings.Contains(strings.Join(args, " "), secret) {
				t.Fatal("execution arguments lost secret")
			}
			if failed {
				return Result{Stderr: []byte(secret), Stdout: []byte(secret), ExitCode: 7}, errors.New(secret)
			}
			return Result{Stdout: []byte("id")}, nil
		}))
		_, err := c.CreateContainer(ctx, CreateContainerOptions{Image: "nginx", Env: []string{"TOKEN=" + secret}})
		if failed != (err != nil) {
			t.Fatalf("unexpected err %v", err)
		}
		if (err != nil && strings.Contains(err.Error(), secret)) || strings.Contains(log.String(), secret) {
			t.Fatal("credential leaked")
		}
		if log.Len() == 0 {
			t.Fatal("expected debug log output")
		}
	}
}

func TestProcessRunnerErrorDoesNotIncludeArguments(t *testing.T) {
	if os.Getenv("WSLC_TEST_HELPER") == "1" {
		os.Exit(7)
	}
	t.Setenv("WSLC_TEST_HELPER", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewProcessRunner(exe).Run(context.Background(), "-test.run=^TestProcessRunnerErrorDoesNotIncludeArguments$", "--", "TOKEN=dummy-secret")
	if err == nil || result.ExitCode != 7 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if strings.Contains(err.Error(), "dummy-secret") {
		t.Fatal("credential leaked in process error")
	}
}

func TestInspect(t *testing.T) {
	for _, tc := range []struct {
		name, output    string
		runErr          error
		missing, failed bool
	}{
		{"found", `[{"Id":"id","Name":"/test","Config":{"Image":"nginx"},"State":{"Status":"running"}}]`, nil, false, false},
		{"missing", `[]`, errors.New("not found"), true, true},
		{"malformed", `invalid`, nil, false, true},
		{"exec failure", ``, errors.New("unavailable"), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient(runnerFunc(func(_ context.Context, args ...string) (Result, error) {
				return Result{Stdout: []byte(tc.output)}, tc.runErr
			}))
			got, err := c.InspectContainer(context.Background(), "id")
			if (err != nil) != tc.failed || errors.Is(err, ErrNotFound) != tc.missing {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			if !tc.failed && (got.Config.Image != "nginx" || got.State.Status != "running") {
				t.Fatalf("unexpected inspect %+v", got)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	for _, tc := range []struct {
		name          string
		removeErr     error
		inspectOutput string
		inspectErr    error
		wantCalls     int
		failed        bool
	}{
		{"success", nil, "", nil, 1, false},
		{"already absent", errors.New("missing"), "[]", errors.New("missing"), 2, false},
		{"still present", errors.New("denied"), `[{"Id":"id"}]`, nil, 2, true},
		{"inspect unavailable", errors.New("denied"), "", errors.New("offline"), 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := NewClient(runnerFunc(func(_ context.Context, args ...string) (Result, error) {
				calls++
				if calls == 1 {
					if !reflect.DeepEqual(args, []string{"remove", "--force", "id"}) {
						t.Fatalf("unexpected args %v", args)
					}
					return Result{}, tc.removeErr
				}
				return Result{Stdout: []byte(tc.inspectOutput)}, tc.inspectErr
			}))
			err := c.RemoveContainer(context.Background(), "id")
			if (err != nil) != tc.failed || calls != tc.wantCalls {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}
