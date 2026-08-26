// cmd/emojify/root_test.go
package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunSuggestPlainOutput(t *testing.T) {
	var out bytes.Buffer
	err := runSuggest(&out, "such a beautiful sunny afternoon", suggestOptions{limit: 3})
	if err != nil {
		t.Fatalf("runSuggest: %v", err)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatal("expected non-empty plain output")
	}
}

func TestRunSuggestJSONOutput(t *testing.T) {
	var out bytes.Buffer
	err := runSuggest(&out, "such a beautiful sunny afternoon", suggestOptions{limit: 3, json: true})
	if err != nil {
		t.Fatalf("runSuggest: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "[") {
		t.Fatalf("expected JSON array output, got: %s", out.String())
	}
}

func TestRunSuggestNoMatchSentinel(t *testing.T) {
	var out bytes.Buffer
	err := runSuggest(&out, "such a beautiful sunny afternoon", suggestOptions{limit: 3, minScore: 0.999})
	if !errors.Is(err, errNoMatch) {
		t.Fatalf("err = %v, want errNoMatch", err)
	}
}
