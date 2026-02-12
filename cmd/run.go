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
		Use:   "run [flags] <script.lua>",
		Short: "Run a Lua script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := bootstrap(); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			return runScript(cmd.Context(), viper.GetString("kit"), viper.GetBool("refresh"), args[0])
		},
	}

	cmd.Flags().Bool("refresh", false, "force re-fetch of tool sources")
	viper.BindPFlag("refresh", cmd.Flags().Lookup("refresh"))

	return cmd
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
