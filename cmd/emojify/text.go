// cmd/emojify/text.go
package main

import (
	"github.com/spf13/cobra"
)

var textCmd = &cobra.Command{
	Use:   "text <passage>",
	Short: "Suggest emoji for the given text, given as an argument instead of stdin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSuggest(cmd.OutOrStdout(), args[0], suggestOpts)
	},
}

func init() {
	registerSuggestFlags(textCmd.Flags())
	rootCmd.AddCommand(textCmd)
}
