// embedder_static_test.go
//go:build !onnx

package emojify

import (
	"bytes"
	"context"
	"testing"

	"github.com/jphastings/emojify/internal/staticvecs"
)

func TestStaticEmbedderEmbed(t *testing.T) {
	words := map[string][]float32{
		"sun":    {1, 0},
		"bright": {0.9, 0.1},
		"cold":   {-1, 0},
	}
	centroid := []float32{0.5, 0.5}
	var buf bytes.Buffer
	if err := staticvecs.Write(&buf, 2, words, centroid); err != nil {
		t.Fatal(err)
	}

	emb, err := newStaticEmbedder(buf.Bytes())
	if err != nil {
		t.Fatalf("newStaticEmbedder: %v", err)
	}
	if emb.Dims() != 2 {
		t.Fatalf("Dims() = %d, want 2", emb.Dims())
	}

	vecs, err := emb.Embed(context.Background(), []string{"sun bright", "cold", "unknownword"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("got %d vectors, want 3", len(vecs))
	}

	// "sun bright" should land closer to "sun" than "cold" does.
	simSunBright := dot(vecs[0], []float32{1, 0})
	simCold := dot(vecs[1], []float32{1, 0})
	if simSunBright <= simCold {
		t.Errorf("sim(sun bright, sun-dir)=%v should exceed sim(cold, sun-dir)=%v", simSunBright, simCold)
	}

	// Every output vector must be unit-normalised.
	for i, v := range vecs {
		n := dot(v, v)
		if n < 0.99 || n > 1.01 {
			t.Errorf("vector %d has squared norm %v, want ~1", i, n)
		}
	}

	// Out-of-vocabulary input falls back to the centroid, still normalised.
	simCentroid := dot(vecs[2], unitOf(centroid))
	if simCentroid < 0.99 {
		t.Errorf("OOV embedding should equal normalised centroid, got sim %v", simCentroid)
	}
}

func unitOf(v []float32) []float32 {
	n := float32(0)
	for _, f := range v {
		n += f * f
	}
	n = sqrt32(n)
	out := make([]float32, len(v))
	for i, f := range v {
		out[i] = f / n
	}
	return out
}

func TestStaticEmbedderLoadsRealData(t *testing.T) {
	emb, err := NewDefaultEmbedder("")
	if err != nil {
		t.Fatalf("NewDefaultEmbedder: %v", err)
	}
	vecs, err := emb.Embed(context.Background(), []string{"sun", "sunny weather"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if sim := dot(vecs[0], vecs[1]); sim < 0.3 {
		t.Errorf("sim(sun, sunny weather) = %v, expected clearly positive", sim)
	}
}
