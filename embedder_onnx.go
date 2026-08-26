// embedder_onnx.go
//go:build onnx

package emojify

import (
	"bufio"
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
var defaultIndexData []byte

const (
	onnxDims         = 384
	onnxMaxTokens    = 256
	DefaultModelPath = "data/model.onnx"
	// DefaultIndexPath mirrors DefaultModelPath's pattern: the path `index
	// build` writes to by default under this build tag.
	DefaultIndexPath = "data/index.bin"
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

// NewDefaultEmbedder loads the ONNX Runtime embedder. modelPath resolution
// order: explicit argument -> EMOJIFY_MODEL_PATH env var -> DefaultModelPath.
// vocab.txt is expected alongside the model file.
func NewDefaultEmbedder(modelPath string) (Embedder, error) {
	if modelPath == "" {
		modelPath = os.Getenv("EMOJIFY_MODEL_PATH")
	}
	if modelPath == "" {
		modelPath = DefaultModelPath
	}
	vocabPath := filepath.Join(filepath.Dir(modelPath), "vocab.txt")

	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("emojify: opening tokenizer vocab %s (expected alongside the model; run scripts/fetch-onnx-model.sh): %w", vocabPath, err)
	}
	defer f.Close()
	vocab, err := wordpiece.LoadVocab(bufio.NewReader(f))
	if err != nil {
		return nil, fmt.Errorf("emojify: parsing tokenizer vocab: %w", err)
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

	session, err := ort.NewAdvancedSession(modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		[]ort.Value{inputIDs, attnMask, typeIDs},
		[]ort.Value{output},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("emojify: loading ONNX model %s: %w", modelPath, err)
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
