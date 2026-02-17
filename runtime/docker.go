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

// detectContainerRuntime finds docker or podman on PATH.
func detectContainerRuntime() (string, error) {
	if path, err := exec.LookPath("docker"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("podman"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("neither docker nor podman found on PATH")
}

// imageName derives a docker image name from the source path and entrypoint.
// /home/user/my-project + script.py → kael-home-user-my-project-script-py:latest
func imageName(sourcePath, entrypoint, tag string) string {
	name := strings.ToLower(sourcePath)
	if entrypoint != "" {
		name += "/" + strings.ToLower(entrypoint)
	}
	name = strings.NewReplacer("/", "-", ".", "-", ":", "-", "@", "-").Replace(name)
	name = strings.TrimLeft(name, "-")
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("kael-%s:%s", name, tag)
}

// imageExists checks if a docker image already exists.
func imageExists(ctx context.Context, runtime, image string) bool {
	cmd := exec.CommandContext(ctx, runtime, "images", "-q", "--filter", "reference="+image)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// ensureImage builds the docker image if it doesn't exist or refresh is requested.
func ensureImage(ctx context.Context, runtime, sourcePath string, opts RunOptions) (string, error) {
	tag := opts.Tag
	if tag == "" {
		tag = "latest"
	}
	image := imageName(sourcePath, opts.Entrypoint, tag)

	if !opts.Refresh && imageExists(ctx, runtime, image) {
		return image, nil
	}

	// Generate Dockerfile
	kaelDir := filepath.Join(sourcePath, ".kael")
	if err := os.MkdirAll(kaelDir, 0755); err != nil {
		return "", fmt.Errorf("create .kael dir: %w", err)
	}

	dockerfile := generateDockerfile(opts.Type, opts.Entrypoint, opts.Deps)
	dockerfilePath := filepath.Join(kaelDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return "", fmt.Errorf("write Dockerfile: %w", err)
	}

	ignorePath := filepath.Join(sourcePath, ".dockerignore")
	if _, err := os.Stat(ignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(ignorePath, []byte(dockerIgnore), 0644); err != nil {
			return "", fmt.Errorf("write .dockerignore: %w", err)
		}
	}

	// Build
	buildCmd := exec.CommandContext(ctx, runtime, "build",
		"-t", image,
		"-f", dockerfilePath,
		sourcePath,
	)
	var buildOut bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut
	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("docker build: %w\n%s", err, buildOut.String())
	}

	return image, nil
}

// dockerRun executes a container and returns stdout.
func dockerRun(ctx context.Context, runtime, sourcePath string, opts RunOptions, input []byte) ([]byte, error) {
	image, err := ensureImage(ctx, runtime, sourcePath, opts)
	if err != nil {
		return nil, err
	}

	args := []string{"run", "-i", "--rm"}

	// Unique container name for timeout handling
	containerName := fmt.Sprintf("kael-%d-%d", os.Getpid(), time.Now().UnixNano())
	args = append(args, "--name", containerName)

	// Network
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}

	// Environment variables
	for _, e := range opts.EnvMap {
		if strings.Contains(e, "=") {
			args = append(args, "-e", e)
		} else {
			// Passthrough from host
			if val, ok := os.LookupEnv(e); ok {
				args = append(args, "-e", fmt.Sprintf("%s=%s", e, val))
			}
		}
	}

	// Volume mounts
	for _, f := range opts.FsMap {
		args = append(args, "-v", f)
	}

	// Port mappings
	for _, p := range opts.PortMap {
		args = append(args, "-p", p)
	}

	args = append(args, image)

	// Extra args passed to the entrypoint
	args = append(args, opts.ExtraArgs...)

	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, runtime, args...)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// On timeout, kill the container
	if ctx.Err() == context.DeadlineExceeded {
		killCmd := exec.Command(runtime, "kill", containerName)
		_ = killCmd.Run()
		return nil, fmt.Errorf("execution timed out after %ds", opts.Timeout)
	}

	if err != nil {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return nil, &RunError{
			ExitCode: exitCode,
			Stderr:   stderr.String(),
		}
	}

	return stdout.Bytes(), nil
}
