package wslc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Result is the outcome of running a wslc.exe invocation.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Runner executes wslc.exe (or a stand-in for it in tests) and returns its
// captured output. Implementations must not use a shell: args are passed
// through to the child process argv directly, so callers never need to
// quote or escape image names, paths, or values that contain spaces.
type Runner interface {
	Run(ctx context.Context, args ...string) (Result, error)
}

// ProcessRunner is the production Runner: it invokes a real executable
// (normally "wslc.exe") as a child process. It never shells out through
// cmd.exe or PowerShell, so argument quoting is handled entirely by
// os/exec's argv-based process creation.
type ProcessRunner struct {
	// Executable is the program to run, either a bare name resolved
	// against PATH (e.g. "wslc.exe") or an absolute path.
	Executable string
}

// NewProcessRunner returns a ProcessRunner that invokes executable. If
// executable is empty, "wslc.exe" is used.
func NewProcessRunner(executable string) *ProcessRunner {
	if executable == "" {
		executable = "wslc.exe"
	}
	return &ProcessRunner{Executable: executable}
}

// ErrExecutableNotFound wraps errors from a missing wslc.exe so callers can
// give a diagnostic pointing at WSL container setup rather than a raw OS
// error.
var ErrExecutableNotFound = errors.New("wslc: executable not found")

func (r *ProcessRunner) Run(ctx context.Context, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, r.Executable, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := Result{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: -1,
	}

	if err == nil {
		result.ExitCode = 0
		return result, nil
	}

	var notFound *exec.Error
	if errors.As(err, &notFound) {
		return result, fmt.Errorf("%w: %s: %w", ErrExecutableNotFound, r.Executable, notFound.Err)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		// A non-zero exit is a normal, expected outcome for several wslc
		// commands (e.g. inspecting an object that does not exist), so it
		// is returned as an error the caller can inspect alongside the
		// captured stdout/stderr rather than only a bare Go error value.
		return result, fmt.Errorf("wslc: process failed: exit code %d: %w", result.ExitCode, err)
	}

	// context.Canceled / context.DeadlineExceeded surface here.
	return result, fmt.Errorf("wslc: process failed: %w", err)
}

// describeOutput formats a command's captured output for an error message.
func describeOutput(result Result) string {
	stderr := string(bytes.TrimSpace(result.Stderr))
	stdout := string(bytes.TrimSpace(result.Stdout))
	switch {
	case stderr != "" && stdout != "":
		return fmt.Sprintf("(stderr: %s, stdout: %s)", stderr, stdout)
	case stderr != "":
		return fmt.Sprintf("(stderr: %s)", stderr)
	case stdout != "":
		return fmt.Sprintf("(stdout: %s)", stdout)
	default:
		return "(no output captured)"
	}
}

// run deliberately excludes argv from logs: environment, labels and command
// arguments can contain credentials. Create diagnostics also omit process output.
func run(ctx context.Context, runner Runner, args ...string) (Result, error) {
	tflog.Debug(ctx, "wslc: invoking process")
	result, err := runner.Run(ctx, args...)
	tflog.Debug(ctx, "wslc: process completed", map[string]interface{}{"exit_code": result.ExitCode})
	return result, err
}
