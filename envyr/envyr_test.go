package envyr

import (
	"testing"
)

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name     string
		client   Client
		opts     RunOptions
		expected []string
	}{
		{
			name:   "minimal",
			client: Client{},
			opts: RunOptions{
				Source: "git@github.com:someone/repo.git",
			},
			expected: []string{
				"run",
				"git@github.com:someone/repo.git",
			},
		},
		{
			name:   "full options",
			client: Client{Verbose: true, Root: "/tmp/envyr"},
			opts: RunOptions{
				Source:      "git@github.com:someone/repo.git",
				SubDir:     "scripts",
				Entrypoint: "query.py",
				Tag:        "v1.0",
				Executor:   ExecutorDocker,
				Autogen:    true,
				Timeout:    60,
				EnvMap:     []string{"AWS_PROFILE", "KUBECONFIG"},
				FsMap:      []string{"/data:/data"},
				PortMap:    []string{"8080:80"},
				Network:    "host",
				Refresh:    true,
				Name:       "my-action",
				Interpreter: "python3",
				Type:       "python",
				Interactive: true,
			},
			expected: []string{
				"-v",
				"--root", "/tmp/envyr",
				"run",
				"--sub-dir", "scripts",
				"--entrypoint", "query.py",
				"--tag", "v1.0",
				"--executor", "docker",
				"--autogen",
				"--timeout", "60",
				"--env-map", "AWS_PROFILE",
				"--env-map", "KUBECONFIG",
				"--fs-map", "/data:/data",
				"--port-map", "8080:80",
				"--network", "host",
				"--refresh",
				"--name", "my-action",
				"--interpreter", "python3",
				"--type", "python",
				"--interactive",
				"git@github.com:someone/repo.git",
			},
		},
		{
			name:   "executor native with env passthrough",
			client: Client{},
			opts: RunOptions{
				Source:   "/local/path",
				Executor: ExecutorNative,
				EnvMap:   []string{"HOME"},
			},
			expected: []string{
				"run",
				"--executor", "native",
				"--env-map", "HOME",
				"/local/path",
			},
		},
		{
			name:   "docker autogen typical kael usage",
			client: Client{},
			opts: RunOptions{
				Source:     "git@github.com:someone/prometheus-tools.git",
				Entrypoint: "query.py",
				Executor:  ExecutorDocker,
				Autogen:   true,
				Timeout:   60,
				EnvMap:    []string{"AWS_PROFILE"},
			},
			expected: []string{
				"run",
				"--entrypoint", "query.py",
				"--executor", "docker",
				"--autogen",
				"--timeout", "60",
				"--env-map", "AWS_PROFILE",
				"git@github.com:someone/prometheus-tools.git",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.client.buildArgs(tt.opts)
			if len(got) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(tt.expected), got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("arg[%d]: got %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestRunSourceRequired(t *testing.T) {
	c := &Client{}
	_, err := c.Run(t.Context(), RunOptions{}, nil)
	if err == nil {
		t.Fatal("expected error for empty Source")
	}
	want := "envyr: Source is required"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestRunErrorFormat(t *testing.T) {
	e := &RunError{ExitCode: 1, Stderr: "connection refused"}
	got := e.Error()
	want := "envyr: exit code 1: connection refused"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	e2 := &RunError{ExitCode: 2}
	got2 := e2.Error()
	want2 := "envyr: exit code 2"
	if got2 != want2 {
		t.Errorf("got %q, want %q", got2, want2)
	}
}
