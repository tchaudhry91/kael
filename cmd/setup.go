package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

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

	// 1. Bootstrap ~/.kael
	if err := bootstrap(); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	fmt.Println("~/.kael directory ready")

	// 2. Initialize default kit if not exists
	kitPath := filepath.Join(home, ".kael", "kit")
	kitInit := filepath.Join(kitPath, "init.lua")
	if _, err := os.Stat(kitInit); os.IsNotExist(err) {
		if err := os.MkdirAll(kitPath, 0755); err != nil {
			return fmt.Errorf("create kit directory: %w", err)
		}
		if err := os.WriteFile(kitInit, []byte("local M = {}\nreturn M\n"), 0644); err != nil {
			return fmt.Errorf("create kit init.lua: %w", err)
		}
		fmt.Printf("kit initialized at %s\n", kitPath)
	} else {
		fmt.Printf("kit already exists at %s\n", kitPath)
	}

	// 3. Install skills
	if err := installSkills(home); err != nil {
		return fmt.Errorf("install skills: %w", err)
	}

	// 4. Detect available AI tools
	fmt.Println()
	detectAITools()

	fmt.Println("\nsetup complete")
	return nil
}

func installSkills(home string) error {
	skillDir := filepath.Join(home, ".claude", "skills", "kit-add")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
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

		destPath := filepath.Join(home, ".claude", "skills", path)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", destPath, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("skill installed to %s\n", skillDir)
	return nil
}

func detectAITools() {
	found := false
	if _, err := exec.LookPath("claude"); err == nil {
		fmt.Println("found: claude (Claude Code)")
		fmt.Println("  usage: kael kit add <script-or-url> <namespace>")
		found = true
	}
	if _, err := exec.LookPath("opencode"); err == nil {
		fmt.Println("found: opencode")
		fmt.Println("  usage: kael kit add <script-or-url> <namespace>")
		found = true
	}
	if !found {
		fmt.Println("no AI tools found (claude or opencode)")
		fmt.Println("  install Claude Code: https://docs.anthropic.com/en/docs/claude-code")
		fmt.Println("  you can still add tools manually by writing .lua definitions")
	}
}
