package engine

import (
	"testing"

	"github.com/yuin/gopher-lua"
)

func TestLuaToGo(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	t.Run("nil", func(t *testing.T) {
		got := luaToGo(lua.LNil)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("bool", func(t *testing.T) {
		got := luaToGo(lua.LTrue)
		if got != true {
			t.Errorf("got %v, want true", got)
		}
	})

	t.Run("number", func(t *testing.T) {
		got := luaToGo(lua.LNumber(42.5))
		if got != 42.5 {
			t.Errorf("got %v, want 42.5", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		got := luaToGo(lua.LString("hello"))
		if got != "hello" {
			t.Errorf("got %v, want hello", got)
		}
	})

	t.Run("map table", func(t *testing.T) {
		tbl := L.NewTable()
		L.SetField(tbl, "name", lua.LString("test"))
		L.SetField(tbl, "count", lua.LNumber(3))
		L.SetField(tbl, "active", lua.LTrue)

		got := luaToGo(tbl)
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("got type %T, want map[string]any", got)
		}
		if m["name"] != "test" {
			t.Errorf("name: got %v, want test", m["name"])
		}
		if m["count"] != float64(3) {
			t.Errorf("count: got %v, want 3", m["count"])
		}
		if m["active"] != true {
			t.Errorf("active: got %v, want true", m["active"])
		}
	})

	t.Run("array table", func(t *testing.T) {
		tbl := L.NewTable()
		tbl.Append(lua.LString("a"))
		tbl.Append(lua.LString("b"))
		tbl.Append(lua.LNumber(3))

		got := luaToGo(tbl)
		arr, ok := got.([]any)
		if !ok {
			t.Fatalf("got type %T, want []any", got)
		}
		if len(arr) != 3 {
			t.Fatalf("len: got %d, want 3", len(arr))
		}
		if arr[0] != "a" {
			t.Errorf("[0]: got %v, want a", arr[0])
		}
		if arr[1] != "b" {
			t.Errorf("[1]: got %v, want b", arr[1])
		}
		if arr[2] != float64(3) {
			t.Errorf("[2]: got %v, want 3", arr[2])
		}
	})

	t.Run("nested table", func(t *testing.T) {
		inner := L.NewTable()
		L.SetField(inner, "x", lua.LNumber(1))

		outer := L.NewTable()
		L.SetField(outer, "inner", inner)

		got := luaToGo(outer)
		m := got.(map[string]any)
		nested := m["inner"].(map[string]any)
		if nested["x"] != float64(1) {
			t.Errorf("inner.x: got %v, want 1", nested["x"])
		}
	})
}

func TestGoToLua(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	t.Run("nil", func(t *testing.T) {
		got := goToLua(L, nil)
		if got != lua.LNil {
			t.Errorf("got %v, want LNil", got)
		}
	})

	t.Run("bool", func(t *testing.T) {
		got := goToLua(L, true)
		if got != lua.LTrue {
			t.Errorf("got %v, want LTrue", got)
		}
	})

	t.Run("number", func(t *testing.T) {
		got := goToLua(L, float64(99))
		if got != lua.LNumber(99) {
			t.Errorf("got %v, want 99", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		got := goToLua(L, "hello")
		if got != lua.LString("hello") {
			t.Errorf("got %v, want hello", got)
		}
	})

	t.Run("slice to array table", func(t *testing.T) {
		got := goToLua(L, []any{"a", float64(2), true})
		tbl, ok := got.(*lua.LTable)
		if !ok {
			t.Fatalf("got type %T, want *LTable", got)
		}
		if tbl.Len() != 3 {
			t.Fatalf("len: got %d, want 3", tbl.Len())
		}
		if tbl.RawGetInt(1) != lua.LString("a") {
			t.Errorf("[1]: got %v, want a", tbl.RawGetInt(1))
		}
		if tbl.RawGetInt(2) != lua.LNumber(2) {
			t.Errorf("[2]: got %v, want 2", tbl.RawGetInt(2))
		}
		if tbl.RawGetInt(3) != lua.LTrue {
			t.Errorf("[3]: got %v, want true", tbl.RawGetInt(3))
		}
	})

	t.Run("map to table", func(t *testing.T) {
		got := goToLua(L, map[string]any{"key": "val", "num": float64(5)})
		tbl, ok := got.(*lua.LTable)
		if !ok {
			t.Fatalf("got type %T, want *LTable", got)
		}
		if L.GetField(tbl, "key") != lua.LString("val") {
			t.Errorf("key: got %v, want val", L.GetField(tbl, "key"))
		}
		if L.GetField(tbl, "num") != lua.LNumber(5) {
			t.Errorf("num: got %v, want 5", L.GetField(tbl, "num"))
		}
	})

	t.Run("roundtrip", func(t *testing.T) {
		// Go → Lua → Go should preserve types
		original := map[string]any{
			"name":   "test",
			"count":  float64(42),
			"active": true,
			"tags":   []any{"a", "b"},
		}
		lv := goToLua(L, original)
		back := luaToGo(lv)

		m := back.(map[string]any)
		if m["name"] != "test" {
			t.Errorf("name: got %v", m["name"])
		}
		if m["count"] != float64(42) {
			t.Errorf("count: got %v", m["count"])
		}
		if m["active"] != true {
			t.Errorf("active: got %v", m["active"])
		}
		tags := m["tags"].([]any)
		if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
			t.Errorf("tags: got %v", tags)
		}
	})
}
