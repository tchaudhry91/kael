package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/tchaudhry91/kael/skills"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Initialize kael and install AI skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup()
		},
	}
}

func runSetup() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Detect available tools
	hasClaudeCode, _ := exec.LookPath("claude")
	hasOpenCode, _ := exec.LookPath("opencode")
	hasDocker, _ := exec.LookPath("docker")
	hasPodman, _ := exec.LookPath("podman")

	// Build AI tool options
	aiOptions := []huh.Option[string]{}
	if hasClaudeCode != "" {
		aiOptions = append(aiOptions, huh.NewOption[string]("Claude Code", "claude"))
	}
	if hasOpenCode != "" {
		aiOptions = append(aiOptions, huh.NewOption[string]("OpenCode", "opencode"))
	}
	aiOptions = append(aiOptions, huh.NewOption[string]("None (manual only)", "none"))

	// Build container runtime options
	runtimeOptions := []huh.Option[string]{}
	if hasDocker != "" {
		runtimeOptions = append(runtimeOptions, huh.NewOption[string]("docker", "docker"))
	}
	if hasPodman != "" {
		runtimeOptions = append(runtimeOptions, huh.NewOption[string]("podman", "podman"))
	}
	if len(runtimeOptions) == 0 {
		runtimeOptions = append(runtimeOptions,
			huh.NewOption[string]("docker (not found)", "docker"),
			huh.NewOption[string]("podman (not found)", "podman"),
		)
	}

	// Form values with defaults
	kitPath := filepath.Join(home, ".kael", "kit")
	aiTool := "none"
	if hasClaudeCode != "" {
		aiTool = "claude"
	} else if hasOpenCode != "" {
		aiTool = "opencode"
	}
	containerRuntime := "docker"
	if hasDocker == "" && hasPodman != "" {
		containerRuntime = "podman"
	}
	skillDir := filepath.Join(home, ".claude", "skills")

	// Group 1: AI tool selection
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("AI tool").
				Description("Used by kael kit add to analyze scripts").
				Options(aiOptions...).
				Value(&aiTool),
		).Title("AI Integration"),

		// Group 2: Skill directory (hidden if no AI tool)
		huh.NewGroup(
			huh.NewInput().
				Title("Skill directory").
				Description("Where AI skills are installed").
				Value(&skillDir),
		).Title("AI Integration").
			WithHideFunc(func() bool { return aiTool == "none" }),

		// Group 3: Kit & execution
		huh.NewGroup(
			huh.NewInput().
				Title("Kit path").
				Description("Where tool definitions live").
				Value(&kitPath),

			huh.NewSelect[string]().
				Title("Container runtime").
				Description("Used for sandboxed tool execution").
				Options(runtimeOptions...).
				Value(&containerRuntime),
		).Title("Kit & Execution"),
	)

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("setup cancelled")
			return nil
		}
		return err
	}

	// 1. Bootstrap ~/.kael
	if err := bootstrap(); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// 2. Initialize kit if not exists
	kitInitPath := filepath.Join(kitPath, "init.lua")
	if _, err := os.Stat(kitInitPath); os.IsNotExist(err) {
		if err := os.MkdirAll(kitPath, 0755); err != nil {
			return fmt.Errorf("create kit directory: %w", err)
		}
		if err := os.WriteFile(kitInitPath, []byte("local M = {}\nreturn M\n"), 0644); err != nil {
			return fmt.Errorf("create kit init.lua: %w", err)
		}
		fmt.Printf("kit initialized at %s\n", kitPath)
	}

	// 3. Write config
	aiCommand := ""
	switch aiTool {
	case "claude":
		aiCommand = "claude -p"
	case "opencode":
		aiCommand = "opencode run"
	}

	configPath := filepath.Join(home, ".kael", "config.yaml")
	config := fmt.Sprintf(`# kael configuration
# Values here are overridden by KAEL_ env vars and --flags

kit: %s
container_runtime: %s
`, kitPath, containerRuntime)

	if aiTool != "none" {
		config += fmt.Sprintf(`
ai:
  tool: %s
  command: "%s"
  skill_dir: %s
`, aiTool, aiCommand, skillDir)
	}

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Printf("config written to %s\n", configPath)

	// 4. Install skills if AI tool selected
	if aiTool != "none" {
		if err := installSkills(skillDir); err != nil {
			return fmt.Errorf("install skills: %w", err)
		}
	}

	fmt.Println("\nsetup complete")
	return nil
}

func installSkills(skillDir string) error {
	destDir := filepath.Join(skillDir, "kit-add")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create skill directory: %w", err)
	}

	err := fs.WalkDir(skills.KitAddSkill, "kit-add", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, err := skills.KitAddSkill.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		fileName := filepath.Base(path)
		destPath := filepath.Join(destDir, fileName)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", destPath, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("skill installed to %s\n", destDir)
	return nil
}
