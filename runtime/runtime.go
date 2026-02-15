package runtime

import (
	"context"
	"fmt"
	"strings"
)

// Executor types
const (
	ExecutorDocker = "docker"
	ExecutorNative = "native"
)

// RunOptions configures a tool execution.
type RunOptions struct {
	Source     string   // Local path or git URL (required)
	SubDir     string   // Subdirectory within source (for monorepos)
	Entrypoint string   // Script filename
	Tag        string   // Git tag, branch, or commit hash
	Executor   string   // "docker" or "native"
	Type       string   // "python", "node", "shell"
	Timeout    int      // Execution timeout in seconds
	EnvMap     []string // Environment variable passthrough
	FsMap      []string // Volume mounts (host:container)
	PortMap    []string // Port mappings (host:container)
	Network    string   // Docker network
	Refresh    bool     // Force re-fetch/rebuild
	ExtraArgs  []string // Args passed to the script
	Deps       []string // Dependencies (pip packages, npm packages, or apk packages for shell)
}

// RunError is returned when execution fails with a non-zero exit code.
type RunError struct {
	ExitCode int
	Stderr   string
}

func (e *RunError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("exit code %d: %s", e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("exit code %d", e.ExitCode)
}

// Runner executes tools. This is the interface the engine depends on.
type Runner interface {
	Run(ctx context.Context, opts RunOptions, input []byte) ([]byte, error)
}

// DefaultRunner is the standard implementation that resolves sources
// and dispatches to docker or native executors.
type DefaultRunner struct{}

func NewDefaultRunner() *DefaultRunner {
	return &DefaultRunner{}
}

func (r *DefaultRunner) Run(ctx context.Context, opts RunOptions, input []byte) ([]byte, error) {
	if opts.Source == "" {
		return nil, fmt.Errorf("source is required")
	}

	// Resolve source to a local path
	sourcePath, err := ResolveSource(opts.Source, opts.Tag, opts.SubDir, opts.Refresh)
	if err != nil {
		return nil, fmt.Errorf("resolve source: %w", err)
	}

	executor := strings.ToLower(opts.Executor)
	if executor == "" {
		executor = ExecutorDocker
	}

	switch executor {
	case ExecutorDocker:
		runtime, err := detectContainerRuntime()
		if err != nil {
			return nil, err
		}
		return dockerRun(ctx, runtime, sourcePath, opts, input)
	case ExecutorNative:
		return nativeRun(ctx, sourcePath, opts, input)
	default:
		return nil, fmt.Errorf("unknown executor: %s", opts.Executor)
	}
}
