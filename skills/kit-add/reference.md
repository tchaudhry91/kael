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
| `executor` | string | `"docker"` | Execution environment: `"docker"` or `"native"`. Note: `kit add` defaults to `"native"`. |
| `type` | string | auto-detected | Script type: `"python"`, `"shell"`, `"node"`, `"other"`. |
| `subdir` | string | — | Subdirectory within source (for monorepos). |
| `tag` | string | — | Git tag, branch, or commit hash for git sources. |
| `timeout` | number | — | Execution timeout in seconds. |

### Adapters

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `input_adapter` | string | `"args"` | How input is passed to the script. `"args"`: as CLI flags (`--key value`). `"json"`: as JSON on stdin. `"positional_args"`: as positional arguments in `args_order`. |
| `output_adapter` | string | `"text"` | How output is interpreted. `"text"`: raw stdout wrapped as `{output: "..."}`. `"json"`: stdout parsed as JSON. `"lines"`: stdout split by newlines as `{lines: [...]}`. |
| `args_order` | array of strings | — | Required when `input_adapter` is `"positional_args"`. Specifies the order in which input fields are passed as positional arguments. |

### Input adapter details

**`args` (default)**: The input Lua table is converted to CLI flags:
- `{name = "world", count = 3}` → `--name world --count 3`
- Boolean `true` → standalone flag: `{verbose = true}` → `--verbose`
- Boolean `false` → omitted entirely
- No data is sent on stdin

**`json`**: The input Lua table is serialized as JSON and sent on stdin. Nothing is passed as CLI args.

**`positional_args`**: The input Lua table values are passed as positional arguments (no `--` prefix), in the order defined by `args_order`. Any input fields not listed in `args_order` are appended as `--key value` flags after the positional arguments.

Example:
```lua
return tools.define_tool({
    source = "/path/to/scripts",
    entrypoint = "lookup.sh",
    type = "shell",
    input_adapter = "positional_args",
    args_order = {"tenant_name", "region"},
    schema = {
        input = {
            tenant_name = "string",
            region = "string?",
        },
    },
})
```

Calling `kit.lookup({ tenant_name = "acme", region = "us-east-1" })` runs:
```
lookup.sh acme us-east-1
```

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

Supported types: `"string"`, `"number"`, `"boolean"`, `"array"`, `"object"`.

Fields default to required. Use `"type?"` shorthand for optional.

### Other fields

| Field | Type | Description |
|-------|------|-------------|
| `deps` | array of strings | Dependencies to install. For Python: pip packages. For Node: npm packages. For Shell: apk system packages. |
| `env` | array of strings | Environment variable names to pass through to the executor. |

### Naming convention

Always use underscores (`_`) in field names, never hyphens (`-`). For example: `tenant_name`, `from_time`, `resource_group`.
