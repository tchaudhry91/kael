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
	InputAdapter  string                       `json:"input_adapter,omitempty"`
	OutputAdapter string                       `json:"output_adapter,omitempty"`
	ArgsOrder     []string                     `json:"args_order,omitempty"`
	Schema        *toolAnalysisSchema          `json:"schema,omitempty"`
	Deps          []string                     `json:"deps,omitempty"`
	Env           []string                     `json:"env,omitempty"`
}

type toolAnalysisSchema struct {
	Input  map[string]interface{} `json:"input,omitempty"`
	Output map[string]interface{} `json:"output,omitempty"`
}

func newKitAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <source> <namespace>",
		Short: "Add a tool to the kit from a script or git repo",
		Long: `Add a tool to the kit by analyzing a script. Source can be a local path
to a script file or a git URL. The namespace must already exist (use kit init).

By default, uses the configured AI tool to analyze the script. Use --manual
to skip AI and generate a skeleton definition.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			manual, _ := cmd.Flags().GetBool("manual")
			extraPrompt, _ := cmd.Flags().GetString("prompt")
			return kitAdd(args[0], args[1], manual, extraPrompt)
		},
	}

	cmd.Flags().Bool("manual", false, "skip AI analysis, generate skeleton definition")
	cmd.Flags().String("prompt", "", "additional instructions for the AI analysis")

	return cmd
}

func kitAdd(source, namespace string, manual bool, extraPrompt string) error {
	kitPath := viper.GetString("kit")

	// 1. Validate namespace exists
	nsParts := strings.Split(namespace, ".")
	nsDir := filepath.Join(append([]string{kitPath}, nsParts...)...)
	nsInit := filepath.Join(nsDir, "init.lua")
	if _, err := os.Stat(nsInit); os.IsNotExist(err) {
		return fmt.Errorf("namespace %q does not exist — run: kael kit init %s", namespace, namespace)
	}

	// 2. Resolve source to local path
	localPath, originalSource, entrypoint, err := resolveAddSource(source)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}

	// 3. Find the entrypoint script (if not already known from source)
	if entrypoint == "" {
		entrypoint, err = findEntrypoint(localPath)
		if err != nil {
			return fmt.Errorf("find entrypoint: %w", err)
		}
	}

	scriptPath := filepath.Join(localPath, entrypoint)

	// 4. Get tool analysis (AI or manual)
	var analysis toolAnalysis
	if manual {
		analysis = skeletonAnalysis(entrypoint)
	} else {
		a, err := aiAnalysis(scriptPath, extraPrompt)
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

	// 5. Generate Lua definition
	toolName := strings.TrimSuffix(entrypoint, filepath.Ext(entrypoint))
	luaContent := generateLua(originalSource, analysis)

	// 6. Write Lua file
	luaPath := filepath.Join(nsDir, toolName+".lua")
	if _, err := os.Stat(luaPath); err == nil {
		return fmt.Errorf("tool %q already exists at %s", toolName, luaPath)
	}
	if err := os.WriteFile(luaPath, []byte(luaContent), 0644); err != nil {
		return fmt.Errorf("write tool definition: %w", err)
	}
	fmt.Printf("wrote %s\n", luaPath)

	// 7. Wire into namespace init.lua
	requirePath := namespace + "." + toolName
	if err := wireNamespace(nsInit, toolName, requirePath); err != nil {
		return fmt.Errorf("wire namespace: %w", err)
	}
	fmt.Printf("wired %s into %s\n", toolName, nsInit)

	// 8. Validate
	if err := kitValidate(kitPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: kit validation failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "you may need to edit %s\n", luaPath)
	}

	return nil
}

// resolveAddSource resolves a source to (localPath, originalSource, entrypoint).
// For git URLs, it clones/caches and returns the cache path + git URL.
// For local paths, it returns the absolute path for both.
// If source is a file (not a directory), entrypoint is set to the filename.
func resolveAddSource(source string) (string, string, string, error) {
	if runtime.IsGitURL(source) {
		localPath, err := runtime.ResolveSource(source, "", "", false)
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
		Schema: &toolAnalysisSchema{
			Input:  map[string]interface{}{"param": "string"},
			Output: map[string]interface{}{"output": "string"},
		},
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

	args := append(parts[1:], prompt)
	cmd := exec.Command(parts[0], args...)
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

// extractJSON finds the first JSON object in a string by matching braces.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
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

	if a.Executor != "" && a.Executor != "docker" {
		b.WriteString(fmt.Sprintf("    executor = %q,\n", a.Executor))
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

	if a.Schema != nil && (len(a.Schema.Input) > 0 || len(a.Schema.Output) > 0) {
		b.WriteString("    schema = {\n")
		if len(a.Schema.Input) > 0 {
			b.WriteString("        input = {\n")
			for k, v := range a.Schema.Input {
				b.WriteString(fmt.Sprintf("            %s = %q,\n", k, flattenType(v)))
			}
			b.WriteString("        },\n")
		}
		if len(a.Schema.Output) > 0 {
			b.WriteString("        output = {\n")
			for k, v := range a.Schema.Output {
				b.WriteString(fmt.Sprintf("            %s = %q,\n", k, flattenType(v)))
			}
			b.WriteString("        },\n")
		}
		b.WriteString("    },\n")
	}

	b.WriteString("})\n")
	return b.String()
}
