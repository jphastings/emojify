// rank.go
package emojify

import (
	"context"
	"fmt"
	"sort"
)

const (
	rankTopK      = 20   // candidate pool size before MMR diversification
	rankMMRLambda = 0.75 // relevance vs. diversity trade-off
)

// Matcher is a loaded index + embedder pair. Safe for concurrent use.
type Matcher struct {
	embedder Embedder
	index    *Index
	minScore float32
	maxRunes int
}

// Suggest returns up to limit emoji whose meaning best matches text. Input
// longer than MaxRunes is an error, not a silent truncation.
func (m *Matcher) Suggest(ctx context.Context, text string, limit int) ([]Suggestion, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("emojify: limit must be positive, got %d", limit)
	}
	runeCount := 0
	for range text {
		runeCount++
	}
	if runeCount > m.maxRunes {
		return nil, fmt.Errorf("emojify: input is %d runes, exceeds MaxRunes %d", runeCount, m.maxRunes)
	}

	vecs, err := m.embedder.Embed(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("emojify: embedding input: %w", err)
	}
	query := vecs[0]

	type scored struct {
		idx   int
		score float32
	}
	dims := m.index.Dims
	all := make([]scored, m.index.Count)
	for i := 0; i < m.index.Count; i++ {
		row := m.index.Vectors[i*dims : (i+1)*dims]
		all[i] = scored{idx: i, score: dot(query, row) * m.index.Metadata[i].Penalty}
	}
	sort.Slice(all, func(a, b int) bool { return all[a].score > all[b].score })

	topK := rankTopK
	if topK > len(all) {
		topK = len(all)
	}
	pool := all[:topK]

	selected := make([]scored, 0, limit)
	chosen := make(map[int]bool, limit)
	for len(selected) < limit && len(selected) < len(pool) {
		bestIdx := -1
		bestPos := -1
		var bestMMR float32 = -1 << 30
		for pos, cand := range pool {
			if chosen[cand.idx] {
				continue
			}
			var maxSim float32
			for _, s := range selected {
				sim := dot(m.index.Vectors[cand.idx*dims:(cand.idx+1)*dims], m.index.Vectors[s.idx*dims:(s.idx+1)*dims])
				if sim > maxSim {
					maxSim = sim
				}
			}
			mmr := rankMMRLambda*cand.score - (1-rankMMRLambda)*maxSim
			if mmr > bestMMR {
				bestMMR = mmr
				bestIdx = cand.idx
				bestPos = pos
			}
		}
		if bestIdx == -1 {
			break
		}
		chosen[bestIdx] = true
		selected = append(selected, pool[bestPos])
	}

	out := make([]Suggestion, 0, len(selected))
	for _, s := range selected {
		if s.score < m.minScore {
			continue
		}
		meta := m.index.Metadata[s.idx]
		out = append(out, Suggestion{Emoji: meta.Emoji, Name: meta.Label, Score: s.score})
	}
	return out, nil
}

// Close releases resources held by the embedder (e.g. an ONNX Runtime session).
func (m *Matcher) Close() error {
	if c, ok := m.embedder.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
