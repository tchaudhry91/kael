# kael

Scriptable automation engine — Lua orchestration over sandboxed polyglot actions.

Kael lets you wrap scripts written in any language (Python, Node.js, Shell) into a composable **kit** of tools, then run them individually or orchestrate them together with Lua. Each tool runs in Docker for isolation or natively for speed — your choice.

## Install

Grab the latest binary for your platform from [GitHub Releases](https://github.com/tchaudhry91/kael/releases). Kael ships as a single binary with no runtime dependencies beyond what your tools need.

```sh
# Example: Linux amd64
tar xzf kael_*_linux_amd64.tar.gz
sudo mv kael /usr/local/bin/
```

## Quick start

### 1. Run setup

```sh
kael setup
```

This walks you through initial configuration: where your kit lives, and optionally wiring up an AI tool (Claude Code or OpenCode) to help analyze scripts when you onboard them.

Configuration is written to `~/.kael/config.yaml`. You can also set any value via `KAEL_` environment variables (e.g. `KAEL_KIT=/path/to/kit`).

### 2. Create a namespace

Tools live in namespaces — think of them as folders in a tree. Initialize one:

```sh
kael kit init misc
```

This creates `~/.kael/kit/misc/` with an `init.lua` and wires it into the root kit. Namespaces can be nested:

```sh
kael kit init infra.kubernetes
```

### 3. Add a tool

Point kael at a script (local file or git repo) and tell it which namespace to put it in:

```sh
# From a local script
kael kit add ./scripts/download.py misc

# From a git repo
kael kit add git@github.com:user/repo.git misc

# From a monorepo — use // to specify a path within the repo
kael kit add git@github.com:org/scripts.git//tools/check.py misc
```

If you have an AI tool configured, kael will analyze the script and generate a complete tool definition automatically. Additional flags:

| Flag | Description |
|---|---|
| `--manual` | Skip AI analysis, generate a skeleton definition |
| `--force` | Overwrite an existing tool definition |
| `--prompt "..."` | Additional instructions for the AI analysis |
| `--executor native` | Override executor (native, docker) |
| `--entrypoint main.py` | Override entrypoint script filename |
| `--subdir tools/` | Override subdirectory within source |
| `--tag v1.2.0` | Pin to a git tag, branch, or commit |
| `--type python` | Override script type (python, shell, node) |

Override flags are applied after AI analysis, so you can let the AI do most of the work and just correct specific fields.

### 4. Run it

```sh
# Execute a single tool
kael exec misc.download --url https://example.com

# Pipe JSON input
echo '{"url": "https://example.com"}' | kael exec misc.download
```

Output comes back as JSON on stdout, so you can pipe it into `jq` or other tools.

## Writing tool definitions

Under the hood, each tool is a Lua file that calls `tools.define_tool()`. Here's a complete example:

```lua
-- ~/.kael/kit/misc/download.lua
return tools.define_tool({
    source = "git@github.com:user/python-downloader.git",
    entrypoint = "download.py",
    type = "python",
    deps = {"requests"},
    input_adapter = "json",
    output_adapter = "json",
    schema = {
        input = {
            url = "string",
        },
    },
})
```

### Fields

| Field | Default | Description |
|---|---|---|
| `source` | *(required)* | Local path or git URL to the script directory |
| `entrypoint` | auto-detected | Script filename to run |
| `type` | auto-detected | `"python"`, `"node"`, or `"shell"` |
| `executor` | `"docker"` | `"docker"` for isolation, `"native"` for speed |
| `tag` | — | Git tag, branch, or commit hash |
| `subdir` | — | Subdirectory within source (for monorepos) |
| `timeout` | — | Execution timeout in seconds |
| `deps` | — | Packages to install (pip / npm / apk) |
| `env` | — | Host environment variables to pass through |
| `input_adapter` | `"args"` | How input reaches the script: `"args"`, `"json"`, or `"positional_args"` |
| `args_order` | — | Ordered field names for `positional_args` adapter |
| `output_adapter` | `"text"` | How output is read: `"text"`, `"json"`, or `"lines"` |
| `schema` | — | Input type declarations for validation |
| `defaults` | — | Default values merged into every call |

### Adapters

**Input adapters** control how parameters are passed to your script:

- `args` — Converts `{name = "world", count = 3}` into `--name world --count 3`. Booleans become standalone flags.
- `json` — Sends the full input as JSON on stdin.
- `positional_args` — Emits values as bare positional arguments in the order specified by `args_order`. Requires `args_order` to be set. Any remaining keys not in `args_order` are appended as `--key value` flags.

```lua
-- Script uses $1 for tenant and $2 for region
input_adapter = "positional_args",
args_order = {"tenant_name", "region"},
```

**Output adapters** control how stdout is interpreted:

- `text` — Returns raw stdout as `{output = "..."}`.
- `json` — Parses stdout as JSON.
- `lines` — Splits stdout by newlines into `{lines = [...]}`.

### Schema

Declare types for input fields. Fields are required by default; suffix with `?` to make optional:

```lua
schema = {
    input = {
        hostnames = "array",
        port = "number?",
        timeout = "number?",
    },
}
```

Supported types: `string`, `number`, `boolean`, `array`, `object`.

### Sharing defaults across tools

If several tools share the same source or executor, extract a defaults function:

```lua
-- infra/kubernetes/defaults.lua
return function()
    return {
        source = "/path/to/kubernetes/scripts",
        executor = "native",
    }
end

-- infra/kubernetes/images_in_use.lua
local base = require("infra.kubernetes.defaults")()
base.entrypoint = "images_in_use.sh"
base.type = "shell"
base.schema = {
    input = { namespace = "string?" },
}
return tools.define_tool(base)
```

## Lua scripting

For more complex workflows, write a Lua script that calls multiple tools:

```lua
-- scan.lua
local results = kit.infra.analysis.url_probe({
    urls = {"https://example.com", "https://example.org"},
    timeout = 10,
})

for _, r in ipairs(results.results) do
    print(r.url, r.status_code)
end

local certs = kit.infra.analysis.ssl_check({
    hostnames = {"example.com", "example.org"},
})

print(inspect(certs))
```

Run it:

```sh
kael run scan.lua
```

The `kit` table mirrors your namespace tree, so `kit.infra.analysis.ssl_check(...)` calls the tool at `infra/analysis/ssl_check.lua`.

### Built-in helpers

These are available in all Lua scripts and in the REPL:

| Function | Description |
|---|---|
| `pp(val, depth?)` | Pretty-print a value to stdout (default depth 4) |
| `inspect(val)` | Return a string representation of any value |
| `keys(tbl)` | Return a list of all keys in a table |
| `pluck(list, field)` | Extract one field from each table in a list |
| `count(tbl)` | Count entries in a table (arrays and maps) |
| `jq(val, filter)` | Pipe a value through `jq` and return the result |
| `readfile(path)` | Read a file, return contents as a string |
| `writefile(path, str)` | Write a string to a file |
| `json.encode(val)` | Serialize to JSON string |
| `json.pretty(val)` | Serialize to indented JSON string |
| `json.decode(str)` | Parse a JSON string |

## Interactive REPL

Start an interactive session to explore your kit and run tools:

```sh
kael repl
```

```
kael> kit.infra.azure.get_tenant_resources({tenant_name = "acme"})
{
  name = "my-vm",
  type = "Microsoft.Compute/virtualMachines",
  location = "eastus"
}
```

Features:
- **Tab completion** on `kit.*` tool paths
- **Auto-print** — expressions display their result automatically, no `print()` needed
- **Multiline** — blocks (`if`/`for`/`function`) are detected and continued with indentation
- **History** — persisted to `~/.kael/history`
- Type `help` for a list of all available helpers

## Kit management

```sh
# List all tools in the kit
kael kit list

# Validate kit loads without errors
kael kit validate

# Show full details for a tool
kael kit describe infra.analysis.ssl_check

# Open a tool definition in $EDITOR
kael kit edit infra.analysis.ssl_check

# Remove a tool
kael kit remove misc.download
```

## Refreshing sources

Git-sourced tools are cloned once to `~/.kael/cache/` and Docker images are built once. To force a re-fetch and rebuild, use `--refresh`:

```sh
kael run --refresh scan.lua
kael exec --refresh infra.ssl_check --hostname example.com
kael repl --refresh
```

This runs `git fetch` + reset on cached repos and forces a fresh `docker build`.

## Configuration

Kael reads configuration from `~/.kael/config.yaml`, environment variables (`KAEL_` prefix), and CLI flags — in that order of precedence.

```yaml
# ~/.kael/config.yaml
kit: /home/user/.kael/kit

ai:
  tool: claude
  command: "claude -p"
  skill_dir: /home/user/.claude/skills
```

| Setting | Flag | Env var | Description |
|---|---|---|---|
| `kit` | `--kit` | `KAEL_KIT` | Path to the kit directory |
| `ai.tool` | — | `KAEL_AI_TOOL` | AI tool name (`claude` or `opencode`) |
| `ai.command` | — | `KAEL_AI_COMMAND` | Command to invoke the AI tool |
| `ai.skill_dir` | — | `KAEL_AI_SKILL_DIR` | Where AI skills are installed |

## Requirements

Kael itself is a single binary. Depending on how you run your tools, you'll also need:

- **Docker or Podman** — for containerized execution (default)
- **Git** — for tools sourced from git repositories
- **Python 3 / Node.js / Shell** — for native execution of the respective tool types
