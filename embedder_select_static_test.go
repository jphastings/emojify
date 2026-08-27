// embedder_select_static_test.go
//go:build !onnx

package emojify

import "testing"

func TestEmojifyEmbedderOnnxErrorsWithoutOnnxTag(t *testing.T) {
	t.Setenv("EMOJIFY_EMBEDDER", "onnx")
	if _, err := NewDefaultEmbedder(""); err == nil {
		t.Fatal("EMOJIFY_EMBEDDER=onnx should fail in a binary built without -tags onnx, not silently fall back")
	}
}
