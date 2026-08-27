// embedder_onnx.go
//go:build onnx

package emojify

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/jphastings/emojify/internal/wordpiece"
)

//go:embed data/index.bin
var onnxIndexData []byte

// embeddedModel and embeddedVocab let an onnx-tagged binary run the real
// model with no data/ directory on disk at all — see newONNXEmbedder's
// resolution order. They make -tags onnx strictly additive to the static
// build rather than requiring a companion data/ directory to be deployed
// alongside it.
//
//go:embed data/model.onnx
var embeddedModel []byte

//go:embed data/vocab.txt
var embeddedVocab []byte

const (
	onnxDims         = 384
	onnxMaxTokens    = 256
	DefaultModelPath = "data/model.onnx"
)

var (
	ortInitOnce sync.Once
	ortInitErr  error
)

func ensureORTInitialized() error {
	ortInitOnce.Do(func() {
		ort.SetSharedLibraryPath(sharedLibraryPath())
		ortInitErr = ort.InitializeEnvironment()
	})
	return ortInitErr
}

// ortAvailable reports whether ONNX Runtime can actually be dlopen'd on this
// machine — the runtime signal newONNXEmbedderOrErr and NewDefaultEmbedder
// use to decide between the real model and the static fallback.
func ortAvailable() bool { return ensureORTInitialized() == nil }

func sharedLibraryPath() string {
	if p := os.Getenv("EMOJIFY_ORT_LIBRARY_PATH"); p != "" {
		return p
	}
	switch runtime.GOOS {
	case "darwin":
		return "/opt/homebrew/lib/libonnxruntime.dylib"
	case "linux":
		return "/usr/lib/libonnxruntime.so"
	default:
		return "onnxruntime.dll"
	}
}

type onnxEmbedder struct {
	mu        sync.Mutex
	session   *ort.AdvancedSession
	inputIDs  *ort.Tensor[int64]
	attnMask  *ort.Tensor[int64]
	typeIDs   *ort.Tensor[int64]
	output    *ort.Tensor[float32]
	tokenizer *wordpiece.Tokenizer
}

// newONNXEmbedderOrErr is the onnx-tagged half of the shim
// embedder_select.go calls without needing a build tag of its own; see
// embedder_available_static.go for the !onnx half.
func newONNXEmbedderOrErr(modelPath string) (Embedder, error) {
	if !ortAvailable() {
		return nil, fmt.Errorf("emojify: this binary supports ONNX Runtime but can't load it from %s — install onnxruntime (>= 1.29) or point EMOJIFY_ORT_LIBRARY_PATH at libonnxruntime: %w", sharedLibraryPath(), ensureORTInitialized())
	}
	return newONNXEmbedder(modelPath)
}

// newONNXEmbedder loads the ONNX Runtime embedder. Model/vocab resolution
// order: explicit modelPath argument -> EMOJIFY_MODEL_PATH env var ->
// DefaultModelPath if it exists on disk -> the model+vocab embedded in this
// binary. The first three read vocab.txt alongside the model file on disk;
// the embedded fallback needs no filesystem at all.
func newONNXEmbedder(modelPath string) (Embedder, error) {
	// Only the implicit default path (no explicit arg or env override) may
	// fall back to the embedded model: an explicit path that doesn't exist
	// should fail loudly, not silently substitute a different model.
	explicit := modelPath != ""
	if !explicit {
		modelPath = os.Getenv("EMOJIFY_MODEL_PATH")
		explicit = modelPath != ""
	}
	useDisk := explicit
	if !explicit {
		modelPath = DefaultModelPath
		if _, err := os.Stat(modelPath); err == nil {
			useDisk = true
		}
	}

	var vocab map[string]int64
	if useDisk {
		vocabPath := filepath.Join(filepath.Dir(modelPath), "vocab.txt")
		f, err := os.Open(vocabPath)
		if err != nil {
			return nil, fmt.Errorf("emojify: opening tokenizer vocab %s (expected alongside the model; run scripts/fetch-onnx-model.sh): %w", vocabPath, err)
		}
		defer f.Close()
		vocab, err = wordpiece.LoadVocab(bufio.NewReader(f))
		if err != nil {
			return nil, fmt.Errorf("emojify: parsing tokenizer vocab: %w", err)
		}
	} else {
		var err error
		vocab, err = wordpiece.LoadVocab(bufio.NewReader(bytes.NewReader(embeddedVocab)))
		if err != nil {
			return nil, fmt.Errorf("emojify: parsing embedded tokenizer vocab: %w", err)
		}
	}

	if err := ensureORTInitialized(); err != nil {
		return nil, fmt.Errorf("emojify: initialising ONNX Runtime: %w", err)
	}

	inputIDs, err := ort.NewTensor(ort.NewShape(1, onnxMaxTokens), make([]int64, onnxMaxTokens))
	if err != nil {
		return nil, fmt.Errorf("emojify: allocating input_ids tensor: %w", err)
	}
	attnMask, err := ort.NewTensor(ort.NewShape(1, onnxMaxTokens), make([]int64, onnxMaxTokens))
	if err != nil {
		return nil, fmt.Errorf("emojify: allocating attention_mask tensor: %w", err)
	}
	typeIDs, err := ort.NewTensor(ort.NewShape(1, onnxMaxTokens), make([]int64, onnxMaxTokens))
	if err != nil {
		return nil, fmt.Errorf("emojify: allocating token_type_ids tensor: %w", err)
	}
	output, err := ort.NewEmptyTensor[float32](ort.NewShape(1, onnxMaxTokens, onnxDims))
	if err != nil {
		return nil, fmt.Errorf("emojify: allocating output tensor: %w", err)
	}

	inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputNames := []string{"last_hidden_state"}
	inputs := []ort.Value{inputIDs, attnMask, typeIDs}
	outputs := []ort.Value{output}

	var session *ort.AdvancedSession
	if useDisk {
		session, err = ort.NewAdvancedSession(modelPath, inputNames, outputNames, inputs, outputs, nil)
	} else {
		session, err = ort.NewAdvancedSessionWithONNXData(embeddedModel, inputNames, outputNames, inputs, outputs, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("emojify: loading ONNX model: %w", err)
	}

	return &onnxEmbedder{
		session:   session,
		inputIDs:  inputIDs,
		attnMask:  attnMask,
		typeIDs:   typeIDs,
		output:    output,
		tokenizer: wordpiece.New(vocab),
	}, nil
}

func (e *onnxEmbedder) Dims() int { return onnxDims }

// defaultIndex pairs this embedder with the index built for its own
// (384-dim) vectors — see indexProvider in embedder.go.
func (e *onnxEmbedder) defaultIndex() []byte { return onnxIndexData }

func (e *onnxEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([][]float32, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ids, mask := e.tokenizer.Encode(text, onnxMaxTokens)
		paddedIDs := make([]int64, onnxMaxTokens)
		paddedMask := make([]int64, onnxMaxTokens)
		copy(paddedIDs, ids)
		copy(paddedMask, mask)

		copy(e.inputIDs.GetData(), paddedIDs)
		copy(e.attnMask.GetData(), paddedMask)
		typeIDs := e.typeIDs.GetData()
		for j := range typeIDs {
			typeIDs[j] = 0 // single-segment input
		}

		if err := e.session.Run(); err != nil {
			return nil, fmt.Errorf("emojify: running ONNX session: %w", err)
		}

		out[i] = meanPoolAndNormalize(e.output.GetData(), paddedMask, onnxMaxTokens, onnxDims)
	}
	return out, nil
}

func meanPoolAndNormalize(hidden []float32, mask []int64, seqLen, dims int) []float32 {
	sum := make([]float32, dims)
	var count float32
	for t := 0; t < seqLen; t++ {
		if mask[t] == 0 {
			continue
		}
		count++
		row := hidden[t*dims : (t+1)*dims]
		for d := 0; d < dims; d++ {
			sum[d] += row[d]
		}
	}
	if count == 0 {
		count = 1
	}
	var normSq float32
	for d := 0; d < dims; d++ {
		sum[d] /= count
		normSq += sum[d] * sum[d]
	}
	norm := sqrt32(normSq)
	if norm > 0 {
		for d := 0; d < dims; d++ {
			sum[d] /= norm
		}
	}
	return sum
}

func (e *onnxEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.inputIDs.Destroy()
	e.attnMask.Destroy()
	e.typeIDs.Destroy()
	e.output.Destroy()
	return e.session.Destroy()
}
