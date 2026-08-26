// embedder.go
package emojify

import "context"

// Embedder turns text into unit-normalised vectors for semantic comparison.
// Two implementations exist behind build tags (embedder_onnx.go, the
// default; embedder_static.go for !onnx); rank.go never knows which is active.
type Embedder interface {
	// Embed returns one unit-normalised vector per input text, in order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dims reports the vector width this embedder produces.
	Dims() int
}
