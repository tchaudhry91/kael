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
```

If you have an AI tool configured, kael will analyze the script and generate a complete tool definition automatically. Otherwise, use `--manual` to get a skeleton you can fill in.

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
        output = {
            html_text = "string",
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
| `input_adapter` | `"args"` | How input reaches the script: `"args"` (CLI flags) or `"json"` (stdin) |
| `output_adapter` | `"text"` | How output is read: `"text"`, `"json"`, or `"lines"` |
| `schema` | — | Input/output type declarations for validation |
| `defaults` | — | Default values merged into every call |

### Adapters

**Input adapters** control how parameters are passed to your script:

- `args` — Converts `{name = "world", count = 3}` into `--name world --count 3`. Booleans become standalone flags.
- `json` — Sends the full input as JSON on stdin.

**Output adapters** control how stdout is interpreted:

- `text` — Returns raw stdout as `{output = "..."}`.
- `json` — Parses stdout as JSON.
- `lines` — Splits stdout by newlines into `{lines = [...]}`.

### Schema

Declare types for input and output fields. Fields are required by default; suffix with `?` to make optional:

```lua
schema = {
    input = {
        hostnames = "array",
        port = "number?",
        timeout = "number?",
    },
    output = {
        results = "array",
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
    output = { images = "array" },
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

The `kit` table mirrors your namespace tree, so `kit.infra.analysis.ssl_check(...)` calls the tool at `infra/analysis/ssl_check.lua`. Two helpers are available in every script: `inspect(value)` for pretty-printing and `json_encode(value)` / `json_decode(string)` for JSON conversion.

## Kit management

```sh
# List all tools in the kit
kael kit list

# Validate kit loads without errors
kael kit validate

# Show full details for a tool
kael kit describe infra.analysis.ssl_check

# Remove a tool
kael kit remove misc.download
```

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
