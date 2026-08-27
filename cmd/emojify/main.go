// cmd/emojify/main.go
package main

// Set via -ldflags "-X main.version=... -X main.commit=... -X main.date=..."
// at build time (goreleaser does this for released binaries); "dev" covers
// a plain `go build`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	Execute()
}
