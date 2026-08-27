// server/static.go
package server

import (
	_ "embed"
	"net/http"
)

// indexHTML is the demo/landing page served at "/". It is deliberately
// self-contained — no webfonts, CDN scripts, or external assets — because
// this binary is meant to be self-hosted (a Pi on someone's desk, an
// air-gapped box), and a landing page that phones out to a third party to
// render is a poor fit for that.
//
//go:embed index.html
var indexHTML []byte

func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}
