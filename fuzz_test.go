// fuzz_test.go
package emojify

import (
	"context"
	"testing"
)

// FuzzSuggestInputHandling exercises the input path with the edge cases
// spec §9 calls out: emoji-only input, zero-width joiners, RTL text, and 300
// graphemes of combining characters. It asserts only that Suggest never
// panics and always returns either a result or an error — never both nil.
func FuzzSuggestInputHandling(f *testing.F) {
	seeds := []string{
		"🔥🔥🔥",
		"👨‍👩‍👧‍👦", // ZWJ family sequence
		"مرحبا بك في هذا اليوم الجميل", // Arabic, RTL
		"שלום לכם ביום נפלא זה",        // Hebrew, RTL
		string(makeCombiningRunes(300)),
		"",
		" ",
		"a\x00b", // embedded NUL
	}
	for _, s := range seeds {
		f.Add(s)
	}

	m, err := New()
	if err != nil {
		f.Fatalf("New: %v", err)
	}
	f.Cleanup(func() { m.Close() })

	f.Fuzz(func(t *testing.T, text string) {
		suggestions, err := m.Suggest(context.Background(), text, 3)
		if err != nil {
			return // an error is an acceptable outcome (e.g. too many runes)
		}
		if suggestions == nil {
			t.Errorf("Suggest(%q) returned nil, nil (want a slice, possibly empty, when err is nil)", text)
		}
	})
}

func makeCombiningRunes(n int) []rune {
	out := make([]rune, n)
	for i := range out {
		out[i] = 'e'
		if i%2 == 1 {
			out[i] = '́' // combining acute accent
		}
	}
	return out
}
