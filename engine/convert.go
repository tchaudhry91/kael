package engine

import (
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
