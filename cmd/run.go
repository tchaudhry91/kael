package main

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"
	"github.com/tchaudhry91/kael/engine"
)

func newRunCmd(rootFlags *ff.FlagSet, kitPath *string) *ff.Command {
	runFlags := ff.NewFlagSet("run").SetParent(rootFlags)
	refresh := runFlags.BoolLong("refresh", "force re-fetch of tool sources")

	return &ff.Command{
		Name:      "run",
		Usage:     "kael [--kit PATH] run [--refresh] <script.lua>",
		ShortHelp: "Run a Lua script",
		Flags:     runFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("script path is required")
			}
			if err := bootstrap(); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			return runScript(ctx, *kitPath, *refresh, args[0])
		},
	}
}

func runScript(ctx context.Context, kitPath string, refresh bool, scriptPath string) error {
	e, err := engine.NewEngine(kitPath)
	if err != nil {
		return fmt.Errorf("engine init: %w", err)
	}
	defer e.Close()

	e.Refresh = refresh
	return e.RunFile(ctx, scriptPath)
}
