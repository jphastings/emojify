// onnxfallback_test.go
//go:build onnx

// Package onnxfallbacktest checks that New() falls back to the static
// embedder when ONNX Runtime can't be loaded — in its own test binary,
// deliberately. ensureORTInitialized (embedder_onnx.go) uses a sync.Once, so
// whichever test in a process first touches ORT fixes the outcome for that
// process's lifetime; sharing a binary with the package's other onnx tests
// (which expect ORT to succeed) would make this check order-dependent and
// flaky. A lone test in its own package guarantees this is that first touch.
package onnxfallbacktest

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jphastings/emojify"
)

func TestFallsBackToStaticWhenORTLibraryMissing(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed while locating the repo's data/ dir")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	t.Setenv("EMOJIFY_MODEL_PATH", filepath.Join(repoRoot, "data", "model.onnx"))
	t.Setenv("EMOJIFY_ORT_LIBRARY_PATH", filepath.Join(repoRoot, "does-not-exist.dylib"))

	m, err := emojify.New()
	if err != nil {
		t.Fatalf("New() with an unloadable ORT library should fall back to the static embedder, not error: %v", err)
	}
	defer m.Close()

	suggestions, err := m.Suggest(context.Background(), "let's get pizza", 1)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(suggestions))
	}
}
