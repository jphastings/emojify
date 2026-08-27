// embedder_static.go
package emojify

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"github.com/jphastings/emojify/internal/staticvecs"
)

//go:embed data/staticvecs.bin
var staticVecsData []byte

//go:embed data/index_static.bin
var staticIndexData []byte

var wordPattern = regexp.MustCompile(`[a-z]+`)

type staticEmbedder struct {
	words    map[string][]float32
	centroid []float32
	dims     int
}

// newDefaultStaticEmbedder returns the compiled-in pure-Go embedder. Always
// available regardless of build tag — it's the fallback NewDefaultEmbedder
// (embedder_select.go) reaches for when ONNX Runtime isn't loadable, and the
// only embedder that exists at all in a bare `go build`.
func newDefaultStaticEmbedder() (Embedder, error) {
	return newStaticEmbedder(staticVecsData)
}

func newStaticEmbedder(data []byte) (*staticEmbedder, error) {
	words, centroid, dims, err := staticvecs.Read(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("emojify: loading static vectors: %w", err)
	}
	return &staticEmbedder{words: words, centroid: centroid, dims: dims}, nil
}

func (e *staticEmbedder) Dims() int { return e.dims }

// defaultIndex pairs this embedder with the index built for its own
// (50-dim) vectors — see indexProvider in embedder.go.
func (e *staticEmbedder) defaultIndex() []byte { return staticIndexData }

func (e *staticEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		sum := make([]float32, e.dims)
		var count int
		for _, w := range wordPattern.FindAllString(strings.ToLower(text), -1) {
			vec, ok := e.words[w]
			if !ok {
				continue
			}
			count++
			for d := range sum {
				sum[d] += vec[d]
			}
		}
		if count == 0 {
			copy(sum, e.centroid)
		}
		out[i] = normalize(sum)
	}
	return out, nil
}
