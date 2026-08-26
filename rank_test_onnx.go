//go:build onnx

package emojify

// wantPassRate is the minimum fraction of non-informational golden rows this
// build's embedder must place correctly.
//
// The onnx build (production embedder) is expected to achieve meaningfully
// better results than the static fallback (see rank_test_static.go).
const wantPassRate = 0.8
