# define_tool API Reference

## Overview

A kael tool definition is a Lua file that calls `tools.define_tool()` with a configuration table and returns the result. The engine parses this at kit load time and creates a callable Lua function.

```lua
return tools.define_tool({
    source = "/path/to/actions/directory",
    entrypoint = "script.py",
    type = "python",
    -- ... other fields
})
```

## Configuration fields

### Required

| Field | Type | Description |
|-------|------|-------------|
| `source` | string | Path to the directory containing the script. Can be a local absolute path or a git URL (`git@github.com:user/repo.git`). |

### Execution

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `entrypoint` | string | auto-detected | Script filename within the source directory. |
| `executor` | string | `"docker"` | Execution environment: `"docker"` or `"native"`. |
| `type` | string | auto-detected | Script type: `"python"`, `"shell"`, `"node"`, `"other"`. |
| `subdir` | string | — | Subdirectory within source (for monorepos). |
| `tag` | string | — | Git tag, branch, or commit hash for git sources. |
| `timeout` | number | — | Execution timeout in seconds. |

### Adapters

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `input_adapter` | string | `"args"` | How input is passed to the script. `"args"`: as CLI flags (`--key value`). `"json"`: as JSON on stdin. |
| `output_adapter` | string | `"text"` | How output is interpreted. `"text"`: raw stdout wrapped as `{output: "..."}`. `"json"`: stdout parsed as JSON. `"lines"`: stdout split by newlines as `{lines: [...]}`. |

**Important**: Only include adapter fields when they differ from the defaults. If a script uses CLI args for input and produces plain text, omit both adapter fields entirely.

### Input adapter details

**`args` (default)**: The input Lua table is converted to CLI flags:
- `{name = "world", count = 3}` → `--name world --count 3`
- Boolean `true` → standalone flag: `{verbose = true}` → `--verbose`
- Boolean `false` → omitted entirely
- No data is sent on stdin

**`json`**: The input Lua table is serialized as JSON and sent on stdin. Nothing is passed as CLI args.

### Output adapter details

**`text` (default)**: Raw stdout is returned as `{output = "the raw text"}`. Access in Lua as `result.output`.

**`json`**: Stdout is parsed as JSON and returned as a Lua table. Fields are accessed directly: `result.count`, `result.pods`, etc.

**`lines`**: Stdout is split on newlines and returned as `{lines = {"line1", "line2", ...}}`. Access in Lua as `result.lines`.

### Schema

| Field | Type | Description |
|-------|------|-------------|
| `schema` | table | Optional. Declares input and output field types for validation and documentation. |

Schema has two sub-tables: `input` and `output`. Each maps field names to type declarations.

**Shorthand syntax**: A type string, with optional `?` suffix for optional fields.

```lua
schema = {
    input = {
        node = "string",       -- required string
        namespace = "string?", -- optional string
        limit = "number?",     -- optional number
    },
}
```

**Long form**: A table with `type`, `description`, and optionally `required`.

```lua
schema = {
    input = {
        node = { type = "string", description = "target node name" },
        limit = { type = "number", required = false, description = "max results" },
    },
}
```

Supported types: `"string"`, `"number"`, `"boolean"`, `"array"`, `"object"`.

Fields default to required. Use `"type?"` shorthand or `required = false` for optional.

The engine validates input against the schema before dispatching. Missing required fields or type mismatches raise a clear Lua error.

### Other fields

| Field | Type | Description |
|-------|------|-------------|
| `defaults` | table | Default values merged with user input. User values override defaults. |
| `deps` | array of strings | Dependencies to install. For Python: pip packages. For Node: npm packages. For Shell: apk system packages. |
| `env` | array of strings | Environment variable names to pass through to the executor. |

## Defaults pattern

For tools sharing common config (same source, executor, etc.), use a defaults factory:

```lua
-- namespace/defaults.lua
return function()
    return {
        source = "/path/to/actions/namespace",
        executor = "native",
        input_adapter = "json",
        output_adapter = "json",
    }
end
```

Then each tool inherits and extends:

```lua
-- namespace/my_tool.lua
local base = require("namespace.defaults")()
base.entrypoint = "my_tool.py"
base.type = "python"
base.schema = { ... }
return tools.define_tool(base)
```

This is optional — only use it when multiple tools share the same source and executor.

## init.lua wiring

Each namespace has an `init.lua` that requires its tools:

```lua
-- namespace/init.lua
local M = {}
M.my_tool = require("namespace.my_tool")
M.other_tool = require("namespace.other_tool")
return M
```

The top-level `init.lua` requires namespaces:

```lua
-- init.lua
local M = {}
M.namespace = require("namespace")
return M
```

Tools are then called as `kit.namespace.my_tool({ ... })` in Lua scripts.

When adding a tool, insert a new `M.<name> = require("<namespace>.<name>")` line before the `return M` in the namespace's `init.lua`. Check that the line doesn't already exist before adding it.
