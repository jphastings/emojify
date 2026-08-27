// embedder_select_onnx_test.go
//go:build onnx

package emojify

import (
	"context"
	"testing"
)

// EMOJIFY_EMBEDDER=static forces the pure-Go embedder even in an onnx build,
// and New() must pair it with the matching 50-dim index rather than the
// 384-dim one also baked into this binary — that pairing following the
// runtime choice is the whole point of making -tags onnx additive.
func TestEmojifyEmbedderStaticOverrideInOnnxBuild(t *testing.T) {
	t.Setenv("EMOJIFY_EMBEDDER", "static")

	emb, err := NewDefaultEmbedder("")
	if err != nil {
		t.Fatalf("NewDefaultEmbedder: %v", err)
	}
	if emb.Dims() != 50 {
		t.Fatalf("Dims() = %d, want 50 (static embedder)", emb.Dims())
	}

	m, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer m.Close()

	suggestions, err := m.Suggest(context.Background(), "let's get pizza", 1)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(suggestions))
	}
}
