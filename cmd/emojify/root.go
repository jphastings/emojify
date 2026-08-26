// cmd/emojify/root.go
package main

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "emojify",
	Short: "Suggest emoji that match the meaning of a short passage of text",
}

// Execute runs the CLI, exiting the process with an appropriate code.
// Task 7 replaces this with the full stdin/positional/exit-code behaviour;
// this minimal version exists so `index`/`server` subcommands have a root
// to attach to.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
