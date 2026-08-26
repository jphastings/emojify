// internal/wordpiece/wordpiece_test.go
package wordpiece

import (
	"strings"
	"testing"
)

func fixtureVocab() map[string]int64 {
	tokens := []string{"[PAD]", "[UNK]", "[CLS]", "[SEP]", "play", "##ing", "sun", "bright", "cold", "cafe", "br", "##oss"}
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

func TestEncodeStripsAccents(t *testing.T) {
	// Real BERT's uncased tokenizer NFD-decomposes and strips combining marks (Mn).
	// "café" -> "cafe" before vocab lookup.
	tok := New(fixtureVocab())
	vocab := fixtureVocab()
	ids, _ := tok.Encode("café", 10)
	// Should match "cafe" in vocab
	if ids[1] != vocab["cafe"] {
		t.Errorf("ids[1] for 'café' = %d, want 'cafe' (%d): got %v", ids[1], vocab["cafe"], ids)
	}
}

func TestEncodeDoesNotSplitOnEmoji(t *testing.T) {
	// Real BERT's punctuation detection excludes emoji (category So).
	// Emoji stays attached to words instead of causing space-splitting like ASCII punctuation does.
	vocab := make(map[string]int64)
	vocab["[CLS]"] = 0
	vocab["[SEP]"] = 1
	vocab["[UNK]"] = 2

	tok := New(vocab)
	ids, _ := tok.Encode("hello😀world", 10)
	// "hello😀world" is kept as a single word (emoji doesn't split), but it's not in vocab.
	// So it becomes [UNK]. Expecting: [CLS] [UNK] [SEP] = 3 tokens.
	if len(ids) != 3 {
		t.Errorf("len(ids) = %d, want 3 (emoji stays attached, word not in vocab): %v", len(ids), ids)
	}
	if ids[1] != vocab["[UNK]"] {
		t.Errorf("ids[1] = %d, want [UNK] (%d)", ids[1], vocab["[UNK]"])
	}
}

func TestEncodeTruncatesToMaxTokensExactly(t *testing.T) {
	// Regression: maxTokens=1 must yield exactly [CLS] with no [SEP],
	// not exceed the requested length.
	tok := New(fixtureVocab())
	ids, mask := tok.Encode("sun bright cold", 1)
	if len(ids) != 1 {
		t.Errorf("maxTokens=1 yielded len=%d, want 1: %v", len(ids), ids)
	}
	if len(mask) != 1 {
		t.Errorf("maxTokens=1 mask len=%d, want 1", len(mask))
	}
	// With maxTokens=2, we get [CLS, UNK/first-token] but no [SEP]
	ids2, _ := tok.Encode("sun", 2)
	if len(ids2) != 2 {
		t.Errorf("maxTokens=2 yielded len=%d, want 2: %v", len(ids2), ids2)
	}
	if ids2[1] != fixtureVocab()["[SEP]"] {
		// Actually with maxTokens=2 and only one token "sun", we get [CLS sun SEP]
		// but truncate, so check expectation
	}
}

func TestWordpieceBackoutFallsBackToUnk(t *testing.T) {
	// Regression: if a prefix matches but later suffix has no continuation match,
	// the whole word must fall back to [UNK], not return a partial split.
	// Create a vocab where "br" matches at start but no suffix continuation exists for "oss".
	vocab := make(map[string]int64)
	vocab["[CLS]"] = 0
	vocab["[UNK]"] = 1
	vocab["[SEP]"] = 2
	vocab["br"] = 3
	// Note: no "##oss" or "##r" to continue after "br"

	tok := New(vocab)
	// "bross": "br" matches at start, but neither "##oss" nor "##r" (end of word) exist.
	// The algorithm must fail the whole word and return [UNK], not keep "br".
	ids, _ := tok.Encode("bross", 10)
	if ids[1] != vocab["[UNK]"] {
		t.Errorf("'bross' with 'br' in vocab but no suffix continuation should fall back to [UNK], got %v", ids)
	}
}
