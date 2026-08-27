// cmd/emojify/root.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jphastings/emojify"
)

var errNoMatch = errors.New("no suggestion cleared --min-score")

type suggestOptions struct {
	limit    int
	json     bool
	scores   bool
	minScore float32
}

var suggestOpts suggestOptions

// rootCmd's default behaviour reads stdin only. This resolves the design
// doc's own §6 conflict (bare positional args on root vs. `emojify server`
// needing "server" to always resolve to a subcommand) in favour of its own
// stated decision: positional input requires `emojify text "..."` (text.go).
var rootCmd = &cobra.Command{
	Use:   "emojify",
	Short: "Suggest emoji that match the meaning of a short passage of text",
	Args:  cobra.NoArgs,
	// A --min-score miss (exit code 2, see Execute) is "search returned
	// nothing," not CLI misuse — printing the full flags block after that
	// error is just noise. Genuine usage errors (e.g. an unrecognized flag)
	// still print their own message from cobra; this only silences the
	// trailing usage block.
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		text := strings.TrimRight(string(data), "\n")
		if text == "" {
			return fmt.Errorf("no input: pipe text to stdin, or use `emojify text \"...\"`")
		}
		return runSuggest(cmd.OutOrStdout(), text, suggestOpts)
	},
}

// registerSuggestFlags adds the suggest-related flags to fs, backed by the
// shared suggestOpts. Used by both rootCmd and textCmd (text.go) — the only
// two commands that read suggestOpts — so that server/index build/index
// inspect neither inherit nor advertise flags they never look at. These must
// NOT go on rootCmd.PersistentFlags(): persistent flags are inherited by
// every subcommand, which is exactly how server/index build/index inspect
// ended up silently advertising (and ignoring) --min-score et al.
func registerSuggestFlags(fs *pflag.FlagSet) {
	fs.IntVar(&suggestOpts.limit, "limit", 3, "maximum number of suggestions")
	fs.BoolVar(&suggestOpts.json, "json", false, "output as JSON")
	fs.BoolVar(&suggestOpts.scores, "scores", false, "include similarity scores in plain output")
	fs.Float32Var(&suggestOpts.minScore, "min-score", 0, "drop matches below this score (cosine similarity adjusted by a generic-emoji penalty)")
}

func init() {
	registerSuggestFlags(rootCmd.Flags())
}

// Execute runs the CLI. Exit codes: 0 output produced, 1 error, 2 nothing
// cleared --min-score (so shell callers can distinguish "no good match" from "broken").
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}
	if errors.Is(err, errNoMatch) {
		os.Exit(2)
	}
	os.Exit(1)
}

func runSuggest(w io.Writer, text string, opts suggestOptions) error {
	m, err := emojify.New(emojify.WithMinScore(opts.minScore))
	if err != nil {
		return err
	}
	defer m.Close()

	suggestions, err := m.Suggest(context.Background(), text, opts.limit)
	if err != nil {
		return err
	}
	if len(suggestions) == 0 {
		return errNoMatch
	}

	if opts.json {
		return json.NewEncoder(w).Encode(suggestions)
	}
	parts := make([]string, len(suggestions))
	for i, s := range suggestions {
		if opts.scores {
			parts[i] = fmt.Sprintf("%s(%.2f)", s.Emoji, s.Score)
		} else {
			parts[i] = s.Emoji
		}
	}
	fmt.Fprintln(w, strings.Join(parts, " "))
	return nil
}
