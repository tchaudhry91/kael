package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tchaudhry91/kael/envyr"
	"github.com/yuin/gopher-lua"
)

// mockRunner records the call and returns a canned response.
type mockRunner struct {
	lastOpts  envyr.RunOptions
	lastInput []byte
	output    []byte
	err       error
}

func (m *mockRunner) Run(_ context.Context, opts envyr.RunOptions, input []byte) ([]byte, error) {
	m.lastOpts = opts
	m.lastInput = input
	return m.output, m.err
}

// writeKit creates a minimal kit in a temp directory and returns the path.
func writeKit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// newTestEngine creates an engine with a mock runner and the given kit files.
func newTestEngine(t *testing.T, mock *mockRunner, kitFiles map[string]string) *Engine {
	t.Helper()
	kitDir := writeKit(t, kitFiles)

	L := lua.NewState()
	pkg := L.GetGlobal("package")
	L.SetField(pkg, "path", lua.LString(
		fmt.Sprintf("%s/?.lua;%s/?/init.lua", kitDir, kitDir),
	))

	e := &Engine{
		KitRoot:  kitDir,
		lstate:   L,
		Runner:   mock,
		Registry: make(map[*lua.LFunction]ToolConfig),
	}
	e.RegisterTools()
	if err := L.DoString("kit = require(\"init\")"); err != nil {
		t.Fatalf("failed to load kit: %v", err)
	}
	return e
}

func TestDefineToolConfigParsing(t *testing.T) {
	mock := &mockRunner{output: []byte(`{"ok": true}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				entrypoint = "run.py",
				subdir = "scripts",
				executor = "native",
				tag = "v1.2.0",
				defaults = {
					endpoint = "http://localhost:9090",
					step = 5,
				},
				env = { "AWS_PROFILE", "HOME" },
				timeout = 30,
			})
		`,
	})
	defer e.Close()

	// Call the tool to trigger the closure, which lets us inspect what mockRunner received
	err := e.RunString(context.Background(), `kit.test({ query = "up" })`)
	if err != nil {
		t.Fatalf("tool call failed: %v", err)
	}

	// Verify RunOptions
	if mock.lastOpts.Source != "git@github.com:someone/repo.git" {
		t.Errorf("source: got %q", mock.lastOpts.Source)
	}
	if mock.lastOpts.Entrypoint != "run.py" {
		t.Errorf("entrypoint: got %q", mock.lastOpts.Entrypoint)
	}
	if mock.lastOpts.SubDir != "scripts" {
		t.Errorf("subdir: got %q", mock.lastOpts.SubDir)
	}
	if mock.lastOpts.Executor != envyr.ExecutorNative {
		t.Errorf("executor: got %q", mock.lastOpts.Executor)
	}
	if mock.lastOpts.Timeout != 30 {
		t.Errorf("timeout: got %d", mock.lastOpts.Timeout)
	}
	if mock.lastOpts.Tag != "v1.2.0" {
		t.Errorf("tag: got %q", mock.lastOpts.Tag)
	}
	if !mock.lastOpts.Autogen {
		t.Error("autogen should be true")
	}
	if len(mock.lastOpts.EnvMap) != 2 || mock.lastOpts.EnvMap[0] != "AWS_PROFILE" || mock.lastOpts.EnvMap[1] != "HOME" {
		t.Errorf("env: got %v", mock.lastOpts.EnvMap)
	}
}

func TestDefineToolTagVariants(t *testing.T) {
	tests := []struct {
		name string
		tag  string
	}{
		{"git tag", "v1.0.0"},
		{"branch", "main"},
		{"commit hash", "abc1234def5678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRunner{output: []byte(`{}`)}
			e := newTestEngine(t, mock, map[string]string{
				"init.lua": `
					local M = {}
					M.test = require("test")
					return M
				`,
				"test.lua": fmt.Sprintf(`
					return tools.define_tool({
						source = "git@github.com:someone/repo.git",
						tag = "%s",
					})
				`, tt.tag),
			})
			defer e.Close()

			err := e.RunString(context.Background(), `kit.test({})`)
			if err != nil {
				t.Fatalf("tool call failed: %v", err)
			}

			if mock.lastOpts.Tag != tt.tag {
				t.Errorf("tag: got %q, want %q", mock.lastOpts.Tag, tt.tag)
			}
		})
	}
}

func TestDefineToolExecutorDefault(t *testing.T) {
	mock := &mockRunner{output: []byte(`{}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `kit.test({})`)
	if err != nil {
		t.Fatalf("tool call failed: %v", err)
	}

	if mock.lastOpts.Executor != envyr.ExecutorDocker {
		t.Errorf("executor should default to docker, got %q", mock.lastOpts.Executor)
	}
}

func TestDefaultMergingUserWins(t *testing.T) {
	mock := &mockRunner{output: []byte(`{}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				input_adapter = "json",
				output_adapter = "json",
				defaults = {
					endpoint = "http://default:9090",
					step = 5,
				},
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `kit.test({ endpoint = "http://custom:9090", query = "up" })`)
	if err != nil {
		t.Fatalf("tool call failed: %v", err)
	}

	var input map[string]any
	if err := json.Unmarshal(mock.lastInput, &input); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}

	// User param overrides default
	if input["endpoint"] != "http://custom:9090" {
		t.Errorf("endpoint: got %v, want http://custom:9090", input["endpoint"])
	}
	// Default preserved when user doesn't set it
	if input["step"] != float64(5) {
		t.Errorf("step: got %v (type %T), want 5", input["step"], input["step"])
	}
	// User-only param present
	if input["query"] != "up" {
		t.Errorf("query: got %v, want up", input["query"])
	}
}

func TestToolReturnsParsedOutput(t *testing.T) {
	mock := &mockRunner{
		output: []byte(`{"windows": [{"day": "Monday", "hour": 3}], "count": 1}`),
	}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				input_adapter = "json",
				output_adapter = "json",
			})
		`,
	})
	defer e.Close()

	// Call the tool and inspect the return value in Lua
	err := e.RunString(context.Background(), `
		result = kit.test({})
		assert(result.count == 1, "expected count=1, got " .. tostring(result.count))
		assert(result.windows[1].day == "Monday", "expected day=Monday")
		assert(result.windows[1].hour == 3, "expected hour=3")
	`)
	if err != nil {
		t.Fatalf("tool call failed: %v", err)
	}
}

func TestToolRunnerError(t *testing.T) {
	mock := &mockRunner{
		err: fmt.Errorf("connection refused"),
	}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
			})
		`,
	})
	defer e.Close()

	// Direct call should raise a Lua error
	err := e.RunString(context.Background(), `kit.test({})`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// pcall should catch it
	err = e.RunString(context.Background(), `
		local ok, err = pcall(kit.test, {})
		assert(not ok, "expected pcall to fail")
		assert(string.find(err, "Tool Run Failure"), "expected Tool Run Failure in error, got: " .. err)
	`)
	if err != nil {
		t.Fatalf("pcall test failed: %v", err)
	}
}

func TestToolInvalidJSONResponse(t *testing.T) {
	mock := &mockRunner{
		output: []byte(`not json`),
	}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				output_adapter = "json",
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `
		local ok, err = pcall(kit.test, {})
		assert(not ok, "expected pcall to fail")
		assert(string.find(err, "Data UnMarshal Failure"), "expected unmarshal error, got: " .. err)
	`)
	if err != nil {
		t.Fatalf("test failed: %v", err)
	}
}

func TestNamespacedKit(t *testing.T) {
	mock := &mockRunner{output: []byte(`{"result": "from_query"}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.prometheus = require("prometheus")
			return M
		`,
		"prometheus/init.lua": `
			local M = {}
			M.query = require("prometheus.query")
			return M
		`,
		"prometheus/query.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/prometheus-tools.git",
				entrypoint = "query.py",
				executor = "docker",
				input_adapter = "json",
				output_adapter = "json",
				defaults = {
					endpoint = "http://prometheus:9090",
				},
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `
		local result = kit.prometheus.query({ query = "up" })
		assert(result.result == "from_query", "expected from_query")
	`)
	if err != nil {
		t.Fatalf("namespaced call failed: %v", err)
	}

	if mock.lastOpts.Source != "git@github.com:someone/prometheus-tools.git" {
		t.Errorf("source: got %q", mock.lastOpts.Source)
	}
	if mock.lastOpts.Entrypoint != "query.py" {
		t.Errorf("entrypoint: got %q", mock.lastOpts.Entrypoint)
	}
}

func TestListTools(t *testing.T) {
	mock := &mockRunner{output: []byte(`{}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.prometheus = require("prometheus")
			M.kubernetes = require("kubernetes")
			return M
		`,
		"prometheus/init.lua": `
			local M = {}
			M.query = require("prometheus.query")
			return M
		`,
		"prometheus/query.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/prometheus-tools.git",
				entrypoint = "query.py",
				executor = "docker",
			})
		`,
		"kubernetes/init.lua": `
			local M = {}
			M.nodes = require("kubernetes.nodes")
			M.pods = require("kubernetes.pods")
			return M
		`,
		"kubernetes/nodes.lua": `
			return tools.define_tool({
				source = "/local/k8s-tools",
				entrypoint = "nodes.sh",
				executor = "native",
				type = "shell",
			})
		`,
		"kubernetes/pods.lua": `
			return tools.define_tool({
				source = "/local/k8s-tools",
				entrypoint = "pods.py",
				executor = "native",
				type = "python",
			})
		`,
	})
	defer e.Close()

	root := e.ListTools()

	// Check namespaces exist
	if _, ok := root.Children["prometheus"]; !ok {
		t.Fatal("expected prometheus namespace")
	}
	if _, ok := root.Children["kubernetes"]; !ok {
		t.Fatal("expected kubernetes namespace")
	}

	// Check prometheus tools
	prom := root.Children["prometheus"]
	if cfg, ok := prom.Tools["query"]; !ok {
		t.Error("expected prometheus.query tool")
	} else {
		if cfg.Source != "git@github.com:someone/prometheus-tools.git" {
			t.Errorf("prometheus.query source: got %q", cfg.Source)
		}
		if cfg.Entrypoint != "query.py" {
			t.Errorf("prometheus.query entrypoint: got %q", cfg.Entrypoint)
		}
		if cfg.Executor != "docker" {
			t.Errorf("prometheus.query executor: got %q", cfg.Executor)
		}
	}

	// Check kubernetes tools
	k8s := root.Children["kubernetes"]
	if cfg, ok := k8s.Tools["nodes"]; !ok {
		t.Error("expected kubernetes.nodes tool")
	} else {
		if cfg.Executor != "native" {
			t.Errorf("kubernetes.nodes executor: got %q", cfg.Executor)
		}
		if cfg.Type != "shell" {
			t.Errorf("kubernetes.nodes type: got %q", cfg.Type)
		}
	}
	if cfg, ok := k8s.Tools["pods"]; !ok {
		t.Error("expected kubernetes.pods tool")
	} else {
		if cfg.Type != "python" {
			t.Errorf("kubernetes.pods type: got %q", cfg.Type)
		}
	}
}

func TestSchemaParsing(t *testing.T) {
	mock := &mockRunner{output: []byte(`{}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				schema = {
					input = {
						node = "string",
						namespace = "string?",
						limit = { type = "number", description = "max results" },
						verbose = { type = "boolean", required = false, description = "enable verbose output" },
					},
					output = {
						pods = "array",
						count = "number",
					},
				},
			})
		`,
	})
	defer e.Close()

	root := e.ListTools()
	cfg := root.Tools["test"]
	if cfg.Schema == nil {
		t.Fatal("expected schema to be parsed")
	}

	// Input fields
	if len(cfg.Schema.Input) != 4 {
		t.Fatalf("expected 4 input fields, got %d", len(cfg.Schema.Input))
	}

	node := cfg.Schema.Input["node"]
	if node.Type != "string" || !node.Required {
		t.Errorf("node: got type=%q required=%v", node.Type, node.Required)
	}

	ns := cfg.Schema.Input["namespace"]
	if ns.Type != "string" || ns.Required {
		t.Errorf("namespace: got type=%q required=%v (want optional)", ns.Type, ns.Required)
	}

	limit := cfg.Schema.Input["limit"]
	if limit.Type != "number" || !limit.Required || limit.Description != "max results" {
		t.Errorf("limit: got type=%q required=%v desc=%q", limit.Type, limit.Required, limit.Description)
	}

	verbose := cfg.Schema.Input["verbose"]
	if verbose.Type != "boolean" || verbose.Required || verbose.Description != "enable verbose output" {
		t.Errorf("verbose: got type=%q required=%v desc=%q", verbose.Type, verbose.Required, verbose.Description)
	}

	// Output fields
	if len(cfg.Schema.Output) != 2 {
		t.Fatalf("expected 2 output fields, got %d", len(cfg.Schema.Output))
	}
	if cfg.Schema.Output["pods"].Type != "array" {
		t.Errorf("pods output: got type=%q", cfg.Schema.Output["pods"].Type)
	}
	if cfg.Schema.Output["count"].Type != "number" {
		t.Errorf("count output: got type=%q", cfg.Schema.Output["count"].Type)
	}
}

func TestSchemaValidationRequiredField(t *testing.T) {
	mock := &mockRunner{output: []byte(`{}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				schema = {
					input = {
						node = "string",
						namespace = "string?",
					},
				},
			})
		`,
	})
	defer e.Close()

	// Missing required "node" should fail
	err := e.RunString(context.Background(), `
		local ok, err = pcall(kit.test, { namespace = "default" })
		assert(not ok, "expected pcall to fail")
		assert(string.find(err, "Schema Validation Failure"), "expected schema error, got: " .. err)
		assert(string.find(err, "node"), "expected field name in error, got: " .. err)
	`)
	if err != nil {
		t.Fatalf("test failed: %v", err)
	}

	// Providing required "node" should pass
	err = e.RunString(context.Background(), `kit.test({ node = "worker-1" })`)
	if err != nil {
		t.Fatalf("valid call failed: %v", err)
	}
}

func TestSchemaValidationTypeCheck(t *testing.T) {
	mock := &mockRunner{output: []byte(`{}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				schema = {
					input = {
						count = "number",
						name = "string",
					},
				},
			})
		`,
	})
	defer e.Close()

	// Wrong type: string instead of number
	err := e.RunString(context.Background(), `
		local ok, err = pcall(kit.test, { count = "not_a_number", name = "test" })
		assert(not ok, "expected pcall to fail")
		assert(string.find(err, "Schema Validation Failure"), "expected schema error, got: " .. err)
		assert(string.find(err, "count"), "expected field name in error, got: " .. err)
	`)
	if err != nil {
		t.Fatalf("type check test failed: %v", err)
	}

	// Correct types should pass
	err = e.RunString(context.Background(), `kit.test({ count = 10, name = "test" })`)
	if err != nil {
		t.Fatalf("valid call failed: %v", err)
	}
}

func TestNoSchemaStillWorks(t *testing.T) {
	mock := &mockRunner{output: []byte(`{"ok": true}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
			})
		`,
	})
	defer e.Close()

	// Any input should work without schema
	err := e.RunString(context.Background(), `kit.test({ anything = "goes", count = 42 })`)
	if err != nil {
		t.Fatalf("call without schema failed: %v", err)
	}

	root := e.ListTools()
	cfg := root.Tools["test"]
	if cfg.Schema != nil {
		t.Error("expected nil schema when none declared")
	}
}

func TestInputAdapterArgs(t *testing.T) {
	mock := &mockRunner{output: []byte(`{}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				input_adapter = "args",
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `kit.test({ name = "world", count = 3 })`)
	if err != nil {
		t.Fatalf("tool call failed: %v", err)
	}

	// stdin should be nil/empty for args adapter
	if len(mock.lastInput) > 0 {
		t.Errorf("expected empty stdin for args adapter, got %q", mock.lastInput)
	}

	// ExtraArgs should contain the flags
	args := mock.lastOpts.ExtraArgs
	if len(args) == 0 {
		t.Fatal("expected ExtraArgs to be populated")
	}

	// Check that args contain --name world and --count 3
	argStr := fmt.Sprintf("%v", args)
	if !containsFlag(args, "--name", "world") {
		t.Errorf("expected --name world in args, got %s", argStr)
	}
	if !containsFlag(args, "--count", "3") {
		t.Errorf("expected --count 3 in args, got %s", argStr)
	}
}

func TestInputAdapterArgsBooleans(t *testing.T) {
	mock := &mockRunner{output: []byte(`{}`)}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				input_adapter = "args",
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `kit.test({ verbose = true, quiet = false, name = "test" })`)
	if err != nil {
		t.Fatalf("tool call failed: %v", err)
	}

	args := mock.lastOpts.ExtraArgs
	// true boolean should produce --verbose (no value)
	found := false
	for i, a := range args {
		if a == "--verbose" {
			// Next arg should NOT be "true" — it's a standalone flag
			if i+1 < len(args) && args[i+1] != "true" {
				found = true
			} else if i+1 >= len(args) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected standalone --verbose flag, got %v", args)
	}

	// false boolean should be omitted entirely
	for _, a := range args {
		if a == "--quiet" {
			t.Errorf("expected --quiet to be omitted for false value, got %v", args)
		}
	}
}

func TestOutputAdapterText(t *testing.T) {
	mock := &mockRunner{output: []byte("Hello, this is plain text output\nLine 2\n")}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				output_adapter = "text",
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `
		local result = kit.test({})
		assert(result.output ~= nil, "expected output field")
		assert(string.find(result.output, "Hello"), "expected Hello in output")
		assert(string.find(result.output, "Line 2"), "expected Line 2 in output")
	`)
	if err != nil {
		t.Fatalf("text adapter test failed: %v", err)
	}
}

func TestOutputAdapterLines(t *testing.T) {
	mock := &mockRunner{output: []byte("line1\nline2\nline3\n")}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
				output_adapter = "lines",
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `
		local result = kit.test({})
		assert(result.lines ~= nil, "expected lines field")
		assert(#result.lines == 3, "expected 3 lines, got " .. #result.lines)
		assert(result.lines[1] == "line1", "expected line1, got " .. result.lines[1])
		assert(result.lines[3] == "line3", "expected line3, got " .. result.lines[3])
	`)
	if err != nil {
		t.Fatalf("lines adapter test failed: %v", err)
	}
}

func TestDefaultAdaptersArgsAndText(t *testing.T) {
	// No adapter specified — defaults are args input + text output
	mock := &mockRunner{output: []byte("some plain output")}
	e := newTestEngine(t, mock, map[string]string{
		"init.lua": `
			local M = {}
			M.test = require("test")
			return M
		`,
		"test.lua": `
			return tools.define_tool({
				source = "git@github.com:someone/repo.git",
			})
		`,
	})
	defer e.Close()

	err := e.RunString(context.Background(), `
		local result = kit.test({ name = "world" })
		assert(result.output == "some plain output", "expected text in output field, got " .. tostring(result.output))
	`)
	if err != nil {
		t.Fatalf("default adapter test failed: %v", err)
	}

	// Input should have gone as args, not stdin
	if len(mock.lastInput) > 0 {
		t.Errorf("expected empty stdin for default args adapter, got %q", mock.lastInput)
	}
	if !containsFlag(mock.lastOpts.ExtraArgs, "--name", "world") {
		t.Errorf("expected --name world in ExtraArgs, got %v", mock.lastOpts.ExtraArgs)
	}
}

// containsFlag checks if args contains --key followed by value.
func containsFlag(args []string, key, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}
