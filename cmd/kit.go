package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v4"
	"github.com/tchaudhry91/kael/engine"
)

func newKitCmd(rootFlags *ff.FlagSet, kitPath *string) *ff.Command {
	kitFlags := ff.NewFlagSet("kit").SetParent(rootFlags)

	listCmd := &ff.Command{
		Name:      "list",
		Usage:     "kael [--kit PATH] kit list",
		ShortHelp: "List all tools in the kit",
		Flags:     ff.NewFlagSet("list").SetParent(kitFlags),
		Exec: func(ctx context.Context, args []string) error {
			return kitList(*kitPath)
		},
	}

	validateCmd := &ff.Command{
		Name:      "validate",
		Usage:     "kael [--kit PATH] kit validate",
		ShortHelp: "Validate the kit loads without errors",
		Flags:     ff.NewFlagSet("validate").SetParent(kitFlags),
		Exec: func(ctx context.Context, args []string) error {
			return kitValidate(*kitPath)
		},
	}

	initCmd := &ff.Command{
		Name:      "init",
		Usage:     "kael [--kit PATH] kit init [namespace]",
		ShortHelp: "Initialize kit directory or add a namespace",
		Flags:     ff.NewFlagSet("init").SetParent(kitFlags),
		Exec: func(ctx context.Context, args []string) error {
			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}
			return kitInit(*kitPath, namespace)
		},
	}

	describeCmd := &ff.Command{
		Name:      "describe",
		Usage:     "kael [--kit PATH] kit describe <tool.path>",
		ShortHelp: "Show details of a specific tool",
		Flags:     ff.NewFlagSet("describe").SetParent(kitFlags),
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("tool path is required (e.g. kubernetes.pods_on_node)")
			}
			return kitDescribe(*kitPath, args[0])
		},
	}

	return &ff.Command{
		Name:        "kit",
		Usage:       "kael [--kit PATH] kit SUBCOMMAND",
		ShortHelp:   "Inspect and manage the tool kit",
		Flags:       kitFlags,
		Subcommands: []*ff.Command{initCmd, listCmd, validateCmd, describeCmd},
		Exec: func(ctx context.Context, args []string) error {
			return fmt.Errorf("kit subcommand required: init, list, validate, describe")
		},
	}
}

func kitList(kitPath string) error {
	e, err := engine.NewEngine(kitPath)
	if err != nil {
		return fmt.Errorf("kit load: %w", err)
	}
	defer e.Close()

	root := e.ListTools()
	printNode(root, "")
	return nil
}

func printNode(node *engine.KitNode, prefix string) {
	// Collect and sort namespace names
	nsNames := make([]string, 0, len(node.Children))
	for name := range node.Children {
		nsNames = append(nsNames, name)
	}
	sort.Strings(nsNames)

	// Collect and sort tool names
	toolNames := make([]string, 0, len(node.Tools))
	for name := range node.Tools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	// Print namespaces first
	for _, name := range nsNames {
		fmt.Printf("%s%s/\n", prefix, name)
		printNode(node.Children[name], prefix+"  ")
	}

	// Print tools
	for _, name := range toolNames {
		cfg := node.Tools[name]
		executor := cfg.Executor
		if executor == "" {
			executor = "docker"
		}
		detail := cfg.Source
		if cfg.Entrypoint != "" {
			detail += "  " + cfg.Entrypoint
		}
		if cfg.Type != "" {
			detail += fmt.Sprintf("  (%s)", cfg.Type)
		}
		fmt.Printf("%s%-20s  %-8s  %s\n", prefix, name, executor, detail)
	}
}

func kitValidate(kitPath string) error {
	e, err := engine.NewEngine(kitPath)
	if err != nil {
		return fmt.Errorf("kit validation failed: %w", err)
	}
	defer e.Close()

	root := e.ListTools()
	toolCount, nsCount := countTree(root)
	fmt.Printf("kit OK: %d tools across %d namespaces\n", toolCount, nsCount)
	return nil
}

func countTree(node *engine.KitNode) (tools, namespaces int) {
	tools = len(node.Tools)
	for _, child := range node.Children {
		ct, cn := countTree(child)
		tools += ct
		namespaces += cn + 1
	}
	return
}

func kitDescribe(kitPath, toolPath string) error {
	e, err := engine.NewEngine(kitPath)
	if err != nil {
		return fmt.Errorf("kit load: %w", err)
	}
	defer e.Close()

	root := e.ListTools()
	cfg, err := resolveTool(root, toolPath)
	if err != nil {
		return err
	}

	fmt.Printf("Tool:       %s\n", toolPath)
	fmt.Printf("Source:     %s\n", cfg.Source)
	if cfg.Entrypoint != "" {
		fmt.Printf("Entrypoint: %s\n", cfg.Entrypoint)
	}
	executor := cfg.Executor
	if executor == "" {
		executor = "docker"
	}
	fmt.Printf("Executor:   %s\n", executor)
	if cfg.Type != "" {
		fmt.Printf("Type:       %s\n", cfg.Type)
	}
	if cfg.Tag != "" {
		fmt.Printf("Tag:        %s\n", cfg.Tag)
	}
	if cfg.SubDir != "" {
		fmt.Printf("SubDir:     %s\n", cfg.SubDir)
	}
	if cfg.Timeout > 0 {
		fmt.Printf("Timeout:    %ds\n", cfg.Timeout)
	}
	if len(cfg.Deps) > 0 {
		fmt.Printf("Deps:       %s\n", strings.Join(cfg.Deps, ", "))
	}
	if len(cfg.Env) > 0 {
		fmt.Printf("Env:        %s\n", strings.Join(cfg.Env, ", "))
	}

	if cfg.Schema != nil {
		if len(cfg.Schema.Input) > 0 {
			fmt.Println("\nInput:")
			printSchemaFields(cfg.Schema.Input)
		}
		if len(cfg.Schema.Output) > 0 {
			fmt.Println("\nOutput:")
			printSchemaFields(cfg.Schema.Output)
		}
	}

	return nil
}

func printSchemaFields(fields map[string]engine.FieldSchema) {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f := fields[name]
		req := "required"
		if !f.Required {
			req = "optional"
		}
		desc := ""
		if f.Description != "" {
			desc = "  " + f.Description
		}
		fmt.Printf("  %-20s  %-8s  %s%s\n", name, f.Type, req, desc)
	}
}

func kitInit(kitPath, namespace string) error {
	// Create kit directory
	if err := os.MkdirAll(kitPath, 0755); err != nil {
		return fmt.Errorf("create kit directory: %w", err)
	}

	// Create top-level init.lua if it doesn't exist
	topInit := filepath.Join(kitPath, "init.lua")
	if _, err := os.Stat(topInit); os.IsNotExist(err) {
		if err := os.WriteFile(topInit, []byte("local M = {}\nreturn M\n"), 0644); err != nil {
			return fmt.Errorf("create init.lua: %w", err)
		}
		fmt.Printf("created %s\n", topInit)
	}

	if namespace == "" {
		fmt.Printf("kit initialized at %s\n", kitPath)
		return nil
	}

	// Split dotted namespace into parts: "monitoring.prometheus" → ["monitoring", "prometheus"]
	parts := strings.Split(namespace, ".")

	// Walk the chain, creating each level
	for i := range parts {
		// Directory for this level
		dirParts := append([]string{kitPath}, parts[:i+1]...)
		nsDir := filepath.Join(dirParts...)
		if err := os.MkdirAll(nsDir, 0755); err != nil {
			return fmt.Errorf("create namespace directory: %w", err)
		}

		// init.lua for this level
		nsInit := filepath.Join(nsDir, "init.lua")
		if _, err := os.Stat(nsInit); os.IsNotExist(err) {
			if err := os.WriteFile(nsInit, []byte("local M = {}\nreturn M\n"), 0644); err != nil {
				return fmt.Errorf("create namespace init.lua: %w", err)
			}
			fmt.Printf("created %s\n", nsInit)
		}

		// Wire into parent's init.lua
		var parentInit string
		if i == 0 {
			parentInit = topInit
		} else {
			parentParts := append([]string{kitPath}, parts[:i]...)
			parentInit = filepath.Join(append(parentParts, "init.lua")...)
		}
		// Require path is dotted: "monitoring.prometheus"
		requirePath := strings.Join(parts[:i+1], ".")
		childName := parts[i]

		if err := wireNamespace(parentInit, childName, requirePath); err != nil {
			return fmt.Errorf("wire namespace: %w", err)
		}
	}

	leafDir := filepath.Join(append([]string{kitPath}, parts...)...)
	fmt.Printf("namespace %q ready at %s\n", namespace, leafDir)
	return nil
}

// wireNamespace adds M.<name> = require("<requirePath>") to an init.lua file
// if it's not already present. name is the local key, requirePath is the dotted
// Lua require path (e.g. "monitoring.prometheus").
func wireNamespace(initPath, name, requirePath string) error {
	data, err := os.ReadFile(initPath)
	if err != nil {
		return err
	}

	content := string(data)
	requireLine := fmt.Sprintf("M.%s = require(\"%s\")", name, requirePath)

	// Already wired
	if strings.Contains(content, requireLine) {
		return nil
	}

	// Insert before "return M"
	newContent := strings.Replace(content, "return M", requireLine+"\nreturn M", 1)
	if newContent == content {
		return fmt.Errorf("could not find 'return M' in %s", initPath)
	}

	return os.WriteFile(initPath, []byte(newContent), 0644)
}

// resolveTool walks the KitNode tree using a dotted path like "kubernetes.pods_on_node".
func resolveTool(root *engine.KitNode, path string) (*engine.ToolConfig, error) {
	parts := strings.Split(path, ".")
	node := root
	for i, part := range parts {
		// Last part: look for a tool
		if i == len(parts)-1 {
			if cfg, ok := node.Tools[part]; ok {
				return &cfg, nil
			}
			return nil, fmt.Errorf("tool %q not found", path)
		}
		// Intermediate part: descend into child namespace
		child, ok := node.Children[part]
		if !ok {
			return nil, fmt.Errorf("namespace %q not found in path %q", part, path)
		}
		node = child
	}
	return nil, fmt.Errorf("tool %q not found", path)
}
