// embedder_onnx_test.go
//go:build onnx

package emojify

import (
	"context"
	"testing"
)

func TestONNXEmbedderEmbedsAndNormalizes(t *testing.T) {
	emb, err := NewDefaultEmbedder("")
	if err != nil {
		t.Fatalf("NewDefaultEmbedder: %v", err)
	}
	defer emb.(interface{ Close() error }).Close()

	if emb.Dims() != 384 {
		t.Fatalf("Dims() = %d, want 384", emb.Dims())
	}

	vecs, err := emb.Embed(context.Background(), []string{
		"such a beautiful sunny afternoon",
		"I am freezing cold",
	})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 384 {
		t.Fatalf("got %d vectors of len %d, want 2 of len 384", len(vecs), len(vecs[0]))
	}

	for i, v := range vecs {
		var normSq float32
		for _, f := range v {
			normSq += f * f
		}
		if normSq < 0.98 || normSq > 1.02 {
			t.Errorf("vector %d squared norm = %v, want ~1 (unit-normalised)", i, normSq)
		}
	}

	simSame := dot(vecs[0], vecs[0])
	simDiff := dot(vecs[0], vecs[1])
	if simSame <= simDiff {
		t.Errorf("a vector should be most similar to itself: sim(a,a)=%v sim(a,b)=%v", simSame, simDiff)
	}
}
