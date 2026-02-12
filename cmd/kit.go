package main

import (
	"context"
	"fmt"
	"sort"

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

	return &ff.Command{
		Name:        "kit",
		Usage:       "kael [--kit PATH] kit SUBCOMMAND",
		ShortHelp:   "Inspect and manage the tool kit",
		Flags:       kitFlags,
		Subcommands: []*ff.Command{listCmd, validateCmd},
		Exec: func(ctx context.Context, args []string) error {
			return fmt.Errorf("kit subcommand required: list, validate")
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
