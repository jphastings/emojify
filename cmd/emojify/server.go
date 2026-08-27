// cmd/emojify/server.go
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/jphastings/emojify"
	"github.com/jphastings/emojify/server"
)

var serverAddr string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the emojify XRPC server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer(serverAddr)
	},
}

func init() {
	serverCmd.Flags().StringVar(&serverAddr, "addr", ":8080", "address to listen on")
	rootCmd.AddCommand(serverCmd)
}

func runServer(addr string) error {
	matcher, err := emojify.New()
	if err != nil {
		return fmt.Errorf("loading matcher: %w", err)
	}
	defer matcher.Close()

	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(matcher),
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Fprintf(os.Stderr, "emojify server listening on %s\n", addr)
	return srv.ListenAndServe()
}
