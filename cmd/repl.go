package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tchaudhry91/kael/engine"
)

// ANSI color helpers
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
)

var (
	promptMain = colorCyan + "kael" + colorReset + colorDim + "> " + colorReset
	promptCont = colorDim + " ...> " + colorReset
)

func newReplCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "repl",
		Short: "Start an interactive Lua REPL with kit loaded",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := bootstrap(); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			return startRepl(viper.GetString("kit"), viper.GetBool("refresh"))
		},
	}
}

func startRepl(kitPath string, refresh bool) error {
	e, err := engine.NewEngine(kitPath)
	if err != nil {
		return fmt.Errorf("engine init: %w", err)
	}
	defer e.Close()
	e.Refresh = refresh

	// Build completions from kit tree
	completer := buildCompleter(e)

	// History file
	home, _ := os.UserHomeDir()
	historyFile := filepath.Join(home, ".kael", "history")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          promptMain,
		HistoryFile:     historyFile,
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("readline init: %w", err)
	}
	defer rl.Close()

	fmt.Printf("%skael repl%s — tab to complete, ctrl-d to exit\n", colorBold, colorReset)

	var buf strings.Builder

	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if buf.Len() > 0 {
				buf.Reset()
				rl.SetPrompt(promptMain)
			}
			continue
		}
		if err == io.EOF {
			fmt.Println()
			return nil
		}

		line = strings.TrimSpace(line)

		if line == "" && buf.Len() == 0 {
			continue
		}

		if buf.Len() == 0 && (line == "exit" || line == "quit") {
			return nil
		}

		// Accumulate into buffer
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(line)

		code := buf.String()

		// Try to compile — check if it's incomplete
		if syntaxErr := e.CheckSyntax(code); syntaxErr != nil {
			if isIncomplete(syntaxErr) {
				indent := countIndent(code)
				rl.SetPrompt(promptCont + strings.Repeat("  ", indent))
				continue
			}
			// Real syntax error
			replError(syntaxErr.Error())
			buf.Reset()
			rl.SetPrompt(promptMain)
			continue
		}

		// Valid syntax — execute
		if err := e.RunString(context.Background(), code); err != nil {
			replError(err.Error())
		}

		buf.Reset()
		rl.SetPrompt(promptMain)
	}
}

func replError(msg string) {
	fmt.Fprintf(os.Stderr, "%serror:%s %s\n", colorRed, colorReset, msg)
}

// countIndent estimates the current block nesting depth by counting
// Lua block openers vs closers in the accumulated code.
func countIndent(code string) int {
	// Tokenize loosely by words
	words := strings.Fields(code)
	depth := 0
	for _, w := range words {
		switch w {
		case "do", "then", "repeat":
			depth++
		case "end", "until":
			depth--
		case "{":
			depth++
		case "}":
			depth--
		default:
			// function(...) — count "function" keyword
			if w == "function" || strings.HasPrefix(w, "function(") {
				depth++
			}
		}
	}
	if depth < 0 {
		depth = 0
	}
	return depth
}

// isIncomplete checks if a Lua compile error indicates an unterminated statement.
func isIncomplete(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "at EOF") || strings.Contains(msg, "<eof>")
}

// buildCompleter creates a readline completer from the kit tree.
func buildCompleter(e *engine.Engine) *kitCompleter {
	root := e.ListTools()
	paths := collectPaths("kit", root)
	return &kitCompleter{paths: paths}
}

// collectPaths flattens a KitNode tree into dotted path strings.
func collectPaths(prefix string, node *engine.KitNode) []string {
	if node == nil {
		return nil
	}
	var paths []string

	for name := range node.Children {
		childPrefix := prefix + "." + name
		paths = append(paths, childPrefix)
		paths = append(paths, collectPaths(childPrefix, node.Children[name])...)
	}

	for name := range node.Tools {
		paths = append(paths, prefix+"."+name)
	}

	return paths
}

// kitCompleter implements readline.AutoCompleter for kit paths.
type kitCompleter struct {
	paths []string
}

func (c *kitCompleter) Do(line []rune, pos int) ([][]rune, int) {
	wordStart := pos
	for wordStart > 0 && !isBreak(line[wordStart-1]) {
		wordStart--
	}
	word := string(line[wordStart:pos])

	if word == "" {
		return nil, 0
	}

	if !strings.HasPrefix(word, "k") && !strings.HasPrefix("kit", word) {
		return nil, 0
	}

	var candidates [][]rune
	for _, path := range c.paths {
		if strings.HasPrefix(path, word) {
			suffix := path[len(word):]
			candidates = append(candidates, []rune(suffix))
		}
	}

	return candidates, 0
}

// isBreak returns true for characters that break a word in Lua.
func isBreak(r rune) bool {
	switch r {
	case ' ', '\t', '(', ')', '{', '}', '[', ']', ',', ';', '=', '\n':
		return true
	}
	return false
}
