// internal/indexbuild/build_test.go
package indexbuild

import (
	"bytes"
	"context"
	"testing"

	"github.com/jphastings/emojify"
)

// fakeEmbedder: "generic" texts (containing the word "thing") land near the
// origin-ish shared direction so BuildIndex's penalty pass has something
// real to detect; "sun"/"cold" get distinct, non-generic directions.
type fakeEmbedder struct{}

func (fakeEmbedder) Dims() int { return 2 }
func (fakeEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		switch {
		case t == "sun with face. bright, sun. travel-places, sky-weather.":
			out[i] = []float32{1, 0}
		case t == "cold face. cold, freezing. smileys-emotion, face-unwell.":
			out[i] = []float32{0, 1}
		case t == "generic thing. thing, stuff. objects, other.":
			out[i] = []float32{0.9, 0.9} // deliberately close to every neutral sentence below
		default: // neutral corpus sentences
			out[i] = []float32{0.9, 0.85}
		}
		out[i] = normalize2(out[i])
	}
	return out, nil
}

func normalize2(v []float32) []float32 {
	n := float32(0)
	for _, f := range v {
		n += f * f
	}
	if n == 0 {
		return v
	}
	root := n
	for i := 0; i < 20; i++ {
		root = 0.5 * (root + n/root)
	}
	return []float32{v[0] / root, v[1] / root}
}

func TestBuildIndexAppliesPenalty(t *testing.T) {
	blobs := []Blob{
		{Emoji: "🌞", Label: "sun with face", Group: "travel-places", Subgroup: "sky-weather", Text: "sun with face. bright, sun. travel-places, sky-weather."},
		{Emoji: "🥶", Label: "cold face", Group: "smileys-emotion", Subgroup: "face-unwell", Text: "cold face. cold, freezing. smileys-emotion, face-unwell."},
		{Emoji: "🔜", Label: "generic thing", Group: "objects", Subgroup: "other", Text: "generic thing. thing, stuff. objects, other."},
	}
	neutral := []string{"n1", "n2", "n3", "n4"} // fakeEmbedder maps every non-listed text to the same "generic" direction

	var buf bytes.Buffer
	err := BuildIndex(context.Background(), &buf, fakeEmbedder{}, "fake-model", blobs, neutral, 1.0, 0.85)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	idx, err := emojify.ReadIndex(&buf)
	if err != nil {
		t.Fatalf("ReadIndex: %v", err)
	}
	if idx.Count != 3 {
		t.Fatalf("Count = %d, want 3", idx.Count)
	}

	byEmoji := map[string]emojify.Metadata{}
	for _, m := range idx.Metadata {
		byEmoji[m.Emoji] = m
	}

	if p := byEmoji["🔜"].Penalty; p >= 1.0 {
		t.Errorf("generic entry penalty = %v, want < 1.0 (it sits close to every neutral sentence)", p)
	}
	if p := byEmoji["🌞"].Penalty; p != 1.0 {
		t.Errorf("sun penalty = %v, want 1.0 (distinct direction, not generic)", p)
	}
}

func TestBuildIndexRejectsEmptyNeutralCorpus(t *testing.T) {
	blobs := []Blob{
		{Emoji: "🌞", Label: "sun with face", Group: "travel-places", Subgroup: "sky-weather", Text: "sun with face. bright, sun. travel-places, sky-weather."},
	}

	var buf bytes.Buffer
	err := BuildIndex(context.Background(), &buf, fakeEmbedder{}, "fake-model", blobs, []string{}, 1.0, 0.85)
	if err == nil {
		t.Fatal("BuildIndex with empty neutralCorpus should error, got nil")
	}
	if err.Error() != "indexbuild: neutralCorpus must not be empty" {
		t.Errorf("BuildIndex error = %q, want %q", err.Error(), "indexbuild: neutralCorpus must not be empty")
	}
}
