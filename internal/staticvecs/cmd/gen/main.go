// internal/staticvecs/cmd/gen/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jphastings/emojify/internal/indexbuild"
	"github.com/jphastings/emojify/internal/staticvecs"
)

const topN = 30000

var wordPattern = regexp.MustCompile(`[a-z]+`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	entries, names, err := indexbuild.FetchEmojibaseData(ctx, "17.0.0")
	if err != nil {
		return fmt.Errorf("fetching emojibase data for vocabulary: %w", err)
	}
	blobs := indexbuild.BuildBlobs(entries, names, true) // include flags too: bigger guaranteed vocabulary is harmless

	mustKeep := make(map[string]bool)
	for _, b := range blobs {
		for _, w := range wordPattern.FindAllString(strings.ToLower(b.Text), -1) {
			mustKeep[w] = true
		}
	}

	rc, err := staticvecs.FetchSourceVectors(staticvecs.SourceURL)
	if err != nil {
		return fmt.Errorf("fetching source vectors: %w", err)
	}
	defer rc.Close()

	words, centroid, dims, err := staticvecs.Trim(rc, topN, mustKeep)
	if err != nil {
		return fmt.Errorf("trimming vectors: %w", err)
	}
	fmt.Fprintf(os.Stderr, "gen: kept %d words (dims=%d)\n", len(words), dims)

	f, err := os.Create("data/staticvecs.bin")
	if err != nil {
		return err
	}
	defer f.Close()
	return staticvecs.Write(f, dims, words, centroid)
}
