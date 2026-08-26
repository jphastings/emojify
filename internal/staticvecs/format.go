// internal/staticvecs/format.go
package staticvecs

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	magic   = "SVEC"
	version = uint16(1)
)

// Write serializes a word→vector map plus a fallback centroid vector.
// Every vector (including centroid) must have length dims and be unit-normalised.
func Write(w io.Writer, dims int, words map[string][]float32, centroid []float32) error {
	if len(centroid) != dims {
		return fmt.Errorf("staticvecs: centroid has %d dims, want %d", len(centroid), dims)
	}
	bw := bufio.NewWriter(w)

	if _, err := bw.WriteString(magic); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, version); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint16(dims)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint32(len(words))); err != nil {
		return err
	}

	for word, vec := range words {
		if len(vec) != dims {
			return fmt.Errorf("staticvecs: word %q has %d dims, want %d", word, len(vec), dims)
		}
		if err := writeWordVector(bw, word, vec); err != nil {
			return err
		}
	}
	if err := writeVector(bw, centroid); err != nil {
		return err
	}

	return bw.Flush()
}

// Read parses the staticvecs binary format, returning the word map, the
// fallback centroid, and the vector width.
func Read(r io.Reader) (words map[string][]float32, centroid []float32, dims int, err error) {
	br := bufio.NewReader(r)

	got := make([]byte, len(magic))
	if _, err := io.ReadFull(br, got); err != nil {
		return nil, nil, 0, fmt.Errorf("staticvecs: reading magic: %w", err)
	}
	if string(got) != magic {
		return nil, nil, 0, fmt.Errorf("staticvecs: bad magic %q", got)
	}

	var v uint16
	if err := binary.Read(br, binary.LittleEndian, &v); err != nil {
		return nil, nil, 0, err
	}
	if v != version {
		return nil, nil, 0, fmt.Errorf("staticvecs: version %d, this build supports %d", v, version)
	}

	var dims16 uint16
	if err := binary.Read(br, binary.LittleEndian, &dims16); err != nil {
		return nil, nil, 0, err
	}
	dims = int(dims16)

	var count uint32
	if err := binary.Read(br, binary.LittleEndian, &count); err != nil {
		return nil, nil, 0, err
	}

	words = make(map[string][]float32, count)
	for i := uint32(0); i < count; i++ {
		word, vec, err := readWordVector(br, dims)
		if err != nil {
			return nil, nil, 0, err
		}
		words[word] = vec
	}

	centroid, err = readVector(br, dims)
	if err != nil {
		return nil, nil, 0, err
	}

	return words, centroid, dims, nil
}

func writeWordVector(w io.Writer, word string, vec []float32) error {
	if len(word) > 255 {
		return fmt.Errorf("staticvecs: word too long: %q", word)
	}
	if err := binary.Write(w, binary.LittleEndian, uint8(len(word))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, word); err != nil {
		return err
	}
	return writeVector(w, vec)
}

func readWordVector(r io.Reader, dims int) (string, []float32, error) {
	var n uint8
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", nil, err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", nil, err
	}
	vec, err := readVector(r, dims)
	return string(buf), vec, err
}

func writeVector(w io.Writer, vec []float32) error {
	var maxAbs float32
	for _, f := range vec {
		a := f
		if a < 0 {
			a = -a
		}
		if a > maxAbs {
			maxAbs = a
		}
	}
	scale := float32(1)
	if maxAbs > 0 {
		scale = maxAbs / 127
	}
	if err := binary.Write(w, binary.LittleEndian, scale); err != nil {
		return err
	}
	for _, f := range vec {
		q := f / scale
		if q > 127 {
			q = 127
		}
		if q < -127 {
			q = -127
		}
		var qi int8
		if q >= 0 {
			qi = int8(q + 0.5)
		} else {
			qi = int8(q - 0.5)
		}
		if err := binary.Write(w, binary.LittleEndian, qi); err != nil {
			return err
		}
	}
	return nil
}

func readVector(r io.Reader, dims int) ([]float32, error) {
	var scale float32
	if err := binary.Read(r, binary.LittleEndian, &scale); err != nil {
		return nil, err
	}
	vec := make([]float32, dims)
	for i := range vec {
		var q int8
		if err := binary.Read(r, binary.LittleEndian, &q); err != nil {
			return nil, err
		}
		vec[i] = float32(q) * scale
	}
	return vec, nil
}
