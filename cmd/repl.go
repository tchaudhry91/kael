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
	lua "github.com/yuin/gopher-lua"
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
	defer func() { _ = rl.Close() }()

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

		if buf.Len() == 0 && line == "help" {
			printReplHelp()
			continue
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
		// Try as expression first for auto-print (like Python REPL)
		exprCode := "return (" + code + ")"
		if e.CheckSyntax(exprCode) == nil {
			result, err := e.RunStringResult(context.Background(), exprCode)
			if err != nil {
				replError(err.Error())
			} else if result != nil && result != lua.LNil {
				fmt.Println(engine.FormatLuaValuePP(result, 4))
			}
		} else {
			if err := e.RunString(context.Background(), code); err != nil {
				replError(err.Error())
			}
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

	if !strings.HasPrefix(word, "k") && !strings.HasPrefix("kit", word) { //nolint:gocritic // intentional: checks if "kit" starts with partial input
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

func printReplHelp() {
	fmt.Print(colorBold + "Helpers" + colorReset + `
  pp(val, depth?)          pretty-print a value (default depth 4)
  keys(tbl)                list all keys in a table
  pluck(list, field)       extract one field from each item in a list
  count(tbl)               count entries in a table (arrays and maps)
  jq(val, filter)          pipe a value through jq
  readfile(path)           read a file, return contents as string
  writefile(path, str)     write a string to a file
  inspect(val)             return string representation of a value
  json.encode(val)         serialize to JSON string
  json.pretty(val)         serialize to indented JSON string
  json.decode(str)         parse a JSON string

` + colorBold + "REPL" + colorReset + `
  Expressions auto-print their result
  Tab completes kit.* tool paths
  Multiline blocks (if/for/function) are detected automatically
  Use globals (not local) for variables that persist across lines
  help                     show this help
  exit / quit / ctrl-d     exit the REPL
`)
}
