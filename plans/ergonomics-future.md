# Kael Ergonomics Roadmap

Future improvements organized by where they hit in the authoring/usage workflow.

---

## 1. Tool Schema / Autodiscovery

No way to know what a tool expects without reading its source. A `schema` field in
`define_tool` declares inputs and outputs:

```lua
return tools.define_tool({
    source = "...",
    schema = {
        input = { query = "string", namespace = "string?", limit = "number?" },
        output = { pods = "[{name, node, status}]" },
    },
})
```

Unlocks:
- `kael kit describe kubernetes.pods` prints the schema
- LSP uses it for autocomplete on tool call arguments
- Engine validates inputs before dispatching (fail fast with clear message instead of
  a cryptic Python traceback)

## 2. Script Wrappers / Adapters

Most existing scripts aren't JSON-in/JSON-out. Adapter patterns to cover the majority:

- **`stdin_lines`** — input as newline-separated key=value pairs, output captured as
  raw text and wrapped in `{"output": "..."}`
- **`csv`** — output parsed as CSV into an array of objects
- **`args`** — input fields become CLI flags (`--namespace kube-system --limit 10`)
  instead of JSON on stdin
- **`passthrough`** — no transformation, raw bytes in/out, returned as a string in Lua

Configured via a `adapter` field on `define_tool`:

```lua
return tools.define_tool({
    source = "/path/to/legacy-script",
    adapter = "args",  -- converts input table to CLI flags
})
```

Adapter logic lives in the engine (Go side), wrapping the Runner call. Keeps Lua
scripts clean. Massively expands what can be a "tool" without modifying existing scripts.

## 3. Built-in Lua Helpers

Things script authors will keep reaching for that don't need external tools:

- **`file.read(path)`** / **`file.write(path, content)`** — read/write local files
  (reports, collected output)
- **`file.append(path, content)`** — log-style accumulation
- **`csv.parse(str)`** / **`csv.encode(rows)`** — lots of infra tooling speaks CSV
- **`shell.exec(cmd)`** — run a local command directly from Lua without defining a
  full tool (quick glue, not for reusable tools)
- **`env.get(name)`** — read environment variables from Lua
- **`print_table(rows, columns)`** — tabular stdout formatting (like `column -t`)

All implemented as Go functions exposed to the Lua VM, same pattern as `json.encode`.

## 4. `kael kit describe <tool>`

Detailed view of a single tool by dotted path:

```
$ kael kit describe kubernetes.pods
Source:     /home/.../actions/kubernetes/
Entrypoint: pods_on_node.py
Executor:   native
Type:       python
Defaults:
  namespace: "default"
Env:        KUBECONFIG
Schema:
  input:  { node: string, namespace: string? }
  output: { pods: [{ name, namespace, node, status }] }
```

Shows everything from ToolConfig plus schema (if present).

## 5. `kael kit test <tool>`

Invoke a single tool directly from CLI without writing a Lua script:

```
$ kael kit test kubernetes.pods --input '{"node": "worker-1"}'
```

Loads the kit, finds the tool by dotted path, feeds the input, prints the output.
Fastest debug loop for tool development.

## 6. Parallel Tool Execution

Tool calls are currently sequential. A `tools.parallel()` helper runs multiple
independent tools concurrently:

```lua
local results = tools.parallel({
    pods = function() return kit.kubernetes.pods({ node = "w1" }) end,
    capacity = function() return kit.kubernetes.node_capacity({}) end,
})
-- results.pods and results.capacity available
```

Implemented with goroutines under the hood, each getting its own runner invocation.
Big win for scripts that gather data from multiple sources.

## 7. Result Caching

Some tools are expensive (remote APIs, heavy queries). A `cache` field on `define_tool`:

```lua
return tools.define_tool({
    source = "...",
    cache = 300,  -- TTL in seconds
})
```

Engine hashes the input + tool identity, stores results in `~/.kael/cache/`. Same
input within TTL returns cached output without running the tool.

## 8. Output Formatters / Report Helpers

Scripts currently just `print()`. A report helper module:

```lua
report.table(rows, {"name", "node", "status"})  -- tabular to stdout
report.json(data)                                 -- json.pretty to stdout
report.csv(rows, {"name", "node", "status"})     -- CSV to stdout
report.write("output.json", data)                 -- to file
```

Makes scripts more self-documenting about their intended output format.

## 9. Dry Run / Explain Mode

`kael run --dry-run script.lua` — executes the Lua but replaces the Runner with one
that prints what *would* be called instead of actually calling it:

```
[dry-run] kubernetes.pods_on_node
  source:     /home/.../actions/kubernetes/
  entrypoint: pods_on_node.py
  executor:   native
  input:      {"node": "worker-1", "namespace": "default"}
```

Shows source, entrypoint, merged input JSON for each tool invocation. Useful for
debugging kit wiring without running anything.

## 10. Tool Composition in Kit

Define a tool as a composition of other tools, entirely in Lua within the kit:

```lua
-- kit/kubernetes/node_report.lua
return tools.compose(function(params)
    local pods = kit.kubernetes.pods_on_node({ node = params.node })
    local capacity = kit.kubernetes.node_capacity({ node = params.node })
    return { pods = pods, capacity = capacity }
end)
```

Creates a higher-level "tool" that appears in `kit list` and can be called like any
other tool, but is implemented as Lua glue over existing tools rather than an external
script.

## 11. LSP

Language server for Lua scripts that is kit-aware:

- Autocomplete on `kit.` shows namespaces, `kit.kubernetes.` shows tools
- Autocomplete on tool call arguments from schema
- Hover shows tool description, schema, defaults
- Go-to-definition jumps to the kit `.lua` file
- Diagnostics for unknown tools, missing required fields

## 12. REPL

Interactive Lua session with the kit loaded:

```
$ kael repl --kit examples/kit
kael> kit.kubernetes.pods_on_node({ node = "worker-1" })
{ pods = { ... } }
kael> inspect(kit.kubernetes)
{ pods_on_node = <tool>, node_capacity = <tool>, ... }
```

Exploratory workflow for building scripts interactively.

---

## Priority Ranking

Immediate impact, roughly ordered:

1. **Built-in Lua helpers** (file, shell, env) — unblocks real scripts
2. **Script wrappers/adapters** — massively expands what can be a tool
3. **`kit test`** — fastest debug loop
4. **Tool schema** — foundation for describe, LSP, validation
5. **Dry run** — cheap to build, high debug value
6. **`kit describe`** — requires schema to be most useful
7. **Output formatters** — quality of life for script authors
8. **Result caching** — performance win for heavy workflows
9. **Parallel execution** — performance win for multi-source scripts
10. **Tool composition** — power feature for kit authors
11. **REPL** — exploratory workflow
12. **LSP** — best DX but highest implementation cost
