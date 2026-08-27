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

// TestCheckCandidateFloor guards against a degenerate upstream response (an
// empty array, schema drift in emojibase-data) silently producing a
// structurally valid but tiny/empty index that gets committed as the real
// baked artifact.
func TestCheckCandidateFloor(t *testing.T) {
	if err := checkCandidateFloor(0); err == nil {
		t.Error("checkCandidateFloor(0) = nil error, want an error (empty candidate set)")
	}
	if err := checkCandidateFloor(minIndexCandidates - 1); err == nil {
		t.Errorf("checkCandidateFloor(%d) = nil error, want an error (below floor)", minIndexCandidates-1)
	}
	// Real candidate counts are 1,536 by default and 1,806 with
	// --include-flags (see the plan); both must clear the floor with margin.
	for _, n := range []int{minIndexCandidates, 1536, 1806} {
		if err := checkCandidateFloor(n); err != nil {
			t.Errorf("checkCandidateFloor(%d) = %v, want nil", n, err)
		}
	}
}
