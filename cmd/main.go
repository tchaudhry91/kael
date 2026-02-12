package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

// version is set via ldflags during build
var version = "dev"

func main() {
	rootFlags := ff.NewFlagSet("kael")
	helpFlag := rootFlags.BoolLong("help", "h")
	versionFlag := rootFlags.BoolLong("version", "v")
	kitPath := rootFlags.StringLong("kit", defaultKitPath(), "path to kit directory")

	rootCmd := &ff.Command{
		Name:      "kael",
		Usage:     "kael [FLAGS] SUBCOMMAND ...",
		ShortHelp: "Scriptable infrastructure analysis engine",
		Flags:     rootFlags,
		Subcommands: []*ff.Command{
			newRunCmd(rootFlags, kitPath),
			newExecCmd(rootFlags, kitPath),
			newKitCmd(rootFlags, kitPath),
		},
		Exec: func(ctx context.Context, args []string) error {
			return fmt.Errorf("no subcommand provided")
		},
	}

	if err := rootCmd.Parse(os.Args[1:], ff.WithEnvVarPrefix("KAEL")); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *versionFlag {
		fmt.Printf("kael version %s\n", version)
		return
	}
	if *helpFlag {
		fmt.Println(ffhelp.Command(rootCmd))
		return
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		if err.Error() == "no subcommand provided" {
			fmt.Println(ffhelp.Command(rootCmd))
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
