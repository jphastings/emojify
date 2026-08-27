// embedder_static.go
//go:build !onnx

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
var defaultIndexData []byte

// DefaultIndexPath is the `index build` output path this build tag actually
// embeds (see defaultIndexData above); mirrors embedder_onnx.go's constant of
// the same name so cmd/emojify compiles unchanged under either tag.
const DefaultIndexPath = "data/index_static.bin"

var wordPattern = regexp.MustCompile(`[a-z]+`)

type staticEmbedder struct {
	words    map[string][]float32
	centroid []float32
	dims     int
}

// NewDefaultEmbedder returns the compiled-in pure-Go embedder. modelPath is
// accepted for interface parity with the onnx build and is ignored.
func NewDefaultEmbedder(modelPath string) (Embedder, error) {
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
