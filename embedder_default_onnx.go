// embedder_default_onnx.go
//go:build onnx

package emojify

// DefaultIndexPath is the path `index build` writes to by default under this
// build tag — the index built for the embedder this build prefers (onnx).
const DefaultIndexPath = "data/index.bin"

// preferredIndexData is the tag-gated half DefaultIndexBytes (emojify.go)
// calls once it already knows ONNX Runtime is loadable — onnxIndexData
// itself is only compiled in under this tag.
func preferredIndexData() []byte { return onnxIndexData }
