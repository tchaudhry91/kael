package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"github.com/tchaudhry91/kael/engine"
)

// version is set via ldflags during build
var version = "dev"

func main() {
	rootFlags := ff.NewFlagSet("kael")
	helpFlag := rootFlags.BoolLong("help", "h")
	versionFlag := rootFlags.BoolLong("version", "v")
	kitPath := rootFlags.StringLong("kit", defaultKitPath(), "path to kit directory")

	runFlags := ff.NewFlagSet("run").SetParent(rootFlags)
	refresh := runFlags.BoolLong("refresh", "force re-fetch of tool sources")
	runCmd := &ff.Command{
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

	rootCmd := &ff.Command{
		Name:        "kael",
		Usage:       "kael [FLAGS] SUBCOMMAND ...",
		ShortHelp:   "Scriptable infrastructure analysis engine",
		Flags:       rootFlags,
		Subcommands: []*ff.Command{runCmd},
		Exec: func(ctx context.Context, args []string) error {
			return fmt.Errorf("no subcommand provided")
		},
	}

	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:], ff.WithEnvVarPrefix("KAEL")); err != nil {
		if *versionFlag {
			fmt.Printf("kael version %s\n", version)
			return
		}
		if *helpFlag {
			fmt.Println(ffhelp.Command(rootCmd))
			return
		}
		fmt.Println(ffhelp.Command(rootCmd))
		if err.Error() == "no subcommand provided" {
			os.Exit(0)
		}
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func defaultKitPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.kael/kit"
	}
	return filepath.Join(home, ".kael", "kit")
}

func bootstrap() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	kaelDir := filepath.Join(home, ".kael")
	return os.MkdirAll(kaelDir, 0755)
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
