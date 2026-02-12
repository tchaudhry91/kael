---
name: kit-add
description: Add an action script to a kael kit as a tool definition. Use when the user wants to wrap an existing script (Python, bash, etc.) into a kael kit tool with proper adapters, schema, and wiring.
argument-hint: <path-to-action-script> [namespace]
---

# Add a tool to a kael kit

You are adding an action script to a kael kit. The user will provide a path to a script and optionally a namespace (dotted, e.g. `kubernetes` or `monitoring.prometheus`).

## What you need to determine

Read the action script at `$ARGUMENTS[0]` thoroughly. From the code, determine:

1. **Type**: File extension tells you — `.py` → `python`, `.sh` → `shell`, `.js`/`.ts` → `node`
2. **Input adapter**: How does the script receive input?
   - Uses `argparse`, `sys.argv`, `getopts`, or `$1/$2` positional args → `args` (this is the default, omit the field)
   - Uses `json.load(sys.stdin)` or `$(cat)` with `jq` → `json`
3. **Output adapter**: How does the script produce output?
   - Uses `json.dump()` or pipes through `jq` to produce JSON → `json`
   - Uses `print()` or `echo` for plain text → `text` (this is the default, omit the field)
   - Outputs one item per line → `lines`
4. **Schema**: What input fields does the script read? What does it output?
   - For argparse: read the `add_argument` calls
   - For JSON stdin: read the keys accessed from the parsed input
   - For bash: read jq field accesses or positional args
   - For output: read what gets written to stdout
5. **Source**: The directory containing the script (not the script file itself)
6. **Entrypoint**: The script filename

## Steps

1. Read the action script and determine all the above
2. Determine the kit path — use `$ARGUMENTS[1]` as the namespace if provided, otherwise ask the user
3. Verify the namespace exists in the kit (check for `<kit>/<namespace>/init.lua`). If it doesn't exist, tell the user to run `kael kit init <namespace>` first. Do NOT create namespaces yourself.
4. Generate the `.lua` tool definition file (see reference.md for the API)
5. Wire the tool into the namespace's `init.lua` by adding `M.<toolname> = require("<namespace>.<toolname>")` before `return M`
6. Run `kael kit validate` to verify the kit loads correctly

## Important rules

- NEVER create namespace directories or top-level init.lua — that's `kael kit init`'s job
- The tool name is derived from the script filename without extension: `pods_on_node.py` → `pods_on_node`
- Only set `input_adapter` if it's NOT `args` (args is the default)
- Only set `output_adapter` if it's NOT `text` (text is the default)
- Only set `executor` if it's NOT `docker` (docker is the default)
- Include schema whenever you can infer it from the code. Use `"string?"` shorthand for optional fields.
- For the source field, use an absolute path to the directory containing the script
- Check if the tool is already wired in the namespace init.lua before adding a duplicate require line

## Kit path

The kit path defaults to `~/.kael/kit`. Check if the user has a `--kit` flag or `KAEL_KIT` environment variable set. If working in a project with an `examples/kit` directory, that may be the kit path instead.

For detailed API reference including all define_tool fields, adapter types, and schema syntax, see [reference.md](reference.md).

For complete examples of real tool definitions, see [examples.md](examples.md).
