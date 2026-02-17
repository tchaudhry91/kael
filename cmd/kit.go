package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tchaudhry91/kael/engine"
)

func newKitCmd() *cobra.Command {
	kitCmd := &cobra.Command{
		Use:   "kit [subcommand]",
		Short: "Inspect and manage the tool kit",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all tools in the kit",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return kitList(viper.GetString("kit"))
		},
	}

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the kit loads without errors",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return kitValidate(viper.GetString("kit"))
		},
	}

	initCmd := &cobra.Command{
		Use:   "init [namespace]",
		Short: "Initialize kit directory or add a namespace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}
			return kitInit(viper.GetString("kit"), namespace)
		},
	}

	describeCmd := &cobra.Command{
		Use:   "describe <tool.path>",
		Short: "Show details of a specific tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return kitDescribe(viper.GetString("kit"), args[0])
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <tool.path>",
		Short: "Remove a tool from the kit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return kitRemove(viper.GetString("kit"), args[0])
		},
	}

	kitCmd.AddCommand(listCmd, validateCmd, initCmd, describeCmd, newKitAddCmd(), removeCmd)
	return kitCmd
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
		nsDir := filepath.Join(kitPath, filepath.Join(parts[:i+1]...))
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
			parentInit = filepath.Join(kitPath, filepath.Join(parts[:i]...), "init.lua")
		}
		// Require path is dotted: "monitoring.prometheus"
		requirePath := strings.Join(parts[:i+1], ".")
		childName := parts[i]

		if err := wireNamespace(parentInit, childName, requirePath); err != nil {
			return fmt.Errorf("wire namespace: %w", err)
		}
	}

	leafDir := filepath.Join(kitPath, filepath.Join(parts...))
	fmt.Printf("namespace %q ready at %s\n", namespace, leafDir)
	return nil
}

func kitRemove(kitPath, toolPath string) error {
	// 1. Verify the tool exists
	e, err := engine.NewEngine(kitPath)
	if err != nil {
		return fmt.Errorf("kit load: %w", err)
	}
	defer e.Close()

	root := e.ListTools()
	if _, err := resolveTool(root, toolPath); err != nil {
		return err
	}

	// 2. Split path into namespace parts + tool name
	parts := strings.Split(toolPath, ".")
	toolName := parts[len(parts)-1]
	nsParts := parts[:len(parts)-1]

	// 3. Delete the Lua file
	luaFile := filepath.Join(kitPath, filepath.Join(nsParts...), toolName+".lua")
	if err := os.Remove(luaFile); err != nil {
		return fmt.Errorf("remove %s: %w", luaFile, err)
	}
	fmt.Printf("removed %s\n", luaFile)

	// 4. Remove require line from parent init.lua
	var parentInit string
	if len(nsParts) == 0 {
		parentInit = filepath.Join(kitPath, "init.lua")
	} else {
		parentInit = filepath.Join(kitPath, filepath.Join(nsParts...), "init.lua")
	}

	requirePath := toolPath
	if err := unwireNamespace(parentInit, toolName, requirePath); err != nil {
		return fmt.Errorf("unwire namespace: %w", err)
	}
	fmt.Printf("unwired %s from %s\n", toolName, parentInit)

	// 5. Validate
	if err := kitValidate(kitPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: kit validation failed: %v\n", err)
	}

	return nil
}

// unwireNamespace removes M.<name> = require("<requirePath>") from an init.lua file.
func unwireNamespace(initPath, name, requirePath string) error {
	data, err := os.ReadFile(initPath)
	if err != nil {
		return err
	}

	requireLine := fmt.Sprintf("M.%s = require(\"%s\")\n", name, requirePath)
	content := string(data)
	newContent := strings.Replace(content, requireLine, "", 1)
	if newContent == content {
		return fmt.Errorf("could not find require line for %q in %s", name, initPath)
	}

	return os.WriteFile(initPath, []byte(newContent), 0644)
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
