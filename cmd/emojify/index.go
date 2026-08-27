// cmd/emojify/index.go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/jphastings/emojify"
	"github.com/jphastings/emojify/internal/indexbuild"
)

var (
	indexIncludeFlags bool
	indexOutPath      string
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Build or inspect the emoji index",
}

var indexBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Fetch emojibase-data, embed it with this build's embedder, and write the index",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIndexBuild(cmd.Context(), indexOutPath, indexIncludeFlags)
	},
}

var indexInspectCmd = &cobra.Command{
	Use:   "inspect <emoji>",
	Short: "Show a candidate's blob text and nearest neighbours in the built index",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIndexInspect(cmd.OutOrStdout(), args[0])
	},
}

func init() {
	indexBuildCmd.Flags().BoolVar(&indexIncludeFlags, "include-flags", false, "include country/region flag emoji")
	indexBuildCmd.Flags().StringVar(&indexOutPath, "out", emojify.DefaultIndexPath, "output path for the built index")
	indexCmd.AddCommand(indexBuildCmd, indexInspectCmd)
	rootCmd.AddCommand(indexCmd)
}

// neutralCorpus is the fixed set of generic sentences used to compute
// build-time penalties (spec §5.2) — see the plan's Global Constraints for
// the measured mean/sigma this produced against the real model.
var neutralCorpus = []string{
	"I think that's a good idea.", "Let me know what you think.", "This is going to be interesting.",
	"I'm not sure what happens next.", "That's exactly what I expected.", "We should talk about this later.",
	"It's been a long week.", "I have a few thoughts on this.", "Let's see how it goes.",
	"That makes a lot of sense.", "I wonder what will happen.", "Things are moving along.",
	"I need to think about it.", "This is worth discussing.", "Let's take it one step at a time.",
	"There's a lot going on.", "I'll get back to you soon.", "That's an interesting point.",
	"We'll figure it out.", "It is what it is.", "Let's wait and see.", "I have some updates.",
	"This changes things a bit.", "That's a fair question.", "I appreciate you telling me.",
	"We can revisit this tomorrow.", "There's more to consider.", "Let's keep going.",
	"I'm still working on it.", "That's good to know.", "We made some progress today.",
	"I have mixed feelings about this.", "Let's check in again later.", "This is taking a while.",
	"I don't have an answer yet.", "We'll see how things develop.", "That's roughly what I thought.",
	"I need a bit more time.", "Let's move forward with this.", "It could go either way.",
}

const (
	penaltyThresholdSigma = 1.5
	penaltyMultiplier     = 0.85
	emojibaseVersion      = "17.0.0"

	// minIndexCandidates is a defensive floor on the number of blobs that
	// survive filtering before we build and write an index. Real candidate
	// counts are 1,536 by default and 1,806 with --include-flags; a
	// degenerate upstream response (an empty array, schema drift in
	// emojibase-data) would otherwise produce a structurally valid but
	// tiny/empty index, silently written as the real baked artifact with
	// only a stderr line as signal. 1000 leaves comfortable margin below
	// both real counts while still catching a meaningfully broken fetch.
	minIndexCandidates = 1000
)

// checkCandidateFloor rejects a candidate count too small to plausibly be a
// real emojibase-data fetch, rather than silently building/writing a
// structurally valid but tiny/degenerate index.
func checkCandidateFloor(n int) error {
	if n < minIndexCandidates {
		return fmt.Errorf("index build: only %d candidates survived filtering, want at least %d — refusing to build/write an index this small (degenerate upstream response? schema drift in emojibase-data?)", n, minIndexCandidates)
	}
	return nil
}

func runIndexBuild(ctx context.Context, outPath string, includeFlags bool) error {
	entries, names, err := indexbuild.FetchEmojibaseData(ctx, emojibaseVersion)
	if err != nil {
		return fmt.Errorf("fetching emojibase data: %w", err)
	}
	blobs := indexbuild.BuildBlobs(entries, names, includeFlags)
	fmt.Fprintf(os.Stderr, "index build: %d candidates (include-flags=%v)\n", len(blobs), includeFlags)
	if err := checkCandidateFloor(len(blobs)); err != nil {
		return err
	}

	emb, err := emojify.NewDefaultEmbedder("")
	if err != nil {
		return fmt.Errorf("constructing embedder: %w", err)
	}
	if c, ok := emb.(interface{ Close() error }); ok {
		defer c.Close()
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return indexbuild.BuildIndex(ctx, f, emb, indexModelID(emb), blobs, neutralCorpus, penaltyThresholdSigma, penaltyMultiplier)
}

func indexModelID(emb emojify.Embedder) string {
	// Distinguish the two build tags' output without importing build-tag
	// files here: dims alone is sufficient since the two embedders never
	// coexist in one binary.
	if emb.Dims() == 384 {
		return "onnx-all-minilm-l6-v2"
	}
	return "static-glove50"
}

func runIndexInspect(w io.Writer, emoji string) error {
	// Reads the already-built index directly — no Embedder needed, since
	// inspecting an existing entry never embeds new text.
	idx, err := emojify.ReadIndex(bytes.NewReader(emojify.DefaultIndexBytes()))
	if err != nil {
		return err
	}

	var target *emojify.Metadata
	var targetVec []float32
	for i, meta := range idx.Metadata {
		if meta.Emoji == emoji {
			target = &idx.Metadata[i]
			targetVec = idx.Vectors[i*idx.Dims : (i+1)*idx.Dims]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("emoji %q not found in index", emoji)
	}

	fmt.Fprintf(w, "%s %s\n", target.Emoji, target.Label)
	fmt.Fprintf(w, "  group: %s, %s\n", target.Group, target.Subgroup)
	fmt.Fprintf(w, "  penalty: %.3f\n", target.Penalty)

	type sim struct {
		meta  emojify.Metadata
		score float32
	}
	sims := make([]sim, idx.Count)
	for i := 0; i < idx.Count; i++ {
		row := idx.Vectors[i*idx.Dims : (i+1)*idx.Dims]
		var s float32
		for d := range row {
			s += row[d] * targetVec[d]
		}
		sims[i] = sim{meta: idx.Metadata[i], score: s}
	}
	sort.Slice(sims, func(a, b int) bool { return sims[a].score > sims[b].score })

	fmt.Fprintln(w, "  nearest neighbours:")
	for i := 1; i <= 10 && i < len(sims); i++ { // skip index 0: the emoji itself, similarity 1.0
		fmt.Fprintf(w, "    %.3f  %s  %s\n", sims[i].score, sims[i].meta.Emoji, sims[i].meta.Label)
	}
	return nil
}
