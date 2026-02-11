package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tchaudhry91/kael/envyr"
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
	Defaults   map[string]lua.LValue
	Env        []string
	Timeout    int
}

// Runner is the interface for executing tool implementations.
// *envyr.Client satisfies this.
type Runner interface {
	Run(ctx context.Context, opts envyr.RunOptions, input []byte) ([]byte, error)
}

type Engine struct {
	KitRoot string
	Refresh bool
	lstate  *lua.LState
	Runner  Runner
}

func (e *Engine) Close() {
	e.lstate.Close()
}

func (e *Engine) RunFile(ctx context.Context, path string) error {
	e.lstate.SetContext(ctx)
	defer e.lstate.SetContext(nil)
	return e.lstate.DoFile(path)
}

func (e *Engine) RunString(ctx context.Context, code string) error {
	e.lstate.SetContext(ctx)
	defer e.lstate.SetContext(nil)
	return e.lstate.DoString(code)
}

func (e *Engine) RegisterTools() {
	tools := e.lstate.NewTable()
	e.lstate.SetField(tools, "define_tool", e.lstate.NewFunction(e.defineTool))
	e.lstate.SetGlobal("tools", tools)
}

func (e *Engine) RegisterHelpers() {
	// json
	jsonTbl := e.lstate.NewTable()
	e.lstate.SetField(jsonTbl, "encode", e.lstate.NewFunction(e.jsonEncode))
	e.lstate.SetField(jsonTbl, "pretty", e.lstate.NewFunction(e.jsonEncodePretty))
	e.lstate.SetGlobal("json", jsonTbl)
}

func (e *Engine) jsonEncode(L *lua.LState) int {
	val := L.CheckAny(1)
	valG := luaToGo(val)
	data, err := json.Marshal(valG)
	if err != nil {
		L.RaiseError("Data Marshal Failure: %s", err.Error())
	}
	L.Push(lua.LString(string(data)))
	return 1
}

func (e *Engine) jsonEncodePretty(L *lua.LState) int {
	val := L.CheckAny(1)
	valG := luaToGo(val)
	data, err := json.MarshalIndent(valG, "", "  ")
	if err != nil {
		L.RaiseError("Data Marshal Failure: %s", err.Error())
	}
	L.Push(lua.LString(string(data)))
	return 1
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

		// Map the config to envyr RunOptions
		ro := envyr.RunOptions{
			Source:  cfg.Source,
			Autogen: true,
			Refresh: e.Refresh,
			EnvMap:  cfg.Env,
			Timeout: cfg.Timeout,
		}

		if cfg.Entrypoint != "" {
			ro.Entrypoint = cfg.Entrypoint
		}

		if cfg.SubDir != "" {
			ro.SubDir = cfg.SubDir
		}

		if cfg.Executor != "" {
			ro.Executor = envyr.Executor(cfg.Executor)
		}

		if cfg.Tag != "" {
			ro.Tag = cfg.Tag
		}

		if cfg.Type != "" {
			ro.Type = cfg.Type
		}

		input := luaToGo(merged)
		inputB, err := json.Marshal(input)
		if err != nil {
			L.RaiseError("Data Marshal Failure: %s", err.Error())
			return 0
		}
		outputB, err := e.Runner.Run(e.lstate.Context(), ro, inputB)
		if err != nil {
			L.RaiseError("Tool Run Failure: %s", err.Error())
			return 0
		}

		var output any
		err = json.Unmarshal(outputB, &output)
		if err != nil {
			L.RaiseError("Data UnMarshal Failure: %s", err.Error())
			return 0
		}
		outputL := goToLua(L, output)
		L.Push(outputL)

		return 1
	})

	L.Push(toolFn)
	return 1
}

func NewEngine(kitRoot string) (*Engine, error) {
	lState := lua.NewState()
	pkg := lState.GetGlobal("package")
	packagePath := lua.LString(fmt.Sprintf("%s/?.lua;%s/?/init.lua", kitRoot, kitRoot))
	lState.SetField(pkg, "path", packagePath)

	e := &Engine{
		KitRoot: kitRoot,
		lstate:  lState,
		Runner:  envyr.NewDefaultClient(),
	}
	e.RegisterTools()
	e.RegisterHelpers()
	if err := e.lstate.DoString("kit = require(\"init\")"); err != nil {
		return nil, err
	}

	return e, nil
}
