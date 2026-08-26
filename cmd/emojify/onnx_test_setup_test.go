// cmd/emojify/onnx_test_setup_test.go
//go:build onnx

package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// Under the onnx build tag, this package's tests (root_test.go, index_test.go)
// exercise the real default embedder, which resolves data/model.onnx relative
// to the process's working directory. `go test` sets that to this package's
// own source directory (cmd/emojify/), which has no data/ of its own — so
// this init() points EMOJIFY_MODEL_PATH at the repo's actual data/ dir,
// computed relative to this file's own source location, before any test in
// the package runs. Only takes effect under -tags onnx; the !onnx build never
// needs a model path.
func init() {
	if os.Getenv("EMOJIFY_MODEL_PATH") != "" {
		return // respect an explicit override
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("emojify: runtime.Caller(0) failed while locating the repo's data/ dir for onnx tests")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	os.Setenv("EMOJIFY_MODEL_PATH", filepath.Join(repoRoot, "data", "model.onnx"))
}
