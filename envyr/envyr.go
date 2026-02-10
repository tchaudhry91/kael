package envyr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
)

type Executor string

const (
	ExecutorDocker Executor = "docker"
	ExecutorNative Executor = "native"
)

// RunOptions maps to the flags of `envyr run`.
type RunOptions struct {
	Source      string   // PROJECT_ROOT (required)
	SubDir      string   // --sub-dir
	Entrypoint  string   // --entrypoint
	Tag         string   // --tag
	Executor    Executor // --executor
	Autogen     bool     // --autogen
	Timeout     int      // --timeout (seconds)
	EnvMap      []string // --env-map entries
	FsMap       []string // --fs-map entries
	PortMap     []string // --port-map entries
	Network     string   // --network
	Refresh     bool     // --refresh
	Name        string   // --name
	Interpreter string   // --interpreter
	Type        string   // --type (python/node/shell/other)
	Interactive bool     // --interactive
}

// Client wraps the envyr CLI binary.
type Client struct {
	BinPath string // path to envyr binary, defaults to "envyr"
	Verbose bool   // -v global flag
	Root    string // --root global override
}

func NewDefaultClient() *Client {
	return &Client{}
}

func (c *Client) binPath() string {
	if c.BinPath != "" {
		return c.BinPath
	}
	return "envyr"
}

// buildArgs constructs the argument slice for an envyr run invocation.
func (c *Client) buildArgs(opts RunOptions) []string {
	var args []string

	// Global flags come before the subcommand.
	if c.Verbose {
		args = append(args, "-v")
	}
	if c.Root != "" {
		args = append(args, "--root", c.Root)
	}

	args = append(args, "run")

	if opts.SubDir != "" {
		args = append(args, "--sub-dir", opts.SubDir)
	}
	if opts.Entrypoint != "" {
		args = append(args, "--entrypoint", opts.Entrypoint)
	}
	if opts.Tag != "" {
		args = append(args, "--tag", opts.Tag)
	}
	if opts.Executor != "" {
		args = append(args, "--executor", string(opts.Executor))
	}
	if opts.Autogen {
		args = append(args, "--autogen")
	}
	if opts.Timeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(opts.Timeout))
	}
	for _, e := range opts.EnvMap {
		args = append(args, "--env-map", e)
	}
	for _, f := range opts.FsMap {
		args = append(args, "--fs-map", f)
	}
	for _, p := range opts.PortMap {
		args = append(args, "--port-map", p)
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	if opts.Refresh {
		args = append(args, "--refresh")
	}
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	if opts.Interpreter != "" {
		args = append(args, "--interpreter", opts.Interpreter)
	}
	if opts.Type != "" {
		args = append(args, "--type", opts.Type)
	}
	if opts.Interactive {
		args = append(args, "--interactive")
	}

	// PROJECT_ROOT is the positional argument, must come last.
	args = append(args, opts.Source)

	return args
}

// Run executes `envyr run` with the given options, piping input to stdin.
// It returns the raw stdout bytes on success.
func (c *Client) Run(ctx context.Context, opts RunOptions, input []byte) ([]byte, error) {
	if opts.Source == "" {
		return nil, errors.New("envyr: Source is required")
	}

	args := c.buildArgs(opts)
	cmd := exec.CommandContext(ctx, c.binPath(), args...)

	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, &RunError{
				ExitCode: exitErr.ExitCode(),
				Stderr:   stderr.String(),
			}
		}
		return nil, fmt.Errorf("envyr: %w", err)
	}

	return stdout.Bytes(), nil
}
