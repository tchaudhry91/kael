package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tchaudhry91/kael/runtime"
)

// toolAnalysis is the JSON output from the AI skill.
type toolAnalysis struct {
	Type          string                       `json:"type"`
	Entrypoint    string                       `json:"entrypoint"`
	Executor      string                       `json:"executor,omitempty"`
	SubDir        string                       `json:"subdir,omitempty"`
	Tag           string                       `json:"tag,omitempty"`
	InputAdapter  string                       `json:"input_adapter,omitempty"`
	OutputAdapter string                       `json:"output_adapter,omitempty"`
	ArgsOrder     []string                     `json:"args_order,omitempty"`
	Schema        *toolAnalysisSchema          `json:"schema,omitempty"`
	Deps          []string                     `json:"deps,omitempty"`
	Env           []string                     `json:"env,omitempty"`
}

type toolAnalysisSchema struct {
	Input map[string]interface{} `json:"input,omitempty"`
}

// kitAddOptions holds all flags for the kit add command.
type kitAddOptions struct {
	manual      bool
	force       bool
	extraPrompt string
	// Override flags — applied post-analysis
	executor   string
	entrypoint string
	subdir     string
	tag        string
	toolType   string
}

func newKitAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <source> <namespace>",
		Short: "Add a tool to the kit from a script or git repo",
		Long: `Add a tool to the kit by analyzing a script. Source can be a local path
to a script file or a git URL. The namespace must already exist (use kit init).

For git monorepos, use // to specify a path within the repo:
  kael kit add git@github.com:org/repo.git//scripts/check.py myns

By default, uses the configured AI tool to analyze the script. Use --manual
to skip AI and generate a skeleton definition.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			manual, _ := cmd.Flags().GetBool("manual")
			force, _ := cmd.Flags().GetBool("force")
			extraPrompt, _ := cmd.Flags().GetString("prompt")
			executor, _ := cmd.Flags().GetString("executor")
			entrypoint, _ := cmd.Flags().GetString("entrypoint")
			subdir, _ := cmd.Flags().GetString("subdir")
			tag, _ := cmd.Flags().GetString("tag")
			toolType, _ := cmd.Flags().GetString("type")
			opts := kitAddOptions{
				manual:      manual,
				force:       force,
				extraPrompt: extraPrompt,
				executor:    executor,
				entrypoint:  entrypoint,
				subdir:      subdir,
				tag:         tag,
				toolType:    toolType,
			}
			return kitAdd(args[0], args[1], opts)
		},
	}

	cmd.Flags().Bool("manual", false, "skip AI analysis, generate skeleton definition")
	cmd.Flags().Bool("force", false, "overwrite existing tool definition")
	cmd.Flags().String("prompt", "", "additional instructions for the AI analysis")
	cmd.Flags().String("executor", "", "override executor (native, docker)")
	cmd.Flags().String("entrypoint", "", "override entrypoint script filename")
	cmd.Flags().String("subdir", "", "override subdirectory within source")
	cmd.Flags().String("tag", "", "git tag, branch, or commit hash")
	cmd.Flags().String("type", "", "override script type (python, shell, node)")

	return cmd
}

func kitAdd(source, namespace string, opts kitAddOptions) error {
	kitPath := viper.GetString("kit")

	// 1. Validate namespace exists
	nsParts := strings.Split(namespace, ".")
	nsDir := filepath.Join(append([]string{kitPath}, nsParts...)...)
	nsInit := filepath.Join(nsDir, "init.lua")
	if _, err := os.Stat(nsInit); os.IsNotExist(err) {
		return fmt.Errorf("namespace %q does not exist — run: kael kit init %s", namespace, namespace)
	}

	// 2. Parse // separator for git monorepo paths
	gitSource, inRepoPath := parseSourcePath(source)

	// CLI flags take precedence over // path parsing
	subdir := opts.subdir
	entrypoint := opts.entrypoint
	if inRepoPath != "" && subdir == "" && entrypoint == "" {
		// Check if the in-repo path points to a file or directory
		dir, file := filepath.Split(inRepoPath)
		ext := filepath.Ext(file)
		if ext == ".py" || ext == ".sh" || ext == ".js" || ext == ".ts" {
			// Path ends in a script — split into subdir + entrypoint
			subdir = strings.TrimSuffix(dir, "/")
			entrypoint = file
		} else {
			// Treat the whole path as a subdir
			subdir = inRepoPath
		}
	}

	// 3. Resolve source to local path
	localPath, originalSource, discoveredEntrypoint, err := resolveAddSource(gitSource, subdir, opts.tag)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}

	// Entrypoint priority: CLI flag > // path > discovered
	if entrypoint == "" {
		entrypoint = discoveredEntrypoint
	}

	// 4. Find the entrypoint script (if not already known)
	if entrypoint == "" {
		entrypoint, err = findEntrypoint(localPath)
		if err != nil {
			return fmt.Errorf("find entrypoint: %w", err)
		}
	}

	scriptPath := filepath.Join(localPath, entrypoint)

	// 5. Get tool analysis (AI or manual)
	var analysis toolAnalysis
	if opts.manual {
		analysis = skeletonAnalysis(entrypoint)
	} else {
		a, err := aiAnalysis(scriptPath, opts.extraPrompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "AI analysis failed: %v\nfalling back to skeleton\n", err)
			analysis = skeletonAnalysis(entrypoint)
		} else {
			analysis = a
		}
	}

	// Use AI-detected entrypoint if available, otherwise keep discovered one
	if analysis.Entrypoint != "" {
		entrypoint = analysis.Entrypoint
	}

	// 6. Apply CLI overrides post-analysis
	if opts.executor != "" {
		analysis.Executor = opts.executor
	}
	if opts.toolType != "" {
		analysis.Type = opts.toolType
	}
	if entrypoint != "" {
		analysis.Entrypoint = entrypoint
	}
	if subdir != "" {
		analysis.SubDir = subdir
	}
	if opts.tag != "" {
		analysis.Tag = opts.tag
	}

	// 7. Generate Lua definition
	toolName := strings.TrimSuffix(entrypoint, filepath.Ext(entrypoint))
	luaContent := generateLua(originalSource, analysis)

	// 8. Write Lua file
	luaPath := filepath.Join(nsDir, toolName+".lua")
	if _, err := os.Stat(luaPath); err == nil && !opts.force {
		return fmt.Errorf("tool %q already exists at %s (use --force to overwrite)", toolName, luaPath)
	}
	if err := os.WriteFile(luaPath, []byte(luaContent), 0644); err != nil {
		return fmt.Errorf("write tool definition: %w", err)
	}
	fmt.Printf("wrote %s\n", luaPath)

	// 9. Wire into namespace init.lua
	requirePath := namespace + "." + toolName
	if err := wireNamespace(nsInit, toolName, requirePath); err != nil {
		return fmt.Errorf("wire namespace: %w", err)
	}
	fmt.Printf("wired %s into %s\n", toolName, nsInit)

	// 10. Validate
	if err := kitValidate(kitPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: kit validation failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "you may need to edit %s\n", luaPath)
	}

	return nil
}

// parseSourcePath splits a source on "//" to separate the base source from
// an in-repo path (Terraform-style). For example:
//
//	git@github.com:org/repo.git//scripts/check.py → (git@..., scripts/check.py)
//	/local/path → (/local/path, "")
func parseSourcePath(source string) (base, inRepoPath string) {
	idx := strings.Index(source, "//")
	if idx < 0 {
		return source, ""
	}
	// For git URLs, "//" can appear in https:// — skip the protocol
	protocolEnd := 0
	if strings.HasPrefix(source, "https://") {
		protocolEnd = len("https://")
	} else if strings.HasPrefix(source, "http://") {
		protocolEnd = len("http://")
	}

	idx = strings.Index(source[protocolEnd:], "//")
	if idx < 0 {
		return source, ""
	}
	idx += protocolEnd
	return source[:idx], strings.TrimPrefix(source[idx+2:], "/")
}

// resolveAddSource resolves a source to (localPath, originalSource, entrypoint).
// For git URLs, it clones/caches and returns the cache path + git URL.
// For local paths, it returns the absolute path for both.
// If source is a file (not a directory), entrypoint is set to the filename.
func resolveAddSource(source, subdir, tag string) (string, string, string, error) {
	if runtime.IsGitURL(source) {
		localPath, err := runtime.ResolveSource(source, tag, subdir, false)
		if err != nil {
			return "", "", "", err
		}
		return localPath, source, "", nil
	}

	// Local path — resolve to absolute
	absPath, err := filepath.Abs(source)
	if err != nil {
		return "", "", "", err
	}

	if subdir != "" {
		absPath = filepath.Join(absPath, subdir)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", "", "", err
	}
	if !info.IsDir() {
		return filepath.Dir(absPath), filepath.Dir(absPath), filepath.Base(absPath), nil
	}
	return absPath, absPath, "", nil
}

// findEntrypoint looks for a single script file in the directory.
// If multiple exist, returns an error asking the user to specify.
func findEntrypoint(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var scripts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		switch ext {
		case ".py", ".sh", ".js", ".ts":
			scripts = append(scripts, e.Name())
		}
	}

	if len(scripts) == 0 {
		return "", fmt.Errorf("no script files (.py, .sh, .js, .ts) found in %s", dir)
	}
	if len(scripts) == 1 {
		return scripts[0], nil
	}

	// Multiple scripts — pick common patterns
	for _, name := range scripts {
		lower := strings.ToLower(name)
		if lower == "main.py" || lower == "main.sh" || lower == "main.js" ||
			lower == "index.js" || lower == "app.py" {
			return name, nil
		}
	}

	return "", fmt.Errorf("multiple scripts found (%s) — pass the specific file path", strings.Join(scripts, ", "))
}

func skeletonAnalysis(entrypoint string) toolAnalysis {
	t := "python"
	switch filepath.Ext(entrypoint) {
	case ".sh":
		t = "shell"
	case ".js", ".ts":
		t = "node"
	}
	return toolAnalysis{
		Type:       t,
		Entrypoint: entrypoint,
		Executor:   "native",
	}
}

func aiAnalysis(scriptPath string, extraPrompt string) (toolAnalysis, error) {
	aiCommand := viper.GetString("ai.command")
	if aiCommand == "" {
		return toolAnalysis{}, fmt.Errorf("no AI tool configured — run kael setup or use --manual")
	}

	// Build the prompt
	prompt := fmt.Sprintf("/kit-add %s", scriptPath)
	if extraPrompt != "" {
		prompt += "\n\nAdditional instructions: " + extraPrompt
	}

	// Split command (e.g. "claude -p" → ["claude", "-p"])
	parts := strings.Fields(aiCommand)
	if len(parts) == 0 {
		return toolAnalysis{}, fmt.Errorf("invalid ai.command: %q", aiCommand)
	}

	args := make([]string, 0, len(parts)-1+1)
	args = append(args, parts[1:]...)
	args = append(args, prompt)
	cmd := exec.Command(parts[0], args...)
	cmd.Dir = filepath.Dir(scriptPath)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return toolAnalysis{}, fmt.Errorf("ai command failed: %w", err)
	}

	// Extract JSON from output — find the first { and last }
	raw := strings.TrimSpace(string(out))
	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return toolAnalysis{}, fmt.Errorf("no JSON found in AI output:\n%s", raw)
	}

	var analysis toolAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		return toolAnalysis{}, fmt.Errorf("parse AI output: %w\nraw: %s", err, jsonStr)
	}

	return analysis, nil
}

// extractJSON finds the first JSON object in a string by matching braces,
// correctly skipping braces inside quoted strings.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			switch ch {
			case '\\':
				i++ // skip escaped character
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// flattenType converts a schema value to a flat type string.
// If the value is already a string, it's returned as-is.
// Arrays become "object[]", maps become "object".
func flattenType(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	case []interface{}:
		return "object[]"
	case map[string]interface{}:
		return "object"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// generateLua produces the Lua tool definition from the analysis.
func generateLua(source string, a toolAnalysis) string {
	var b strings.Builder
	b.WriteString("return tools.define_tool({\n")
	b.WriteString(fmt.Sprintf("    source = %q,\n", source))
	b.WriteString(fmt.Sprintf("    entrypoint = %q,\n", a.Entrypoint))
	b.WriteString(fmt.Sprintf("    type = %q,\n", a.Type))

	executor := a.Executor
	if executor == "" {
		executor = "native"
	}
	b.WriteString(fmt.Sprintf("    executor = %q,\n", executor))

	if a.SubDir != "" {
		b.WriteString(fmt.Sprintf("    subdir = %q,\n", a.SubDir))
	}
	if a.Tag != "" {
		b.WriteString(fmt.Sprintf("    tag = %q,\n", a.Tag))
	}

	if len(a.Deps) > 0 {
		quoted := make([]string, len(a.Deps))
		for i, d := range a.Deps {
			quoted[i] = fmt.Sprintf("%q", d)
		}
		b.WriteString(fmt.Sprintf("    deps = {%s},\n", strings.Join(quoted, ", ")))
	}

	if a.InputAdapter != "" && a.InputAdapter != "args" {
		b.WriteString(fmt.Sprintf("    input_adapter = %q,\n", a.InputAdapter))
	}
	if len(a.ArgsOrder) > 0 {
		quoted := make([]string, len(a.ArgsOrder))
		for i, f := range a.ArgsOrder {
			quoted[i] = fmt.Sprintf("%q", f)
		}
		b.WriteString(fmt.Sprintf("    args_order = {%s},\n", strings.Join(quoted, ", ")))
	}
	if a.OutputAdapter != "" && a.OutputAdapter != "text" {
		b.WriteString(fmt.Sprintf("    output_adapter = %q,\n", a.OutputAdapter))
	}

	if len(a.Env) > 0 {
		quoted := make([]string, len(a.Env))
		for i, e := range a.Env {
			quoted[i] = fmt.Sprintf("%q", e)
		}
		b.WriteString(fmt.Sprintf("    env = {%s},\n", strings.Join(quoted, ", ")))
	}

	if a.Schema != nil && len(a.Schema.Input) > 0 {
		b.WriteString("    schema = {\n")
		b.WriteString("        input = {\n")
		for k, v := range a.Schema.Input {
			b.WriteString(fmt.Sprintf("            %s = %q,\n", k, flattenType(v)))
		}
		b.WriteString("        },\n")
		b.WriteString("    },\n")
	}

	b.WriteString("})\n")
	return b.String()
}

