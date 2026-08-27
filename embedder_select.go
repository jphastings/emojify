// embedder_select.go
package emojify

import (
	"fmt"
	"os"
)

// NewDefaultEmbedder builds this process's embedder. -tags onnx is additive,
// not exclusive: a binary built with it contains both embedders and picks at
// runtime — ONNX Runtime if the shared library can actually be dlopen'd,
// the pure-Go static embedder otherwise. A bare `go build` never has ONNX
// compiled in at all, so ortAvailable is always false and this always
// resolves to static.
//
// EMOJIFY_EMBEDDER overrides the automatic choice:
//   - "static" always uses the pure-Go embedder.
//   - "onnx" requires ONNX Runtime to be loadable; it errors rather than
//     silently falling back, since the caller explicitly asked for it.
//   - "" or "auto" (the default) prefers ONNX when available, but — unlike
//     the automatic fallback — a *broken* ONNX (available but failing to
//     construct, e.g. a corrupt model) is reported, not masked.
//
// modelPath is forwarded to the ONNX embedder only; see newONNXEmbedder's own
// resolution order for what an empty value does there.
func NewDefaultEmbedder(modelPath string) (Embedder, error) {
	switch mode := os.Getenv("EMOJIFY_EMBEDDER"); mode {
	case "static":
		return newDefaultStaticEmbedder()
	case "onnx":
		// Delegate even when unavailable, rather than reporting a single
		// generic message: "this binary has no ONNX compiled in" and "it does,
		// but the library isn't where I looked" have different fixes, and only
		// the tag-gated shim knows which case this is.
		emb, err := newONNXEmbedderOrErr(modelPath)
		if err != nil {
			return nil, fmt.Errorf("emojify: EMOJIFY_EMBEDDER=onnx requested but unavailable: %w", err)
		}
		return emb, nil
	case "", "auto":
		if ortAvailable() {
			return newONNXEmbedderOrErr(modelPath)
		}
		return newDefaultStaticEmbedder()
	default:
		return nil, fmt.Errorf("emojify: invalid EMOJIFY_EMBEDDER %q — want \"auto\", \"onnx\", \"static\", or unset", mode)
	}
}
