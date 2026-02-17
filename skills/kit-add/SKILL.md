---
name: kit-add
description: Analyze a script and output a JSON tool definition for kael.
argument-hint: <local-path>
---

# Analyze a script for kael kit

You are analyzing a script to produce a JSON tool definition. The user provides a local path to a script or directory.

## What to do

1. Read the script at the given path
2. If the prompt includes "Additional instructions", those are directives from the user — follow them. They take priority over your own analysis when they conflict.
3. Analyze the script to determine the fields below
4. Output ONLY a valid JSON object — no explanation, no markdown, no code fences

## Fields to determine

1. **type**: File extension → `.py` = `"python"`, `.sh` = `"shell"`, `.js`/`.ts` = `"node"`
2. **entrypoint**: The script filename (e.g. `"download.py"`)
3. **executor**: Execution environment — `"native"` (default, omit) or `"docker"` (containerized). Omit if native.
4. **input_adapter**: How the script receives input
   - `json.load(sys.stdin)`, `$(cat)` with `jq`, `process.stdin` → `"json"`
   - `argparse`, `getopts`, named flags (`--key value`) → omit (args is the default)
   - `$1`, `$2`, `sys.argv[1]`, positional parameters → `"positional_args"`
5. **args_order**: Required when `input_adapter` is `"positional_args"`. An ordered list of field names matching each positional parameter.
   - `$1` = first entry, `$2` = second entry, etc.
   - Example: script uses `$1` as tenant name and `$2` as region → `["tenant_name", "region"]`
6. **output_adapter**: How the script produces output
   - `json.dump()`, pipes through `jq` → `"json"`
   - `print()`, `echo` for plain text → omit (text is the default)
   - One item per line → `"lines"`
7. **schema**: Input field types ONLY — do NOT include output schema
   - Read argument names, JSON keys accessed from parsed input
   - Every value MUST be a simple type string: `"string"`, `"number"`, `"boolean"`, `"object"`, `"string[]"`, `"object[]"`
   - Append `?` for optional: `"string?"`, `"number?"`
   - NEVER use nested objects or arrays as values — always use flat type strings
   - Example: `{"input":{"urls":"string[]","timeout":"number?"}}`
8. **deps**: Third-party packages the script needs
   - Python: look for `import` statements, match against stdlib. Add non-stdlib packages.
   - Node: look for `require()` / `import` of non-builtin modules
   - Shell: look for commands used (`curl`, `jq`, `kubectl`). Add apk package names.
   - Omit if empty or if a `requirements.txt`/`package.json` already covers them.
9. **env**: Environment variables the script reads from the host
   - Python: `os.environ["VAR"]`, `os.environ.get("VAR")`, `os.getenv("VAR")`
   - Bash: `$VAR`, `${VAR}`, `${VAR:-default}`
   - Node: `process.env.VAR`
   - Omit if none found.

## Naming convention

- Always use underscores (`_`) in field names, never hyphens (`-`).
  - Good: `tenant_name`, `from_time`, `resource_group`
  - Bad: `tenant-name`, `from-time`, `resource-group`

## Output format

Output ONLY this JSON object, nothing else:

{"type":"shell","entrypoint":"script.sh","input_adapter":"positional_args","args_order":["tenant_name"],"output_adapter":"json","schema":{"input":{"tenant_name":"string"}},"deps":["jq"],"env":["KUBECONFIG"]}

Rules:
- Omit `executor` if it would be `"native"` (native is the default)
- Omit `input_adapter` if it would be `"args"`
- Omit `output_adapter` if it would be `"text"`
- Omit `deps` if empty — `deps` is a TOP-LEVEL field, NOT inside `schema`
- Omit `env` if empty — `env` is a TOP-LEVEL field, NOT inside `schema`
- Include `args_order` only when `input_adapter` is `"positional_args"`
- Always include `type`, `entrypoint`, and `schema`
- Schema contains ONLY `input` — NEVER include `output` schema
- All schema values MUST be flat type strings (`"string"`, `"number?"`, `"object[]"`, etc.) — never nested objects or arrays
- Output raw JSON only — no markdown, no code fences, no explanation

## Additional instructions from user

The prompt may include an "Additional instructions" section after the script path. These are user-provided directives that you MUST follow. They take priority over your own analysis when they conflict. For example, if the user says "use docker executor", you MUST set `"executor":"docker"` in the output. Apply them directly to the JSON output.

For reference on kael's define_tool API and adapter details, see [reference.md](reference.md).
