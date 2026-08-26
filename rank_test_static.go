//go:build !onnx

package emojify

// wantPassRate is the minimum fraction of non-informational golden rows this
// build's embedder must place correctly.
//
// The static (GloVe-averaging) embedder is expected to do meaningfully worse
// than onnx (see rank_test_onnx.go) — that gap is the point of having both
// (spec §10 phase 3). Measured at 20% against the real static embedder.
const wantPassRate = 0.2
