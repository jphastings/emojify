// cmd/emojify/server_test.go
package main

import "testing"

func TestServerAddrFlagDefault(t *testing.T) {
	// serverCmd is defined at package init; confirm the --addr default matches the doc's example.
	f := serverCmd.Flags().Lookup("addr")
	if f == nil {
		t.Fatal("expected an --addr flag on the server subcommand")
	}
	if f.DefValue != ":8080" {
		t.Errorf("--addr default = %q, want \":8080\"", f.DefValue)
	}
}
