package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/yuin/gopher-lua"
)

// RegisterHelpers registers all global helper functions into the Lua state.
func (e *Engine) RegisterHelpers() {
	// json
	jsonTbl := e.lstate.NewTable()
	e.lstate.SetField(jsonTbl, "encode", e.lstate.NewFunction(e.jsonEncode))
	e.lstate.SetField(jsonTbl, "pretty", e.lstate.NewFunction(e.jsonEncodePretty))
	e.lstate.SetField(jsonTbl, "decode", e.lstate.NewFunction(e.jsonDecode))
	e.lstate.SetGlobal("json", jsonTbl)

	// inspect
	e.lstate.SetGlobal("inspect", e.lstate.NewFunction(e.inspect))

	// pp — pretty-print to stdout
	e.lstate.SetGlobal("pp", e.lstate.NewFunction(e.ppHelper))

	// table utilities
	e.lstate.SetGlobal("keys", e.lstate.NewFunction(e.keysHelper))
	e.lstate.SetGlobal("pluck", e.lstate.NewFunction(e.pluckHelper))
	e.lstate.SetGlobal("count", e.lstate.NewFunction(e.countHelper))

	// jq
	e.lstate.SetGlobal("jq", e.lstate.NewFunction(e.jqHelper))

	// file I/O
	e.lstate.SetGlobal("writefile", e.lstate.NewFunction(e.writefileHelper))
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

func (e *Engine) jsonDecode(L *lua.LState) int {
	str := L.CheckString(1)
	var val any
	if err := json.Unmarshal([]byte(str), &val); err != nil {
		L.RaiseError("JSON Decode Failure: %s", err.Error())
	}
	L.Push(goToLua(L, val))
	return 1
}

func (e *Engine) inspect(L *lua.LState) int {
	val := L.CheckAny(1)
	L.Push(lua.LString(formatLuaValue(val, "")))
	return 1
}

// pp(val, depth?) — pretty-print a value to stdout. Default depth 4.
func (e *Engine) ppHelper(L *lua.LState) int {
	val := L.CheckAny(1)
	depth := L.OptInt(2, 4)
	fmt.Println(FormatLuaValuePP(val, depth))
	return 0
}

// keys(tbl) — return a list of all keys in a table.
func (e *Engine) keysHelper(L *lua.LState) int {
	tbl := L.CheckTable(1)
	result := L.NewTable()
	tbl.ForEach(func(key, _ lua.LValue) {
		result.Append(key)
	})
	L.Push(result)
	return 1
}

// pluck(list, field) — extract one field from each table in a list.
func (e *Engine) pluckHelper(L *lua.LState) int {
	list := L.CheckTable(1)
	field := L.CheckString(2)
	result := L.NewTable()
	list.ForEach(func(_, val lua.LValue) {
		if tbl, ok := val.(*lua.LTable); ok {
			v := L.GetField(tbl, field)
			if v != lua.LNil {
				result.Append(v)
			}
		}
	})
	L.Push(result)
	return 1
}

// count(tbl) — count all entries in a table (works for both arrays and maps).
func (e *Engine) countHelper(L *lua.LState) int {
	tbl := L.CheckTable(1)
	n := 0
	tbl.ForEach(func(_, _ lua.LValue) { n++ })
	L.Push(lua.LNumber(n))
	return 1
}

// jq(val, filter) — pipe a value through the jq binary with the given filter.
func (e *Engine) jqHelper(L *lua.LState) int {
	val := L.CheckAny(1)
	filter := L.CheckString(2)

	goVal := luaToGo(val)
	data, err := json.Marshal(goVal)
	if err != nil {
		L.RaiseError("jq: marshal failed: %s", err.Error())
		return 0
	}

	cmd := exec.Command("jq", filter)
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		L.RaiseError("jq: %s", msg)
		return 0
	}

	raw := bytes.TrimSpace(stdout.Bytes())

	// Try to parse as JSON back into Lua
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		// Return as plain string
		L.Push(lua.LString(string(raw)))
		return 1
	}
	L.Push(goToLua(L, result))
	return 1
}

// writefile(path, content) — write content to a file.
// If content is a table, serialize as pretty JSON. If string, write as-is.
func (e *Engine) writefileHelper(L *lua.LState) int {
	path := L.CheckString(1)
	val := L.CheckAny(2)

	var data []byte
	switch v := val.(type) {
	case lua.LString:
		data = []byte(string(v))
	case *lua.LTable:
		goVal := luaToGo(v)
		var err error
		data, err = json.MarshalIndent(goVal, "", "  ")
		if err != nil {
			L.RaiseError("writefile: marshal failed: %s", err.Error())
			return 0
		}
		data = append(data, '\n')
	default:
		data = []byte(val.String())
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		L.RaiseError("writefile: %s", err.Error())
		return 0
	}
	return 0
}
