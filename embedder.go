// embedder.go
package emojify

import "context"

// Embedder turns text into unit-normalised vectors for semantic comparison.
// Two implementations exist behind build tags: embedder_onnx.go (opt-in via
// `-tags onnx`; the real ONNX Runtime model, production quality) and
// embedder_static.go, the pure-Go fallback that a bare `go build` (no -tags
// at all) actually produces — measured at a fraction of the onnx embedder's
// match quality on the project's golden table. rank.go never knows which is
// active.
type Embedder interface {
	// Embed returns one unit-normalised vector per input text, in order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dims reports the vector width this embedder produces.
	Dims() int
}
