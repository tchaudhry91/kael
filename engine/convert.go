package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/gopher-lua"
)

// luaToGo converts a Lua value to a native Go type suitable for JSON serialization.
// LTable → map[string]any or []any, LNumber → float64, LBool → bool, LString → string.
func luaToGo(lv lua.LValue) any {
	switch v := lv.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		maxN := v.MaxN()
		if maxN > 0 {
			arr := make([]any, 0, maxN)
			for i := 1; i <= maxN; i++ {
				arr = append(arr, luaToGo(v.RawGetInt(i)))
			}
			return arr
		}
		m := make(map[string]any)
		v.ForEach(func(key, val lua.LValue) {
			m[key.String()] = luaToGo(val)
		})
		return m
	default:
		return nil
	}
}

// formatLuaValue produces a human-readable string representation of a Lua value.
func formatLuaValue(lv lua.LValue, indent string) string {
	switch v := lv.(type) {
	case *lua.LNilType:
		return "nil"
	case lua.LBool:
		return fmt.Sprintf("%v", bool(v))
	case lua.LNumber:
		return fmt.Sprintf("%g", float64(v))
	case lua.LString:
		return fmt.Sprintf("%q", string(v))
	case *lua.LTable:
		if v.MaxN() == 0 && v.Len() == 0 {
			// Check if truly empty
			empty := true
			v.ForEach(func(_, _ lua.LValue) { empty = false })
			if empty {
				return "{}"
			}
		}
		var lines []string
		nextIndent := indent + "  "

		// Array part
		maxN := v.MaxN()
		for i := 1; i <= maxN; i++ {
			lines = append(lines, nextIndent+formatLuaValue(v.RawGetInt(i), nextIndent))
		}

		// Map part (collect and sort keys for stable output)
		var keys []string
		v.ForEach(func(key, _ lua.LValue) {
			if num, ok := key.(lua.LNumber); ok && int(num) >= 1 && int(num) <= maxN {
				return // skip array keys
			}
			keys = append(keys, key.String())
		})
		sort.Strings(keys)
		for _, k := range keys {
			val := v.RawGetString(k)
			lines = append(lines, fmt.Sprintf("%s%s = %s", nextIndent, k, formatLuaValue(val, nextIndent)))
		}

		return "{\n" + strings.Join(lines, ",\n") + "\n" + indent + "}"
	default:
		return lv.String()
	}
}

// goToLua converts a native Go value (typically from JSON unmarshaling) to a Lua value.
// map[string]any → LTable, []any → LTable, float64 → LNumber, bool → LBool, string → LString.
func goToLua(L *lua.LState, value any) lua.LValue {
	switch v := value.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []any:
		tbl := L.NewTable()
		for _, item := range v {
			tbl.Append(goToLua(L, item))
		}
		return tbl
	case map[string]any:
		tbl := L.NewTable()
		for key, val := range v {
			L.SetField(tbl, key, goToLua(L, val))
		}
		return tbl
	default:
		return lua.LNil
	}
}
