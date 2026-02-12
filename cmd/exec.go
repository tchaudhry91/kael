package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tchaudhry91/kael/engine"
)

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [flags] <tool.path> [--key value ...]",
		Short: "Execute a single tool from the kit",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := bootstrap(); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			toolPath := args[0]
			toolArgs := args[1:]
			return execTool(cmd.Context(), viper.GetString("kit"), viper.GetBool("refresh"), toolPath, toolArgs)
		},
	}

	// Stop flag parsing after first positional arg (tool path)
	// so that --key value pairs pass through as positional args
	cmd.Flags().SetInterspersed(false)

	cmd.Flags().Bool("refresh", false, "force re-fetch of tool sources")
	viper.BindPFlag("refresh", cmd.Flags().Lookup("refresh"))

	return cmd
}

func execTool(ctx context.Context, kitPath string, refresh bool, toolPath string, toolArgs []string) error {
	e, err := engine.NewEngine(kitPath)
	if err != nil {
		return fmt.Errorf("engine init: %w", err)
	}
	defer e.Close()
	e.Refresh = refresh

	input, err := buildInput(toolArgs)
	if err != nil {
		return fmt.Errorf("input: %w", err)
	}

	result, err := e.ExecTool(ctx, toolPath, input)
	if err != nil {
		return err
	}

	out, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("output marshal: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

// buildInput constructs the tool input map from CLI args and/or stdin.
// If stdin has data (pipe), it's parsed as JSON.
// If CLI args are present (--key value pairs), they're parsed into the map.
// Both can be combined — CLI args override stdin fields.
func buildInput(args []string) (map[string]any, error) {
	input := make(map[string]any)

	// Check for stdin (only if piped, not a terminal)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &input); err != nil {
				return nil, fmt.Errorf("parsing stdin JSON: %w", err)
			}
		}
	}

	// Parse --key value pairs from args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("unexpected argument %q (expected --key or --key value)", arg)
		}
		key := strings.TrimPrefix(arg, "--")
		if key == "" {
			return nil, fmt.Errorf("empty flag name")
		}

		// Check if next arg is a value or another flag
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			// Standalone flag → boolean true
			input[key] = true
			continue
		}

		// Key-value pair
		i++
		input[key] = parseValue(args[i])
	}

	return input, nil
}

// parseValue tries to interpret a string as a number or boolean,
// falling back to string.
func parseValue(s string) any {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		// Only treat as number if it looks like one (not e.g. a URL)
		if !strings.Contains(s, "://") && !strings.Contains(s, "/") {
			return n
		}
	}
	return s
}
