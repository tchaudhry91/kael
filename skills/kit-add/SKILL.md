---
name: kit-add
description: Analyze a script and output a JSON tool definition for kael.
argument-hint: <local-path>
---

# Analyze a script for kael kit

You are analyzing a script to produce a JSON tool definition. The user provides a local path to a script or directory.

## What to do

1. Read the script at the given path
2. Analyze it to determine the fields below
3. Output ONLY a valid JSON object — no explanation, no markdown, no code fences

## Fields to determine

1. **type**: File extension → `.py` = `"python"`, `.sh` = `"shell"`, `.js`/`.ts` = `"node"`
2. **entrypoint**: The script filename (e.g. `"download.py"`)
3. **input_adapter**: How the script receives input
   - `json.load(sys.stdin)`, `$(cat)` with `jq`, `process.stdin` → `"json"`
   - `argparse`, `sys.argv`, `getopts`, `$1/$2` → omit (args is the default)
4. **output_adapter**: How the script produces output
   - `json.dump()`, pipes through `jq` → `"json"`
   - `print()`, `echo` for plain text → omit (text is the default)
   - One item per line → `"lines"`
5. **schema**: Input and output field types
   - For input: read argument names, JSON keys accessed from parsed input
   - For output: read what gets written to stdout
   - Use `"string?"`, `"number?"` for optional fields
6. **deps**: Third-party packages the script needs
   - Python: look for `import` statements, match against stdlib. Add non-stdlib packages.
   - Node: look for `require()` / `import` of non-builtin modules
   - Shell: look for commands used (`curl`, `jq`, `kubectl`). Add apk package names.
   - Omit if empty or if a `requirements.txt`/`package.json` already covers them.
7. **env**: Environment variables the script reads from the host
   - Python: `os.environ["VAR"]`, `os.environ.get("VAR")`, `os.getenv("VAR")`
   - Bash: `$VAR`, `${VAR}`, `${VAR:-default}`
   - Node: `process.env.VAR`
   - Omit if none found.

## Output format

Output ONLY this JSON object, nothing else:

{"type":"python","entrypoint":"script.py","input_adapter":"json","output_adapter":"json","schema":{"input":{"field":"type"},"output":{"field":"type"}},"deps":["requests"],"env":["KUBECONFIG"]}

Rules:
- Omit `input_adapter` if it would be `"args"`
- Omit `output_adapter` if it would be `"text"`
- Omit `deps` if empty
- Omit `env` if empty
- Always include `type`, `entrypoint`, and `schema`
- Output raw JSON only — no markdown, no code fences, no explanation

For reference on kael's define_tool API and adapter details, see [reference.md](reference.md).
