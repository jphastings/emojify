// embedder.go
package emojify

import "context"

// Embedder turns text into unit-normalised vectors for semantic comparison.
// Two implementations exist: embedder_onnx.go (compiled in only via `-tags
// onnx`; the real ONNX Runtime model, production quality) and
// embedder_static.go, the pure-Go fallback that's always compiled in and is
// all a bare `go build` (no -tags at all) ever has — measured at a fraction
// of the onnx embedder's match quality on the project's golden table.
// NewDefaultEmbedder (embedder_select.go) picks between them at runtime when
// both are compiled in. rank.go never knows which is active.
type Embedder interface {
	// Embed returns one unit-normalised vector per input text, in order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dims reports the vector width this embedder produces.
	Dims() int
}

// indexProvider is implemented by the embedders NewDefaultEmbedder builds, so
// New can pair each with the index built for its own model — the runtime
// ONNX/static choice changes the vector width, and an index of the wrong
// width is unusable.
type indexProvider interface{ defaultIndex() []byte }
