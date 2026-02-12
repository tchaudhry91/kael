package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yuin/gopher-lua"
)

// Input adapter types
const (
	InputAdapterJSON = "json" // default: serialize to JSON on stdin
	InputAdapterArgs = "args" // convert table to --key value CLI flags
)

// Output adapter types
const (
	OutputAdapterJSON  = "json"  // default: parse stdout as JSON
	OutputAdapterText  = "text"  // wrap raw stdout as {"output": "..."}
	OutputAdapterLines = "lines" // split on newlines as {"lines": [...]}
)

// adaptInput converts a merged Lua table into stdin bytes and optional extra args,
// based on the input adapter type.
func adaptInput(inputAdapter string, merged *lua.LTable) (stdinBytes []byte, extraArgs []string, err error) {
	switch inputAdapter {
	case InputAdapterJSON:
		input := luaToGo(merged)
		data, err := json.Marshal(input)
		if err != nil {
			return nil, nil, fmt.Errorf("Data Marshal Failure: %s", err.Error())
		}
		return data, nil, nil
	default: // args
		return argsFromTable(merged)
	}
}

// argsFromTable converts a Lua table into CLI flag pairs.
// String/number values become --key value. Boolean true becomes --key (flag only).
// Boolean false is omitted. Array values repeat the flag.
func argsFromTable(tbl *lua.LTable) (stdinBytes []byte, extraArgs []string, err error) {
	var args []string
	tbl.ForEach(func(key, val lua.LValue) {
		name := "--" + key.String()
		switch v := val.(type) {
		case lua.LBool:
			if bool(v) {
				args = append(args, name)
			}
		case lua.LNumber:
			args = append(args, name, fmt.Sprintf("%g", float64(v)))
		case lua.LString:
			args = append(args, name, string(v))
		case *lua.LTable:
			// Array: repeat the flag for each element
			v.ForEach(func(_, item lua.LValue) {
				args = append(args, name, item.String())
			})
		}
	})
	return nil, args, nil
}

// adaptOutput converts raw runner output bytes into a Go value suitable for goToLua,
// based on the output adapter type.
func adaptOutput(outputAdapter string, outputB []byte) (any, error) {
	switch outputAdapter {
	case OutputAdapterJSON:
		var output any
		if err := json.Unmarshal(outputB, &output); err != nil {
			return nil, fmt.Errorf("Data UnMarshal Failure: %s", err.Error())
		}
		return output, nil
	case OutputAdapterLines:
		raw := strings.TrimRight(string(outputB), "\n")
		var lines []any
		if raw != "" {
			for _, line := range strings.Split(raw, "\n") {
				lines = append(lines, line)
			}
		}
		return map[string]any{"lines": lines}, nil
	default: // text
		return map[string]any{"output": string(outputB)}, nil
	}
}
