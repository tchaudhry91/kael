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
		KitRoot: kitDir,
		LState:  L,
		Runner:  mock,
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
	err := e.LState.DoString(`kit.test({ query = "up" })`)
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
	if !mock.lastOpts.Autogen {
		t.Error("autogen should be true")
	}
	if len(mock.lastOpts.EnvMap) != 2 || mock.lastOpts.EnvMap[0] != "AWS_PROFILE" || mock.lastOpts.EnvMap[1] != "HOME" {
		t.Errorf("env: got %v", mock.lastOpts.EnvMap)
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

	err := e.LState.DoString(`kit.test({})`)
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
				defaults = {
					endpoint = "http://default:9090",
					step = 5,
				},
			})
		`,
	})
	defer e.Close()

	err := e.LState.DoString(`kit.test({ endpoint = "http://custom:9090", query = "up" })`)
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
			})
		`,
	})
	defer e.Close()

	// Call the tool and inspect the return value in Lua
	err := e.LState.DoString(`
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
	err := e.LState.DoString(`kit.test({})`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// pcall should catch it
	err = e.LState.DoString(`
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
			})
		`,
	})
	defer e.Close()

	err := e.LState.DoString(`
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
				defaults = {
					endpoint = "http://prometheus:9090",
				},
			})
		`,
	})
	defer e.Close()

	err := e.LState.DoString(`
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
