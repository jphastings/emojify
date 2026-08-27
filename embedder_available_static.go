// embedder_available_static.go
//go:build !onnx

package emojify

import "fmt"

// ortAvailable is always false when this binary was built without -tags
// onnx: there is no ONNX Runtime integration compiled in to dlopen at all.
func ortAvailable() bool { return false }

// newONNXEmbedderOrErr is the !onnx half of the shim embedder_select.go
// calls without needing a build tag of its own; see embedder_onnx.go for the
// onnx half, which actually constructs the embedder.
func newONNXEmbedderOrErr(modelPath string) (Embedder, error) {
	return nil, fmt.Errorf("emojify: ONNX Runtime support isn't compiled into this binary — rebuild with -tags onnx")
}
