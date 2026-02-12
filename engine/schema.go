package engine

import (
	"fmt"
	"strings"

	"github.com/yuin/gopher-lua"
)

// FieldSchema describes a single top-level field in a tool's input or output.
type FieldSchema struct {
	Type        string // "string", "number", "boolean", "array", "object"
	Required    bool
	Description string
}

// ToolSchema holds the declared input and output schemas for a tool.
type ToolSchema struct {
	Input  map[string]FieldSchema
	Output map[string]FieldSchema
}

// parseSchema extracts a ToolSchema from the "schema" field of a define_tool config table.
// Returns nil if no schema is declared.
func parseSchema(L *lua.LState, configTbl *lua.LTable) *ToolSchema {
	schemaVal := L.GetField(configTbl, "schema")
	if schemaVal == lua.LNil {
		return nil
	}
	schemaTbl, ok := schemaVal.(*lua.LTable)
	if !ok {
		return nil
	}

	schema := &ToolSchema{}

	if inputVal := L.GetField(schemaTbl, "input"); inputVal != lua.LNil {
		if inputTbl, ok := inputVal.(*lua.LTable); ok {
			schema.Input = parseFields(L, inputTbl)
		}
	}

	if outputVal := L.GetField(schemaTbl, "output"); outputVal != lua.LNil {
		if outputTbl, ok := outputVal.(*lua.LTable); ok {
			schema.Output = parseFields(L, outputTbl)
		}
	}

	return schema
}

// parseFields parses a Lua table of field definitions.
// Supports shorthand ("string", "number?") and long form ({ type = "string", description = "..." }).
func parseFields(L *lua.LState, tbl *lua.LTable) map[string]FieldSchema {
	fields := make(map[string]FieldSchema)
	tbl.ForEach(func(key, val lua.LValue) {
		name := key.String()
		switch v := val.(type) {
		case lua.LString:
			// Shorthand: "string", "number?", "boolean?", etc.
			fields[name] = parseShorthand(string(v))
		case *lua.LTable:
			// Long form: { type = "string", description = "...", required = false }
			fields[name] = parseLongForm(L, v)
		}
	})
	return fields
}

// parseShorthand parses a type string like "string" or "number?".
// Trailing "?" means optional (not required).
func parseShorthand(s string) FieldSchema {
	required := true
	if strings.HasSuffix(s, "?") {
		required = false
		s = strings.TrimSuffix(s, "?")
	}
	return FieldSchema{
		Type:     s,
		Required: required,
	}
}

// parseLongForm parses a table like { type = "string", description = "node name", required = true }.
// If required is not specified, defaults to true.
func parseLongForm(L *lua.LState, tbl *lua.LTable) FieldSchema {
	fs := FieldSchema{
		Required: true, // default
	}

	if t := L.GetField(tbl, "type"); t != lua.LNil {
		fs.Type = t.String()
	}
	if d := L.GetField(tbl, "description"); d != lua.LNil {
		fs.Description = d.String()
	}
	if r := L.GetField(tbl, "required"); r != lua.LNil {
		if b, ok := r.(lua.LBool); ok {
			fs.Required = bool(b)
		}
	}

	return fs
}

// validateInput checks the merged input table against the schema.
// Returns an error describing the first validation failure, or nil if valid.
func validateInput(L *lua.LState, schema *ToolSchema, merged *lua.LTable) error {
	if schema == nil || schema.Input == nil {
		return nil
	}

	for name, field := range schema.Input {
		val := L.GetField(merged, name)

		if val == lua.LNil {
			if field.Required {
				return fmt.Errorf("missing required field %q (expected %s)", name, field.Type)
			}
			continue
		}

		if err := checkType(name, field.Type, val); err != nil {
			return err
		}
	}

	return nil
}

// checkType validates that a Lua value matches the expected type string.
func checkType(name, expectedType string, val lua.LValue) error {
	switch expectedType {
	case "string":
		if _, ok := val.(lua.LString); !ok {
			return fmt.Errorf("field %q: expected string, got %s", name, val.Type().String())
		}
	case "number":
		if _, ok := val.(lua.LNumber); !ok {
			return fmt.Errorf("field %q: expected number, got %s", name, val.Type().String())
		}
	case "boolean":
		if _, ok := val.(lua.LBool); !ok {
			return fmt.Errorf("field %q: expected boolean, got %s", name, val.Type().String())
		}
	case "array", "object":
		if _, ok := val.(*lua.LTable); !ok {
			return fmt.Errorf("field %q: expected %s, got %s", name, expectedType, val.Type().String())
		}
	}
	return nil
}
