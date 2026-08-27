// index_bench_test.go
package emojify

import (
	"bytes"
	"testing"
)

// BenchmarkReadIndex measures ReadIndex's decode cost against this build's
// real embedded default index (384-dim/1536-entry for onnx, 50-dim for
// static), run at every process start including every one-shot CLI
// invocation. Guards the fix that replaced one binary.Read call per int8
// vector component with a single io.ReadFull per vector block decoded in
// memory.
func BenchmarkReadIndex(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ReadIndex(bytes.NewReader(DefaultIndexBytes())); err != nil {
			b.Fatalf("ReadIndex: %v", err)
		}
	}
}
