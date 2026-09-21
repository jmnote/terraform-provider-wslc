package wslc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// Client is the abstraction internal/provider programs against. The only
// production implementation (client, via NewClient) calls wslc.exe through
// a Runner; tests substitute a fake Runner instead of a fake Client, so the
// argument-building and output-parsing logic under test is the same code
// path the provider actually runs.
type Client interface {
	// CreateContainer creates (but does not start) a container via
	// `wslc create`, returning its full container ID.
	CreateContainer(ctx context.Context, opts CreateContainerOptions) (string, error)

	// StartContainer starts an existing, stopped container via
	// `wslc start`.
	StartContainer(ctx context.Context, id string) error

	// InspectContainer returns the current observed state of the
	// container named or identified by id. It returns ErrNotFound
	// (checkable with errors.Is) if no such container exists.
	InspectContainer(ctx context.Context, id string) (*ContainerInspect, error)

	// RemoveContainer deletes a container via `wslc remove --force`.
	// RemoveContainer is idempotent: removing a container that is
	// already absent returns nil rather than an error.
	RemoveContainer(ctx context.Context, id string) error
}

type client struct {
	runner Runner
}

// NewClient returns a Client that drives wslc.exe through runner.
func NewClient(runner Runner) Client {
	return &client{runner: runner}
}

func (c *client) CreateContainer(ctx context.Context, opts CreateContainerOptions) (string, error) {
	if opts.Image == "" {
		return "", fmt.Errorf("wslc: create container: image is required")
	}

	// Flags are appended in the order `wslc create --help` lists them;
	// Image and Command (its Arguments) are appended last, as required by
	// `wslc create [options] <image> [<command>] [<arguments>...]`.
	args := []string{"create"}
	if opts.CPUs != "" {
		args = append(args, "--cpus", opts.CPUs)
	}
	for _, d := range opts.DNS {
		args = append(args, "--dns", d)
	}
	for _, d := range opts.DNSOptions {
		args = append(args, "--dns-option", d)
	}
	for _, d := range opts.DNSSearch {
		args = append(args, "--dns-search", d)
	}
	if opts.Domainname != "" {
		args = append(args, "--domainname", opts.Domainname)
	}
	if opts.Entrypoint != "" {
		args = append(args, "--entrypoint", opts.Entrypoint)
	}
	for _, e := range opts.Env {
		args = append(args, "--env", e)
	}
	if opts.Hostname != "" {
		args = append(args, "--hostname", opts.Hostname)
	}
	if opts.IP != "" {
		args = append(args, "--ip", opts.IP)
	}
	keys := make([]string, 0, len(opts.Labels))
	for k := range opts.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--label", k+"="+opts.Labels[k])
	}

	if opts.Memory != "" {
		args = append(args, "--memory", opts.Memory)
	}
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	for _, p := range opts.Publish {
		args = append(args, "--publish", p)
	}
	if opts.PublishAll {
		args = append(args, "--publish-all")
	}
	if opts.PullPolicy != "" {
		args = append(args, "--pull", opts.PullPolicy)
	}
	if opts.Remove {
		args = append(args, "--rm")
	}
	if opts.ShmSize != "" {
		args = append(args, "--shm-size", opts.ShmSize)
	}
	if opts.StopSignal != "" {
		args = append(args, "--stop-signal", opts.StopSignal)
	}
	if opts.StopTimeout != nil {
		args = append(args, "--stop-timeout", strconv.Itoa(*opts.StopTimeout))
	}
	for _, t := range opts.Tmpfs {
		args = append(args, "--tmpfs", t)
	}
	for _, u := range opts.Ulimits {
		args = append(args, "--ulimit", u)
	}
	if opts.User != "" {
		args = append(args, "--user", opts.User)
	}
	if opts.WorkDir != "" {
		args = append(args, "--workdir", opts.WorkDir)
	}
	args = append(args, opts.Image)
	args = append(args, opts.Command...)

	result, err := run(ctx, c.runner, args...)
	if err != nil {
		if errors.Is(err, ErrExecutableNotFound) {
			return "", fmt.Errorf("wslc: create container: %w", ErrExecutableNotFound)
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("wslc: create container: %w", ctx.Err())
		}
		return "", fmt.Errorf("wslc: create container failed (exit code %d); process details omitted because they may contain credentials", result.ExitCode)
	}

	id := string(bytes.TrimSpace(result.Stdout))
	if id == "" {
		return "", fmt.Errorf("wslc: create container: no container ID returned; process output omitted because it may contain credentials")
	}
	return id, nil
}

func (c *client) StartContainer(ctx context.Context, id string) error {
	result, err := run(ctx, c.runner, "start", id)
	if err != nil {
		return fmt.Errorf("wslc: start container %q: %w %s", id, err, describeOutput(result))
	}
	return nil
}

func (c *client) InspectContainer(ctx context.Context, id string) (*ContainerInspect, error) {
	result, err := run(ctx, c.runner, "inspect", id, "--type", "container", "--format", "json")

	// wslc.exe always prints a JSON array to stdout for `inspect`, even on
	// failure ("[]" when the object does not exist), so parse stdout first
	// and only fall back to the raw exec error when it is not the
	// well-formed-but-empty shape that means "not found". This avoids
	// relying on locale-dependent stderr text to detect a missing object.
	var results []ContainerInspect
	if jsonErr := json.Unmarshal(result.Stdout, &results); jsonErr != nil {
		if err != nil {
			return nil, fmt.Errorf("wslc: inspect container %q: %w %s", id, err, describeOutput(result))
		}
		return nil, fmt.Errorf("wslc: inspect container %q: parsing output: %w %s", id, jsonErr, describeOutput(result))
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("%w: container %q", ErrNotFound, id)
	}
	return &results[0], nil
}

func (c *client) RemoveContainer(ctx context.Context, id string) error {
	// Optimistically remove in one invocation. Only inspect after a failure,
	// including a concurrent deletion, to distinguish absence from real errors.
	result, err := run(ctx, c.runner, "remove", "--force", id)
	if err == nil {
		return nil
	}
	if ctx.Err() == nil {
		if _, inspectErr := c.InspectContainer(ctx, id); errors.Is(inspectErr, ErrNotFound) {
			return nil
		}
	}
	return fmt.Errorf("wslc: remove container %q: %w %s", id, err, describeOutput(result))
}
