// internal/indexbuild/build.go
package indexbuild

import (
	"context"
	"fmt"
	"io"
	"math"

	"github.com/jphastings/emojify"
)

// BuildIndex embeds every blob's text with emb, scores each against
// neutralCorpus to compute a generic-emoji penalty (spec §5.2), and writes
// the resulting index via emojify.WriteIndex.
//
// An entry's mean cosine similarity to the neutral corpus, standardised
// against the distribution of that mean across all entries, drives the
// penalty: entries more than penaltyThresholdSigma standard deviations above
// the mean get penaltyMultiplier applied; everyone else keeps 1.0.
func BuildIndex(
	ctx context.Context,
	w io.Writer,
	emb emojify.Embedder,
	modelID string,
	blobs []Blob,
	neutralCorpus []string,
	penaltyThresholdSigma float64,
	penaltyMultiplier float32,
) error {
	if len(neutralCorpus) == 0 {
		return fmt.Errorf("indexbuild: neutralCorpus must not be empty")
	}

	texts := make([]string, len(blobs))
	for i, b := range blobs {
		texts[i] = b.Text
	}
	vectors, err := emb.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("indexbuild: embedding blobs: %w", err)
	}

	neutralVecs, err := emb.Embed(ctx, neutralCorpus)
	if err != nil {
		return fmt.Errorf("indexbuild: embedding neutral corpus: %w", err)
	}

	means := make([]float64, len(vectors))
	var sum, sumSq float64
	for i, v := range vectors {
		var s float32
		for _, n := range neutralVecs {
			s += dotProduct(v, n)
		}
		mean := float64(s) / float64(len(neutralVecs))
		means[i] = mean
		sum += mean
		sumSq += mean * mean
	}
	n := float64(len(means))
	mu := sum / n
	variance := sumSq/n - mu*mu
	// Clamp to 0 to prevent NaN from floating-point cancellation
	if variance < 0 {
		variance = 0
	}
	sigma := math.Sqrt(variance)
	threshold := mu + penaltyThresholdSigma*sigma

	metas := make([]emojify.Metadata, len(blobs))
	for i, b := range blobs {
		penalty := float32(1.0)
		if means[i] > threshold {
			penalty = penaltyMultiplier
		}
		metas[i] = emojify.Metadata{Emoji: b.Emoji, Label: b.Label, Group: b.Group, Subgroup: b.Subgroup, Penalty: penalty}
	}

	return emojify.WriteIndex(w, modelID, emb.Dims(), vectors, metas)
}

func dotProduct(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
