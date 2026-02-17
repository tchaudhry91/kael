package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// version is set via ldflags during build
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "kael [flags] [subcommand]",
	Short:   "Scriptable infrastructure analysis engine",
	Version: version,
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && strings.HasSuffix(args[0], ".lua") {
			if err := bootstrap(); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			return runScript(cmd.Context(), viper.GetString("kit"), viper.GetBool("refresh"), args[0])
		}
		return cmd.Help()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("kit", defaultKitPath(), "path to kit directory")
	viper.BindPFlag("kit", rootCmd.PersistentFlags().Lookup("kit"))

	rootCmd.SetVersionTemplate("kael version {{.Version}}\n")

	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newExecCmd())
	rootCmd.AddCommand(newKitCmd())
	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newReplCmd())
}

func initConfig() {
	viper.SetEnvPrefix("KAEL")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(filepath.Join(home, ".kael"))
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}
	// Silently ignore missing config file
	viper.ReadInConfig()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
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
