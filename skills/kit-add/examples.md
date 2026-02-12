# Tool Definition Examples

## Python script with JSON stdin/stdout

A Python script that reads JSON from stdin and outputs JSON:

**Script** (`pods_on_node.py`):
```python
import json, subprocess, sys

def main():
    params = json.load(sys.stdin)
    node = params["node"]
    namespace = params.get("namespace", "")
    # ... runs kubectl, processes results ...
    json.dump({"node": node, "count": len(pods), "pods": pods}, sys.stdout)
```

**Tool definition** (`kubernetes/pods_on_node.lua`):
```lua
local base = require("kubernetes.defaults")()
base.entrypoint = "pods_on_node.py"
base.type = "python"
base.input_adapter = "json"
base.output_adapter = "json"
base.schema = {
    input = {
        node = "string",
        namespace = "string?",
    },
    output = {
        node = "string",
        count = "number",
        pods = "array",
    },
}
return tools.define_tool(base)
```

Note: `input_adapter = "json"` and `output_adapter = "json"` are explicit here because the defaults are `args` and `text`.

## Python script with argparse (CLI args)

A Python script that uses argparse for input and outputs JSON:

**Script** (`greet.py`):
```python
import argparse, json, sys

parser = argparse.ArgumentParser()
parser.add_argument("--name", required=True)
parser.add_argument("--count", type=int, default=1)
parser.add_argument("--loud", action="store_true")
args = parser.parse_args()

greeting = f"Hello, {args.name}!"
if args.loud:
    greeting = greeting.upper()

json.dump({"greeting": greeting, "count": args.count}, sys.stdout)
```

**Tool definition** (`greetings/greet.lua`):
```lua
return tools.define_tool({
    source = "/path/to/actions/greetings",
    entrypoint = "greet.py",
    executor = "native",
    type = "python",
    output_adapter = "json",
    schema = {
        input = {
            name = "string",
            count = "number?",
            loud = "boolean?",
        },
        output = {
            greeting = "string",
            count = "number",
        },
    },
})
```

Note: `input_adapter` is omitted because `args` is the default. `output_adapter = "json"` is set because the script outputs JSON (not plain text).

## Bash script with jq (JSON stdin/stdout)

A bash script that reads JSON from stdin via jq and outputs JSON:

**Script** (`node_capacity.sh`):
```bash
#!/usr/bin/env bash
set -euo pipefail
input=$(cat)
node=$(echo "$input" | jq -r '.node // empty')
if [ -n "$node" ]; then
    data=$(kubectl get node "$node" -o json)
    echo "$data" | jq '{nodes: [{name: .metadata.name, ...}]}'
else
    data=$(kubectl get nodes -o json)
    echo "$data" | jq '{nodes: [.items[] | {name: .metadata.name, ...}]}'
fi
```

**Tool definition** (`kubernetes/node_capacity.lua`):
```lua
local base = require("kubernetes.defaults")()
base.entrypoint = "node_capacity.sh"
base.type = "shell"
base.input_adapter = "json"
base.output_adapter = "json"
base.schema = {
    input = {
        node = "string?",
    },
    output = {
        nodes = "array",
    },
}
return tools.define_tool(base)
```

## Simple bash script with plain text output

A script that takes CLI args and prints plain text:

**Script** (`disk_usage.sh`):
```bash
#!/usr/bin/env bash
df -h "$1" 2>/dev/null || df -h
```

**Tool definition** (`system/disk_usage.lua`):
```lua
return tools.define_tool({
    source = "/path/to/actions/system",
    entrypoint = "disk_usage.sh",
    executor = "native",
    type = "shell",
    schema = {
        input = {
            path = "string?",
        },
        output = {
            output = "string",
        },
    },
})
```

Note: Both adapters are omitted — defaults (`args` input, `text` output) are exactly right. The output is accessed as `result.output` in Lua.

## Tool using a git source (Docker executor)

A tool pointing to a remote git repository, running in Docker:

**Tool definition** (`misc/fetch_html.lua`):
```lua
return tools.define_tool({
    source = "git@github.com:user/python-html-downloader",
    input_adapter = "json",
    output_adapter = "json",
    schema = {
        input = {
            url = "string",
        },
        output = {
            content = "string",
        },
    },
})
```

Note: `executor` is omitted because `docker` is the default. No `entrypoint` or `type` — envyr auto-detects them from the repo.

## Defaults factory pattern

When multiple tools share the same source and executor:

**Defaults** (`kubernetes/defaults.lua`):
```lua
return function()
    return {
        source = "/home/user/actions/kubernetes/",
        executor = "native",
        input_adapter = "json",
        output_adapter = "json",
    }
end
```

**Tool using defaults** (`kubernetes/restart_counts.lua`):
```lua
local base = require("kubernetes.defaults")()
base.entrypoint = "restart_counts.py"
base.type = "python"
base.schema = {
    input = {
        namespace = "string?",
        threshold = "number?",
    },
    output = {
        namespace = "string",
        threshold = "number",
        pods = "array",
    },
}
return tools.define_tool(base)
```

Use the defaults factory when 3+ tools share the same source/executor/adapter configuration. For 1-2 tools, inline the config directly.
