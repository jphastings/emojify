// emojify.go
package emojify

import (
	"bytes"
	"fmt"
	"io"
)

// Suggestion is one candidate emoji match.
type Suggestion struct {
	Emoji string  `json:"emoji"`
	Name  string  `json:"name"`
	Score float32 `json:"score"` // cosine similarity, 0..1
}

type config struct {
	embedder Embedder
	index    *Index
	indexErr error
	minScore float32
	maxRunes int
}

// Option configures a Matcher built by New.
type Option func(*config)

// WithEmbedder swaps the embedder New would otherwise construct by default.
func WithEmbedder(e Embedder) Option { return func(c *config) { c.embedder = e } }

// WithIndex swaps the embedded index New would otherwise use by default.
func WithIndex(r io.Reader) Option {
	return func(c *config) {
		idx, err := ReadIndex(r)
		if err != nil {
			c.indexErr = err
			return
		}
		c.index = idx
	}
}

// WithMinScore drops matches below this cosine similarity rather than padding the result.
func WithMinScore(s float32) Option { return func(c *config) { c.minScore = s } }

// WithMaxRunes overrides the default 300-rune input limit.
func WithMaxRunes(n int) Option { return func(c *config) { c.maxRunes = n } }

// New loads the embedded index and embedder. Cheap to hold, expensive to
// create — construct once, share across goroutines.
func New(opts ...Option) (*Matcher, error) {
	cfg := config{maxRunes: 300}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.indexErr != nil {
		return nil, fmt.Errorf("emojify: loading index from WithIndex: %w", cfg.indexErr)
	}

	if cfg.embedder == nil {
		// The onnx build's NewDefaultEmbedder resolves its own model path
		// (explicit arg -> EMOJIFY_MODEL_PATH -> DefaultModelPath, see Task
		// 9); New() has no dedicated model-path Option, matching the spec's
		// §3 Option list exactly — a caller needing a custom path builds its
		// own Embedder via NewDefaultEmbedder(path) and passes WithEmbedder.
		emb, err := NewDefaultEmbedder("")
		if err != nil {
			return nil, fmt.Errorf("emojify: default embedder: %w", err)
		}
		cfg.embedder = emb
	}
	if cfg.index == nil {
		idx, err := ReadIndex(bytes.NewReader(defaultIndexData))
		if err != nil {
			return nil, fmt.Errorf("emojify: default index: %w", err)
		}
		cfg.index = idx
	}
	if cfg.index.Dims != cfg.embedder.Dims() {
		return nil, fmt.Errorf("emojify: index is %d-dim (model %q) but embedder produces %d-dim vectors — mismatched index/embedder pair",
			cfg.index.Dims, cfg.index.ModelID, cfg.embedder.Dims())
	}

	return &Matcher{embedder: cfg.embedder, index: cfg.index, minScore: cfg.minScore, maxRunes: cfg.maxRunes}, nil
}

// DefaultIndexBytes returns the raw bytes of this build's embedded default
// index (data/index.bin or data/index_static.bin depending on build tag).
// Exposed for `emojify index inspect`; not part of the Suggest/New surface.
func DefaultIndexBytes() []byte { return defaultIndexData }
