# Kael

Kael is a scriptable infrastructure analysis engine. It lets you write Lua scripts that orchestrate actions — data fetching, analysis, transformations — with implementations that live anywhere (git repos, local paths).

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│  User Script (Lua)                                          │
│                                                             │
│  local data = actions.prometheus.query({                    │
│      query = "up",                                          │
│      range = "7d",                                          │
│  })                                                         │
│                                                             │
│  local result = actions.analysis.quiet_periods({            │
│      data = data,                                           │
│      threshold = 0.2,                                       │
│  })                                                         │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  Kael Engine (Go)                                           │
│                                                             │
│  - Embeds Lua VM                                            │
│  - Provides define_action() global                          │
│  - Loads actions repo                                       │
│  - Executes actions via envyr                               │
│  - Handles JSON conversion                                  │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  Envyr                                                      │
│                                                             │
│  - Runs action implementations                              │
│  - Docker executor (sandboxed) or native executor           │
│  - Handles git cloning, dependency installation             │
│  - JSON stdin/stdout contract                               │
└─────────────────────────────────────────────────────────────┘
```

## Core Concepts

### Actions

An action is a unit of work — fetch data from Prometheus, analyze timeseries, query an API. Actions are implemented as Python scripts (or any language) that:

- Read JSON from stdin
- Do work
- Write JSON to stdout

Action implementations live anywhere — public git repos, private repos, local paths. They're generic and reusable.

### Actions Repo (Lua Metadata)

A Lua repo that defines which actions are available and how to call them. This is what you share with your team. It:

- References action implementations by git URL or path
- Defines company-specific defaults
- Provides the interface your scripts use

### Scripts

Lua scripts that use actions to do useful work. Orchestration, looping, conditionals, combining results — that's what Lua handles.

## Actions Repo Structure

```
my-actions/
├── init.lua
├── prometheus/
│   ├── init.lua
│   └── query.lua
├── analysis/
│   ├── init.lua
│   └── quiet_periods.lua
├── kubernetes/
│   ├── init.lua
│   └── nodes.lua
└── internal/
    ├── init.lua
    └── tenant_api.lua
```

### init.lua (entry point)

```lua
local M = {}

M.prometheus = require("prometheus")
M.analysis = require("analysis")
M.kubernetes = require("kubernetes")
M.internal = require("internal")

return M
```

### Namespace init (e.g., prometheus/init.lua)

```lua
local M = {}

M.query = require("prometheus.query")

return M
```

### Action Definition (e.g., prometheus/query.lua)

```lua
return define_action({
    source = "git@github.com:someone/prometheus-tools.git",
    entrypoint = "query.py",
    executor = "docker",
    defaults = {
        endpoint = "http://prometheus.internal:9090",
        step = "5m",
    },
})
```

## define_action Config

```lua
define_action({
    -- Required: where the implementation lives
    source = "git@github.com:someone/repo.git",
    
    -- Optional: specific file if repo has multiple
    entrypoint = "query.py",
    
    -- Optional: subdirectory within repo
    subdir = "scripts",
    
    -- Required: execution mode
    executor = "docker",  -- or "native"
    
    -- Optional: default values merged with user params
    defaults = {
        endpoint = "http://localhost:9090",
    },
    
    -- Optional: environment variables to pass through
    env = { "AWS_PROFILE", "KUBECONFIG" },
    
    -- Optional: max execution time in seconds
    timeout = 60,
})
```

## User Scripts

```lua
-- Fetch data
local data = actions.prometheus.query({
    query = 'sum(rate(http_requests_total[5m])) by (tenant)',
    range = "90d",
})

-- Process results
local filtered = {}
for i, row in ipairs(data) do
    if row.value > 100 then
        table.insert(filtered, row)
    end
end

-- Pass to another action
local result = actions.analysis.quiet_periods({
    data = filtered,
    threshold = 0.2,
})

-- Output
for i, window in ipairs(result.windows) do
    print(window.day, window.start_hour, window.end_hour)
end
```

### Looping Over Tenants

```lua
local tenants = actions.internal.list_tenants({})

for i, tenant in ipairs(tenants) do
    local cost = actions.aws.cost_explorer({
        tenant = tenant.name,
        days = 30,
    })
    
    local quiet = actions.analysis.quiet_periods({
        data = cost.data,
        threshold = 0.2,
    })
    
    print(tenant.name, cost.total, #quiet.windows)
end
```

### Error Handling

Actions raise Lua errors on failure. Simple scripts crash with a message:

```
$ kael run script.lua
Error: prometheus.query failed: connection refused
```

Scripts that need to handle errors use pcall:

```lua
local ok, data = pcall(actions.prometheus.query, {
    query = "up",
    range = "7d",
})

if not ok then
    print("Query failed:", data)
    data = {}  -- fallback
end
```

Actions may return error objects instead of crashing:

```lua
local result = actions.internal.tenant_lookup({ id = "xyz" })

if result.error then
    print("Not found:", result.error)
else
    print("Found:", result.name)
end
```

## Engine Responsibilities

### Startup

1. Create Lua VM
2. Register `define_action` as global
3. Load actions repo: `actions = require("init")`
4. Run user script

### Action Execution

When an action is called:

1. Merge defaults with user params (user wins)
2. Build envyr command from metadata
3. Serialize params to JSON
4. Run envyr with JSON on stdin
5. Parse JSON output
6. Convert to Lua table and return

### Envyr Invocation

```bash
envyr run \
  --executor docker \
  --autogen \
  --timeout 60 \
  --env-map AWS_PROFILE \
  git@github.com:someone/repo.git
```

Stdin receives the merged params as JSON. Stdout is captured and parsed as JSON.

## Action Implementation Contract

Actions are scripts that:

1. Read JSON from stdin
2. Do work
3. Write JSON to stdout
4. Exit 0 on success, non-zero on failure

Example Python action:

```python
import json
import sys

def main():
    params = json.load(sys.stdin)
    
    # Do work...
    result = {"windows": [...], "metadata": {...}}
    
    json.dump(result, sys.stdout)

if __name__ == "__main__":
    main()
```

Actions don't know about Kael. They're generic scripts that follow the JSON stdin/stdout contract.

## CLI

```bash
# Run a script
kael --actions ./my-actions run script.lua

# REPL (future)
kael --actions ./my-actions repl
```

## Future Considerations

### LSP Support

Action definitions can include EmmyLua annotations for editor autocomplete:

```lua
---@class PrometheusQueryParams
---@field query string PromQL query
---@field range string Time range
---@field endpoint? string Prometheus URL

---@param params PrometheusQueryParams
---@return { timestamp: string, value: number }[]
return define_action({...})
```

### Large Data

For large datasets, actions can write to temp files and return file paths:

```json
{"_file": "/tmp/kael-data-xyz.json"}
```

Engine reads the file, converts to Lua table, cleans up.

### Config File

```yaml
# ~/.config/kael/config.yaml
actions_path: ~/my-actions
```

Avoids `--actions` flag on every invocation.
