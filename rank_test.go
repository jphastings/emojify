// rank_test.go
package emojify

import (
	"context"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// stripVariationSelectors removes U+FE0F (VS16, emoji presentation) and
// U+FE0E (VS15, text presentation) so golden.yaml's hand-typed emoji compare
// equal to emojibase-data's canonical strings regardless of which form
// either side happens to use.
func stripVariationSelectors(s string) string {
	return strings.NewReplacer("️", "", "︎", "").Replace(s)
}

type goldenRow struct {
	Phrase        string   `yaml:"phrase"`
	ExpectAnyOf   []string `yaml:"expect_any_of"`
	WithinTop     int      `yaml:"within_top"`
	Informational bool     `yaml:"informational"`
}

func loadGolden(t *testing.T) []goldenRow {
	t.Helper()
	data, err := os.ReadFile("testdata/golden.yaml")
	if err != nil {
		t.Fatalf("reading golden.yaml: %v", err)
	}
	var rows []goldenRow
	if err := yaml.Unmarshal(data, &rows); err != nil {
		t.Fatalf("parsing golden.yaml: %v", err)
	}
	return rows
}


func TestGoldenTable(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	rows := loadGolden(t)
	var asserted, passed int
	for _, row := range rows {
		suggestions, err := m.Suggest(context.Background(), row.Phrase, row.WithinTop)
		if err != nil {
			t.Fatalf("Suggest(%q): %v", row.Phrase, err)
		}
		got := make([]string, len(suggestions))
		for i, s := range suggestions {
			got[i] = s.Emoji
		}

		hit := false
		for _, want := range row.ExpectAnyOf {
			for _, g := range got {
				if stripVariationSelectors(g) == stripVariationSelectors(want) {
					hit = true
				}
			}
		}

		if row.Informational {
			t.Logf("[informational] %q -> %v (expected one of %v)", row.Phrase, got, row.ExpectAnyOf)
			continue
		}
		asserted++
		if hit {
			passed++
		} else {
			t.Logf("MISS %q -> %v (expected one of %v)", row.Phrase, got, row.ExpectAnyOf)
		}
	}

	rate := float64(passed) / float64(asserted)
	t.Logf("golden pass rate: %d/%d = %.0f%%", passed, asserted, rate*100)
	if rate < wantPassRate {
		t.Errorf("golden pass rate %.0f%% below floor %.0f%%", rate*100, wantPassRate*100)
	}
}

func TestSuggestRejectsOverlongInput(t *testing.T) {
	m, err := New(WithMaxRunes(5))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()
	if _, err := m.Suggest(context.Background(), "way too long", 3); err == nil {
		t.Fatal("expected an error for input exceeding MaxRunes, got nil")
	}
}

func TestSuggestDiversifies(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()
	suggestions, err := m.Suggest(context.Background(), "such a beautiful sunny afternoon", 5)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	seen := map[string]bool{}
	for _, s := range suggestions {
		if seen[s.Emoji] {
			t.Errorf("duplicate emoji %q in diversified results %v", s.Emoji, suggestions)
		}
		seen[s.Emoji] = true
	}
}
