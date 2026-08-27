// embedder_default_static.go
//go:build !onnx

package emojify

// DefaultIndexPath is the path `index build` writes to by default under this
// build tag — the index built for the embedder this build prefers (static,
// the only embedder a !onnx build has at all).
const DefaultIndexPath = "data/index_static.bin"

// preferredIndexData mirrors embedder_default_onnx.go's function of the same
// name; in a !onnx build there's only ever the static index to prefer.
func preferredIndexData() []byte { return staticIndexData }
