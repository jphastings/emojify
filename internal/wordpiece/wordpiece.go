// internal/wordpiece/wordpiece.go
package wordpiece

import (
	"bufio"
	"io"
	"strings"
	"unicode"
)

const (
	maxInputCharsPerWord = 100
	ClsToken             = "[CLS]"
	SepToken             = "[SEP]"
	UnkToken             = "[UNK]"
	PadToken             = "[PAD]"
)

// Tokenizer implements BERT-style basic tokenization + WordPiece subword
// splitting: the "uncased" recipe all-MiniLM-L6-v2 was trained with.
type Tokenizer struct {
	vocab map[string]int64
}

func New(vocab map[string]int64) *Tokenizer { return &Tokenizer{vocab: vocab} }

// LoadVocab reads a BERT vocab.txt file: one token per line, its line number is its id.
func LoadVocab(r io.Reader) (map[string]int64, error) {
	vocab := make(map[string]int64)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var id int64
	for scanner.Scan() {
		vocab[scanner.Text()] = id
		id++
	}
	return vocab, scanner.Err()
}

// Encode returns input ids and an attention mask for text, with [CLS]/[SEP]
// added, truncated to maxTokens (including the two special tokens). The
// returned slices are exactly len(ids)==len(mask), unpadded — callers that
// need a fixed-width tensor pad it themselves.
func (t *Tokenizer) Encode(text string, maxTokens int) (ids []int64, mask []int64) {
	ids = append(ids, t.vocab[ClsToken])
	for _, word := range basicTokenize(text) {
		for _, piece := range t.wordpieceSplit(word) {
			if len(ids) >= maxTokens-1 {
				break
			}
			id, ok := t.vocab[piece]
			if !ok {
				id = t.vocab[UnkToken]
			}
			ids = append(ids, id)
		}
	}
	ids = append(ids, t.vocab[SepToken])

	mask = make([]int64, len(ids))
	for i := range mask {
		mask[i] = 1
	}
	return ids, mask
}

func basicTokenize(text string) []string {
	text = strings.ToLower(text)
	var b strings.Builder
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		case unicode.IsPunct(r), unicode.IsSymbol(r):
			b.WriteRune(' ')
			b.WriteRune(r)
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Fields(b.String())
}

func (t *Tokenizer) wordpieceSplit(word string) []string {
	runes := []rune(word)
	if len(runes) > maxInputCharsPerWord {
		return []string{UnkToken}
	}

	var pieces []string
	start := 0
	for start < len(runes) {
		end := len(runes)
		matched := ""
		for end > start {
			piece := string(runes[start:end])
			if start > 0 {
				piece = "##" + piece
			}
			if _, ok := t.vocab[piece]; ok {
				matched = piece
				break
			}
			end--
		}
		if matched == "" {
			return []string{UnkToken}
		}
		pieces = append(pieces, matched)
		start = end
	}
	return pieces
}
