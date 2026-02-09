package engine

import (
	"fmt"

	"github.com/yuin/gopher-lua"
)

type Engine struct {
	KitRoot string
	LState  *lua.LState
}

func (e *Engine) Close() {
	e.LState.Close()
}

func NewEngine(kitRoot string) (*Engine, error) {
	lState := lua.NewState()
	pkg := lState.GetGlobal("package")
	packagePath := lua.LString(fmt.Sprintf("%s/?.lua;%s/?/init.lua", kitRoot, kitRoot))
	lState.SetField(pkg, "path", packagePath)
	if err := lState.DoString("kit = require(\"init\")"); err != nil {
		return nil, err
	}

	return &Engine{
		KitRoot: kitRoot,
		LState:  lState,
	}, nil
}
