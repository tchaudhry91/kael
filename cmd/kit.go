package main

import (
	"context"
	"fmt"
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
		Subcommands: []*ff.Command{listCmd, validateCmd, describeCmd},
		Exec: func(ctx context.Context, args []string) error {
			return fmt.Errorf("kit subcommand required: list, validate, describe")
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
