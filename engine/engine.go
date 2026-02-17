package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/tchaudhry91/kael/runtime"
	"github.com/yuin/gopher-lua"
)

// ToolConfig holds the metadata parsed from a define_tool() call in Lua.
type ToolConfig struct {
	Source     string
	Entrypoint string
	SubDir     string
	Executor   string
	Tag        string // git tag, branch, or commit hash
	Type       string // python, node, shell, other
	Schema        *ToolSchema
	InputAdapter  string   // "json" (default), "args", or "positional_args"
	OutputAdapter string   // "json" (default), "text", or "lines"
	ArgsOrder     []string // ordered field names for positional_args adapter
	Defaults     map[string]lua.LValue
	Deps         []string
	Env          []string
	Timeout      int
}

// KitNode represents a node in the kit tree. It can contain tools (leaf functions)
// and children (nested namespaces).
type KitNode struct {
	Tools    map[string]ToolConfig
	Children map[string]*KitNode
}

// Runner is the interface for executing tool implementations.
type Runner interface {
	Run(ctx context.Context, opts runtime.RunOptions, input []byte) ([]byte, error)
}

type Engine struct {
	KitRoot  string
	Refresh  bool
	lstate   *lua.LState
	Runner   Runner
	Registry map[*lua.LFunction]ToolConfig
}

func (e *Engine) Close() {
	e.lstate.Close()
}

func (e *Engine) RunFile(ctx context.Context, path string) error {
	e.lstate.SetContext(ctx)
	defer e.lstate.SetContext(context.Background())
	return e.lstate.DoFile(path)
}

func (e *Engine) RunString(ctx context.Context, code string) error {
	e.lstate.SetContext(ctx)
	defer e.lstate.SetContext(context.Background())
	return e.lstate.DoString(code)
}

// CheckSyntax attempts to compile the code without executing it.
// Returns nil if valid, an error otherwise. Use isIncomplete() to check
// if the error indicates an unterminated statement.
func (e *Engine) CheckSyntax(code string) error {
	top := e.lstate.GetTop()
	_, err := e.lstate.LoadString(code)
	// Restore stack to previous state regardless of outcome
	e.lstate.SetTop(top)
	return err
}

func (e *Engine) RegisterTools() {
	tools := e.lstate.NewTable()
	e.lstate.SetField(tools, "define_tool", e.lstate.NewFunction(e.defineTool))
	e.lstate.SetGlobal("tools", tools)
}

// ListTools walks the kit global and returns a tree of all registered tools.
func (e *Engine) ListTools() *KitNode {
	kit := e.lstate.GetGlobal("kit")
	tbl, ok := kit.(*lua.LTable)
	if !ok {
		return &KitNode{}
	}
	return e.walkTable(tbl)
}

func (e *Engine) walkTable(tbl *lua.LTable) *KitNode {
	node := &KitNode{
		Tools:    make(map[string]ToolConfig),
		Children: make(map[string]*KitNode),
	}
	tbl.ForEach(func(key, val lua.LValue) {
		name := key.String()
		switch v := val.(type) {
		case *lua.LFunction:
			if cfg, ok := e.Registry[v]; ok {
				node.Tools[name] = cfg
			}
		case *lua.LTable:
			node.Children[name] = e.walkTable(v)
		}
	})
	return node
}

// RunStringResult executes code and returns the first return value.
// Used by the REPL for auto-printing expression results.
func (e *Engine) RunStringResult(ctx context.Context, code string) (lua.LValue, error) {
	e.lstate.SetContext(ctx)
	defer e.lstate.SetContext(context.Background())

	top := e.lstate.GetTop()
	fn, err := e.lstate.LoadString(code)
	if err != nil {
		return nil, err
	}
	e.lstate.Push(fn)
	if err := e.lstate.PCall(0, 1, nil); err != nil {
		e.lstate.SetTop(top)
		return nil, err
	}

	ret := e.lstate.Get(-1)
	e.lstate.SetTop(top)
	return ret, nil
}

func (e *Engine) defineTool(L *lua.LState) int {
	configTbl := L.CheckTable(1)

	var cfg ToolConfig

	cfg.Source = L.GetField(configTbl, "source").String()
	if executor := L.GetField(configTbl, "executor"); executor != lua.LNil {
		cfg.Executor = executor.String()
	} else {
		cfg.Executor = "docker"
	}

	if entrypoint := L.GetField(configTbl, "entrypoint"); entrypoint != lua.LNil {
		cfg.Entrypoint = entrypoint.String()
	}

	if subdir := L.GetField(configTbl, "subdir"); subdir != lua.LNil {
		cfg.SubDir = subdir.String()
	}

	if tag := L.GetField(configTbl, "tag"); tag != lua.LNil {
		cfg.Tag = tag.String()
	}

	if ptype := L.GetField(configTbl, "type"); ptype != lua.LNil {
		cfg.Type = ptype.String()
	}

	if defaults := L.GetField(configTbl, "defaults"); defaults != lua.LNil {
		defaultsTbl, ok := defaults.(*lua.LTable)
		if ok {
			cfg.Defaults = make(map[string]lua.LValue)
			defaultsTbl.ForEach(func(key, value lua.LValue) {
				cfg.Defaults[key.String()] = value
			})
		}
	}

	if deps := L.GetField(configTbl, "deps"); deps != lua.LNil {
		depsTbl, ok := deps.(*lua.LTable)
		if ok {
			cfg.Deps = make([]string, 0, depsTbl.Len())
			depsTbl.ForEach(func(_, value lua.LValue) {
				if value != lua.LNil {
					cfg.Deps = append(cfg.Deps, value.String())
				}
			})
		}
	}

	if env := L.GetField(configTbl, "env"); env != lua.LNil {
		envTbl, ok := env.(*lua.LTable)
		if ok {
			cfg.Env = make([]string, 0, envTbl.Len())
			envTbl.ForEach(func(_, value lua.LValue) {
				if value != lua.LNil {
					cfg.Env = append(cfg.Env, value.String())
				}
			})
		}
	}

	if timeout := L.GetField(configTbl, "timeout"); timeout != lua.LNil {
		if num, ok := timeout.(lua.LNumber); ok {
			cfg.Timeout = int(num)
		}
	}

	if ia := L.GetField(configTbl, "input_adapter"); ia != lua.LNil {
		cfg.InputAdapter = ia.String()
	}
	if oa := L.GetField(configTbl, "output_adapter"); oa != lua.LNil {
		cfg.OutputAdapter = oa.String()
	}
	if ao := L.GetField(configTbl, "args_order"); ao != lua.LNil {
		if orderTbl, ok := ao.(*lua.LTable); ok {
			orderTbl.ForEach(func(_, val lua.LValue) {
				cfg.ArgsOrder = append(cfg.ArgsOrder, val.String())
			})
		}
	}

	cfg.Schema = parseSchema(L, configTbl)

	toolFn := L.NewFunction(func(L *lua.LState) int {
		userParamsTbl := L.CheckTable(1)

		merged := L.NewTable()

		if cfg.Defaults != nil {
			for k, v := range cfg.Defaults {
				L.SetField(merged, k, v)
			}
		}

		userParamsTbl.ForEach(func(k, v lua.LValue) {
			L.SetField(merged, k.String(), v)
		})

		if err := validateInput(L, cfg.Schema, merged); err != nil {
			L.RaiseError("Schema Validation Failure: %s", err.Error())
			return 0
		}

		ro := runtime.RunOptions{
			Source:     cfg.Source,
			Entrypoint: cfg.Entrypoint,
			SubDir:     cfg.SubDir,
			Executor:   cfg.Executor,
			Tag:        cfg.Tag,
			Type:       cfg.Type,
			Refresh:    e.Refresh,
			EnvMap:     cfg.Env,
			Deps:       cfg.Deps,
			Timeout:    cfg.Timeout,
		}

		inputB, extraArgs, err := adaptInput(cfg.InputAdapter, merged, cfg.ArgsOrder)
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		if len(extraArgs) > 0 {
			ro.ExtraArgs = extraArgs
		}

		outputB, err := e.Runner.Run(e.lstate.Context(), ro, inputB)
		if err != nil {
			L.RaiseError("Tool Run Failure: %s", err.Error())
			return 0
		}

		output, err := adaptOutput(cfg.OutputAdapter, outputB)
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		outputL := goToLua(L, output)
		L.Push(outputL)

		return 1
	})

	e.Registry[toolFn] = cfg
	L.Push(toolFn)
	return 1
}

// ExecTool resolves a dotted tool path (e.g. "misc.download") to its Lua function
// and calls it with the given input map. Returns the output as a Go value.
func (e *Engine) ExecTool(ctx context.Context, toolPath string, input map[string]any) (any, error) {
	e.lstate.SetContext(ctx)
	defer e.lstate.SetContext(context.Background())

	// Walk kit.<namespace>.<tool> to find the Lua function
	parts := strings.Split(toolPath, ".")
	current := e.lstate.GetGlobal("kit")
	for _, part := range parts {
		tbl, ok := current.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("cannot resolve %q: %q is not a table", toolPath, part)
		}
		current = e.lstate.GetField(tbl, part)
		if current == lua.LNil {
			return nil, fmt.Errorf("tool %q not found", toolPath)
		}
	}

	fn, ok := current.(*lua.LFunction)
	if !ok {
		return nil, fmt.Errorf("%q is not a tool function", toolPath)
	}

	// Build the input table
	inputTbl := goToLua(e.lstate, map[string]any(input))

	if err := e.lstate.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, inputTbl); err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	ret := e.lstate.Get(-1)
	e.lstate.Pop(1)
	return luaToGo(ret), nil
}

func NewEngine(kitRoot string) (*Engine, error) {
	lState := lua.NewState()
	pkg := lState.GetGlobal("package")
	packagePath := lua.LString(fmt.Sprintf("%s/?.lua;%s/?/init.lua", kitRoot, kitRoot))
	lState.SetField(pkg, "path", packagePath)

	e := &Engine{
		KitRoot:  kitRoot,
		lstate:   lState,
		Runner:   runtime.NewDefaultRunner(),
		Registry: make(map[*lua.LFunction]ToolConfig),
	}
	e.RegisterTools()
	e.RegisterHelpers()
	if err := e.lstate.DoString("kit = require(\"init\")"); err != nil {
		return nil, err
	}

	return e, nil
}
