// internal/staticvecs/format_test.go
package staticvecs

import (
	"bytes"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	words := map[string][]float32{
		"sun":    {0.6, 0.8},
		"bright": {0.8, 0.6},
	}
	centroid := []float32{0.707, 0.707}

	var buf bytes.Buffer
	if err := Write(&buf, 2, words, centroid); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, gotCentroid, dims, err := Read(&buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if dims != 2 {
		t.Fatalf("dims = %d, want 2", dims)
	}
	if len(got) != 2 {
		t.Fatalf("got %d words, want 2", len(got))
	}
	const tol = 1.0 / 127
	for w, want := range words {
		vec, ok := got[w]
		if !ok {
			t.Fatalf("word %q missing", w)
		}
		for i := range want {
			if diff := vec[i] - want[i]; diff > tol || diff < -tol {
				t.Errorf("word %q component %d = %v, want ~%v", w, i, vec[i], want[i])
			}
		}
	}
	for i := range centroid {
		if diff := gotCentroid[i] - centroid[i]; diff > tol || diff < -tol {
			t.Errorf("centroid component %d = %v, want ~%v", i, gotCentroid[i], centroid[i])
		}
	}
}
