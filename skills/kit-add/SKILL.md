---
name: kit-add
description: Add an action script to a kael kit as a tool definition. Use when the user wants to wrap an existing script (Python, bash, etc.) into a kael kit tool with proper adapters, schema, and wiring.
argument-hint: <path-or-git-url> [namespace]
---

# Add a tool to a kael kit

You are adding an action script to a kael kit. The user will provide either a local path to a script or a git URL (e.g. `git@github.com:user/repo.git` or `https://github.com/user/repo`) and optionally a namespace (dotted, e.g. `kubernetes` or `monitoring.prometheus`).

## What you need to determine

Determine if `$ARGUMENTS[0]` is a local path or a git URL. Then read the action script thoroughly to determine:

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
5. **Source**: For local scripts, the directory containing the script. For git repos, the git URL.
6. **Entrypoint**: The script filename (may need auto-detection for git repos)
7. **Environment variables**: What host env vars does the script need?
   - For Python: look for `os.environ["VAR"]`, `os.environ.get("VAR")`, `os.getenv("VAR")`
   - For bash: look for `$VAR`, `${VAR}`, `${VAR:-default}`
   - For node: look for `process.env.VAR`
   - Common ones: `KUBECONFIG`, `AWS_PROFILE`, `AWS_REGION`, `HOME`, `PATH`, `DOCKER_HOST`
   - If any are found, add them to the `env` field as an array of variable names

## Resolving the source

**Local path** (`$ARGUMENTS[0]` is a file path):
- Read the script directly
- Source is the parent directory (absolute path)

**Git URL** (`$ARGUMENTS[0]` starts with `git@`, `https://github.com`, or similar):
1. Clone the repo to a temporary directory: `git clone --depth 1 <url> /tmp/kael-skill-clone-$$`
2. If the user specified a subdir or entrypoint, navigate to it. Otherwise inspect the repo to find the main script.
3. Read and analyze the script from the clone to determine type, adapters, schema, and entrypoint.
4. In the generated tool definition, set `source` to the **git URL** (NOT the local clone path). Envyr handles cloning at runtime.
5. If the repo is a monorepo, set the `subdir` field to the relevant subdirectory.
6. Set `tag` if the user specified a branch, tag, or commit.
7. **Clean up**: After generating the tool definition, remove the temporary clone: `rm -rf /tmp/kael-skill-clone-$$`

## Steps

1. Resolve the source (see above) and read the action script
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
- For local scripts, use an absolute path to the directory as source. For git repos, use the git URL as source.
- Always clean up temporary clones in `/tmp/kael-skill-clone-*` when done
- Check if the tool is already wired in the namespace init.lua before adding a duplicate require line

## Kit path

Resolve the kit path in this order (first match wins):
1. If the user explicitly tells you a kit path, use that
2. Run `echo $KAEL_KIT` — if set and non-empty, use that value
3. Fall back to `~/.kael/kit`

You MUST check the `KAEL_KIT` environment variable before falling back to the default. Run `echo $KAEL_KIT` early in your workflow.

For detailed API reference including all define_tool fields, adapter types, and schema syntax, see [reference.md](reference.md).

For complete examples of real tool definitions, see [examples.md](examples.md).
