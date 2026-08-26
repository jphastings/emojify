// internal/staticvecs/generate.go
package staticvecs

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const SourceURL = "https://github.com/RaRe-Technologies/gensim-data/releases/download/glove-wiki-gigaword-50/glove-wiki-gigaword-50.gz"

// FetchSourceVectors downloads and decompresses the GloVe source file,
// returning it as word2vec-format text (header line "<count> <dims>", then
// "word f1 f2 ... fN" per line, frequency-sorted).
func FetchSourceVectors(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("staticvecs: GET %s: %s", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	return &gzipAndBody{gz: gz, body: resp.Body}, nil
}

type gzipAndBody struct {
	gz   *gzip.Reader
	body io.Closer
}

func (g *gzipAndBody) Read(p []byte) (int, error) { return g.gz.Read(p) }
func (g *gzipAndBody) Close() error {
	g.gz.Close()
	return g.body.Close()
}

// Trim reads word2vec-format text (as FetchSourceVectors returns), keeps the
// `topN` most frequent words (the source is frequency-sorted, most frequent
// first) unioned with everything in `mustKeep`, and returns each kept word's
// vector plus the unit-normalised centroid of all of them.
func Trim(r io.Reader, topN int, mustKeep map[string]bool) (words map[string][]float32, centroid []float32, dims int, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	if !scanner.Scan() {
		return nil, nil, 0, fmt.Errorf("staticvecs: empty source")
	}
	header := strings.Fields(scanner.Text())
	if len(header) != 2 {
		return nil, nil, 0, fmt.Errorf("staticvecs: bad header line %q", scanner.Text())
	}
	dims, err = strconv.Atoi(header[1])
	if err != nil {
		return nil, nil, 0, fmt.Errorf("staticvecs: bad dims in header: %w", err)
	}

	words = make(map[string][]float32)
	sum := make([]float32, dims)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		fields := strings.Fields(scanner.Text())
		if len(fields) != dims+1 {
			continue
		}
		word := fields[0]
		if lineNum > topN && !mustKeep[word] {
			continue
		}
		vec := make([]float32, dims)
		for i, f := range fields[1:] {
			v, err := strconv.ParseFloat(f, 32)
			if err != nil {
				return nil, nil, 0, fmt.Errorf("staticvecs: parsing vector for %q: %w", word, err)
			}
			vec[i] = float32(v)
			sum[i] += vec[i]
		}
		words[word] = vec
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, 0, err
	}

	if len(words) == 0 {
		return nil, nil, 0, fmt.Errorf("staticvecs: no words kept")
	}
	for i := range sum {
		sum[i] /= float32(len(words))
	}
	return words, normalizeVec(sum), dims, nil
}

func normalizeVec(v []float32) []float32 {
	var sumSq float32
	for _, f := range v {
		sumSq += f * f
	}
	if sumSq == 0 {
		return v
	}
	norm := float32(1)
	x := sumSq
	for i := 0; i < 20; i++ {
		x = 0.5 * (x + sumSq/x)
	}
	norm = x
	out := make([]float32, len(v))
	for i, f := range v {
		out[i] = f / norm
	}
	return out
}
