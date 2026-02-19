package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tchaudhry91/kael/engine"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [flags] <script.lua> [-- args...]",
		Short: "Run a Lua script",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := bootstrap(); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			refresh, _ := cmd.Flags().GetBool("refresh")
			return runScript(cmd.Context(), viper.GetString("kit"), refresh, args[0], args[1:])
		},
	}

	cmd.Flags().Bool("refresh", false, "force re-fetch of tool sources")

	return cmd
}

func runScript(ctx context.Context, kitPath string, refresh bool, scriptPath string, scriptArgs []string) error {
	e, err := engine.NewEngine(kitPath)
	if err != nil {
		return fmt.Errorf("engine init: %w", err)
	}
	defer e.Close()

	e.Refresh = refresh
	if len(scriptArgs) > 0 {
		e.SetArgs(scriptArgs)
	}
	return e.RunFile(ctx, scriptPath)
}
