// internal/wordpiece/wordpiece_test.go
package wordpiece

import (
	"strings"
	"testing"
)

func fixtureVocab() map[string]int64 {
	tokens := []string{"[PAD]", "[UNK]", "[CLS]", "[SEP]", "play", "##ing", "sun", "bright", "cold"}
	vocab := make(map[string]int64, len(tokens))
	for i, t := range tokens {
		vocab[t] = int64(i)
	}
	return vocab
}

func TestEncodeAddsSpecialTokens(t *testing.T) {
	tok := New(fixtureVocab())
	ids, mask := tok.Encode("sun", 10)

	vocab := fixtureVocab()
	if ids[0] != vocab["[CLS]"] {
		t.Errorf("ids[0] = %d, want [CLS] (%d)", ids[0], vocab["[CLS]"])
	}
	if ids[len(ids)-1] != vocab["[SEP]"] {
		t.Errorf("last id = %d, want [SEP] (%d)", ids[len(ids)-1], vocab["[SEP]"])
	}
	if len(ids) != 3 { // [CLS] sun [SEP]
		t.Fatalf("len(ids) = %d, want 3: %v", len(ids), ids)
	}
	for _, m := range mask {
		if m != 1 {
			t.Errorf("mask = %v, want all 1s for unpadded output", mask)
		}
	}
}

func TestEncodeSplitsUnknownWordIntoSubwords(t *testing.T) {
	tok := New(fixtureVocab())
	vocab := fixtureVocab()
	ids, _ := tok.Encode("playing", 10)
	// [CLS] play ##ing [SEP]
	want := []int64{vocab["[CLS]"], vocab["play"], vocab["##ing"], vocab["[SEP]"]}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %d, want %d", i, ids[i], want[i])
		}
	}
}

func TestEncodeUnknownWordFallsBackToUNK(t *testing.T) {
	tok := New(fixtureVocab())
	vocab := fixtureVocab()
	ids, _ := tok.Encode("xyzzy", 10)
	if ids[1] != vocab["[UNK]"] {
		t.Errorf("ids[1] = %d, want [UNK] (%d): %v", ids[1], vocab["[UNK]"], ids)
	}
}

func TestEncodeTruncatesToMaxTokens(t *testing.T) {
	tok := New(fixtureVocab())
	ids, mask := tok.Encode("sun bright cold sun bright cold sun bright cold", 5)
	if len(ids) != 5 {
		t.Fatalf("len(ids) = %d, want 5 (truncated)", len(ids))
	}
	if len(mask) != 5 {
		t.Fatalf("len(mask) = %d, want 5", len(mask))
	}
	vocab := fixtureVocab()
	if ids[4] != vocab["[SEP]"] {
		t.Errorf("last id after truncation = %d, want [SEP] (%d)", ids[4], vocab["[SEP]"])
	}
}

func TestLoadVocab(t *testing.T) {
	r := strings.NewReader("[PAD]\n[UNK]\n[CLS]\n[SEP]\nsun\n")
	vocab, err := LoadVocab(r)
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if vocab["sun"] != 4 {
		t.Errorf("vocab[sun] = %d, want 4", vocab["sun"])
	}
}
