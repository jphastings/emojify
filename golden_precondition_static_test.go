//go:build !onnx

package emojify

import "testing"

// checkGoldenPreconditions has nothing to verify in a build with only the
// static embedder compiled in — it's the one wantPassRate already assumes.
func checkGoldenPreconditions(t *testing.T) { t.Helper() }
