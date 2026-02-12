package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// nativeRun executes a script directly on the host without containerization.
func nativeRun(ctx context.Context, sourcePath string, opts RunOptions, input []byte) ([]byte, error) {
	if err := installDeps(sourcePath, opts.Type); err != nil {
		return nil, fmt.Errorf("install deps: %w", err)
	}

	interpreter, interpArgs := resolveInterpreter(sourcePath, opts.Type)
	entrypoint := opts.Entrypoint

	// Build command: interpreter [args...] entrypoint [extra args...]
	cmdArgs := append(interpArgs, entrypoint)
	cmdArgs = append(cmdArgs, opts.ExtraArgs...)

	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, interpreter, cmdArgs...)
	cmd.Dir = sourcePath
	cmd.Stdin = bytes.NewReader(input)

	// Merge environment: inherit current env + add mappings
	cmd.Env = os.Environ()
	for _, e := range opts.EnvMap {
		if strings.Contains(e, "=") {
			cmd.Env = append(cmd.Env, e)
		} else {
			// Passthrough: re-export from current env
			if val, ok := os.LookupEnv(e); ok {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", e, val))
			}
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("execution timed out after %ds", opts.Timeout)
	}

	if err != nil {
		return nil, &RunError{
			ExitCode: cmd.ProcessState.ExitCode(),
			Stderr:   stderr.String(),
		}
	}

	return stdout.Bytes(), nil
}

// installDeps installs dependencies based on script type.
func installDeps(sourcePath, scriptType string) error {
	switch strings.ToLower(scriptType) {
	case "python":
		return installPythonDeps(sourcePath)
	case "node":
		return installNodeDeps(sourcePath)
	default:
		return nil
	}
}

// installPythonDeps creates a venv and installs requirements.txt if present.
func installPythonDeps(sourcePath string) error {
	venvPath := filepath.Join(sourcePath, ".kael", "venv")

	// Create venv if it doesn't exist
	if _, err := os.Stat(filepath.Join(venvPath, "bin", "python")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(sourcePath, ".kael"), 0755); err != nil {
			return err
		}
		cmd := exec.Command("python3", "-m", "venv", venvPath)
		cmd.Dir = sourcePath
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create venv: %w", err)
		}
	}

	// Install requirements if present
	reqPath := filepath.Join(sourcePath, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		pip := filepath.Join(venvPath, "bin", "pip")
		cmd := exec.Command(pip, "install", "-q", "-r", reqPath)
		cmd.Dir = sourcePath
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pip install: %w", err)
		}
	}

	return nil
}

// installNodeDeps runs npm install if package.json exists.
func installNodeDeps(sourcePath string) error {
	pkgPath := filepath.Join(sourcePath, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return nil
	}

	nodeModules := filepath.Join(sourcePath, "node_modules")
	if _, err := os.Stat(nodeModules); err == nil {
		return nil // already installed
	}

	cmd := exec.Command("npm", "install", "--production")
	cmd.Dir = sourcePath
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}

	return nil
}

// resolveInterpreter returns the interpreter binary and any extra args for the script type.
// For Python, uses the venv python if available.
func resolveInterpreter(sourcePath, scriptType string) (string, []string) {
	switch strings.ToLower(scriptType) {
	case "python":
		venvPython := filepath.Join(sourcePath, ".kael", "venv", "bin", "python")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython, nil
		}
		return "python3", nil
	case "node":
		return "node", nil
	case "shell":
		return "/bin/sh", nil
	default:
		return "/bin/sh", nil
	}
}
