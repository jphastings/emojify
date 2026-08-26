// index_test.go
package emojify

import (
	"bytes"
	"testing"
)

func unitVector(vals ...float32) []float32 {
	var sumSq float32
	for _, v := range vals {
		sumSq += v * v
	}
	norm := float32(1)
	if sumSq > 0 {
		norm = sqrt32(sumSq)
	}
	out := make([]float32, len(vals))
	for i, v := range vals {
		out[i] = v / norm
	}
	return out
}

func TestWriteReadIndexRoundTrip(t *testing.T) {
	vectors := [][]float32{
		unitVector(1, 0, 0),
		unitVector(0, 1, 0),
		unitVector(0.6, 0.8, 0),
	}
	metas := []Metadata{
		{Emoji: "🌞", Label: "sun with face", Group: "travel-places", Subgroup: "sky-weather", Penalty: 1.0},
		{Emoji: "☕", Label: "hot beverage", Group: "food-drink", Subgroup: "drink", Penalty: 0.85},
		{Emoji: "🥶", Label: "cold face", Group: "smileys-emotion", Subgroup: "face-unwell", Penalty: 1.0},
	}

	var buf bytes.Buffer
	if err := WriteIndex(&buf, "test-model", 3, vectors, metas); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}

	idx, err := ReadIndex(&buf)
	if err != nil {
		t.Fatalf("ReadIndex: %v", err)
	}

	if idx.ModelID != "test-model" {
		t.Errorf("ModelID = %q, want %q", idx.ModelID, "test-model")
	}
	if idx.Dims != 3 {
		t.Errorf("Dims = %d, want 3", idx.Dims)
	}
	if idx.Count != 3 {
		t.Errorf("Count = %d, want 3", idx.Count)
	}
	if len(idx.Metadata) != 3 || idx.Metadata[1].Emoji != "☕" || idx.Metadata[1].Penalty != 0.85 {
		t.Errorf("Metadata[1] = %+v, want emoji ☕ penalty 0.85", idx.Metadata[1])
	}

	// Quantization introduces error bounded by scale/2 = (maxAbs/127)/2 per component.
	const tol = 1.0 / 127
	for i, want := range vectors {
		got := idx.Vectors[i*3 : (i+1)*3]
		for j := range want {
			if diff := got[j] - want[j]; diff > tol || diff < -tol {
				t.Errorf("vector %d component %d = %v, want ~%v (tol %v)", i, j, got[j], want[j], tol)
			}
		}
	}
}

func TestReadIndexRejectsBadMagic(t *testing.T) {
	if _, err := ReadIndex(bytes.NewReader([]byte("not an index at all"))); err == nil {
		t.Fatal("expected error for bad magic, got nil")
	}
}

func TestWriteIndexRejectsMismatchedLengths(t *testing.T) {
	err := WriteIndex(&bytes.Buffer{}, "m", 3, [][]float32{{1, 0, 0}}, nil)
	if err == nil {
		t.Fatal("expected error for mismatched vectors/metadata lengths")
	}
}
