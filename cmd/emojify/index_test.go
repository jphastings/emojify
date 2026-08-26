// cmd/emojify/index_test.go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestInspectOutputIncludesBlobAndNeighbours(t *testing.T) {
	var buf bytes.Buffer
	if err := runIndexInspect(&buf, "🌞"); err != nil {
		t.Fatalf("runIndexInspect: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "sun with face") {
		t.Errorf("output missing label, got:\n%s", out)
	}
	if !strings.Contains(out, "nearest neighbours") {
		t.Errorf("output missing neighbours section, got:\n%s", out)
	}
}
