//go:build onnx

package emojify

import "testing"

// checkGoldenPreconditions fails fast when wantPassRate's assumptions don't
// hold. The onnx tag is additive, so this binary silently falls back to the
// static embedder when ONNX Runtime can't be loaded — and would then miss the
// onnx floor by a mile, looking like a quality regression rather than a
// missing dependency.
func checkGoldenPreconditions(t *testing.T) {
	t.Helper()
	if !ortAvailable() {
		t.Fatalf("this build's golden floor assumes the ONNX embedder, but ONNX Runtime isn't loadable — install onnxruntime (>= 1.29) or set EMOJIFY_ORT_LIBRARY_PATH")
	}
}
