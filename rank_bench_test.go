// rank_bench_test.go
package emojify

import (
	"context"
	"testing"
)

// BenchmarkSuggest measures end-to-end embedding + ranking latency on this
// machine. Per the plan's Global Constraints, there's no physical target SBC
// reachable here — see the README (Task 15) for the same command run on real
// hardware.
func BenchmarkSuggest(b *testing.B) {
	m, err := New()
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer m.Close()

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.Suggest(ctx, "such a beautiful sunny afternoon", 3); err != nil {
			b.Fatalf("Suggest: %v", err)
		}
	}
}
