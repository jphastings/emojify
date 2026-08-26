// index.go
package emojify

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	indexMagic         = "EMJX"
	indexFormatVersion = uint16(1)
)

// Metadata describes one emoji entry alongside its vector in an Index.
type Metadata struct {
	Emoji    string
	Label    string
	Group    string
	Subgroup string
	Penalty  float32 // multiplicative rank penalty; 1.0 = no penalty
}

// Index is a fully-loaded, dequantized emoji index ready for nearest-neighbour search.
type Index struct {
	ModelID  string
	Dims     int
	Count    int
	Vectors  []float32 // flat, row-major: Count*Dims, each row unit-normalised
	Metadata []Metadata
}

// WriteIndex serializes vectors+metadata in the emojify index binary format.
// Each vector must be unit-normalised (L2 norm 1) and have length dims.
func WriteIndex(w io.Writer, modelID string, dims int, vectors [][]float32, metas []Metadata) error {
	if len(vectors) != len(metas) {
		return fmt.Errorf("emojify: %d vectors but %d metadata entries", len(vectors), len(metas))
	}
	for i, v := range vectors {
		if len(v) != dims {
			return fmt.Errorf("emojify: vector %d has %d dims, want %d", i, len(v), dims)
		}
	}

	bw := bufio.NewWriter(w)

	if _, err := bw.WriteString(indexMagic); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, indexFormatVersion); err != nil {
		return err
	}
	if err := writeString(bw, modelID); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint16(dims)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint32(len(vectors))); err != nil {
		return err
	}

	for _, v := range vectors {
		scale := quantScale(v)
		if err := binary.Write(bw, binary.LittleEndian, scale); err != nil {
			return err
		}
		for _, f := range v {
			if err := binary.Write(bw, binary.LittleEndian, quantize(f, scale)); err != nil {
				return err
			}
		}
	}

	for _, m := range metas {
		if err := writeString(bw, m.Emoji); err != nil {
			return err
		}
		if err := writeString(bw, m.Label); err != nil {
			return err
		}
		if err := writeString(bw, m.Group); err != nil {
			return err
		}
		if err := writeString(bw, m.Subgroup); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.LittleEndian, m.Penalty); err != nil {
			return err
		}
	}

	return bw.Flush()
}

// ReadIndex parses the emojify index binary format.
func ReadIndex(r io.Reader) (*Index, error) {
	br := bufio.NewReader(r)

	magic := make([]byte, len(indexMagic))
	if _, err := io.ReadFull(br, magic); err != nil {
		return nil, fmt.Errorf("emojify: reading magic: %w", err)
	}
	if string(magic) != indexMagic {
		return nil, fmt.Errorf("emojify: bad magic %q, not an emojify index", magic)
	}

	var version uint16
	if err := binary.Read(br, binary.LittleEndian, &version); err != nil {
		return nil, err
	}
	if version != indexFormatVersion {
		return nil, fmt.Errorf("emojify: index format version %d, this build supports %d", version, indexFormatVersion)
	}

	modelID, err := readString(br)
	if err != nil {
		return nil, err
	}

	var dims16 uint16
	if err := binary.Read(br, binary.LittleEndian, &dims16); err != nil {
		return nil, err
	}
	dims := int(dims16)

	var count32 uint32
	if err := binary.Read(br, binary.LittleEndian, &count32); err != nil {
		return nil, err
	}
	count := int(count32)

	vectors := make([]float32, count*dims)
	for i := 0; i < count; i++ {
		var scale float32
		if err := binary.Read(br, binary.LittleEndian, &scale); err != nil {
			return nil, err
		}
		row := vectors[i*dims : (i+1)*dims]
		for j := 0; j < dims; j++ {
			var q int8
			if err := binary.Read(br, binary.LittleEndian, &q); err != nil {
				return nil, err
			}
			row[j] = float32(q) * scale
		}
	}

	metas := make([]Metadata, count)
	for i := range metas {
		emoji, err := readString(br)
		if err != nil {
			return nil, err
		}
		label, err := readString(br)
		if err != nil {
			return nil, err
		}
		group, err := readString(br)
		if err != nil {
			return nil, err
		}
		subgroup, err := readString(br)
		if err != nil {
			return nil, err
		}
		var penalty float32
		if err := binary.Read(br, binary.LittleEndian, &penalty); err != nil {
			return nil, err
		}
		metas[i] = Metadata{Emoji: emoji, Label: label, Group: group, Subgroup: subgroup, Penalty: penalty}
	}

	return &Index{ModelID: modelID, Dims: dims, Count: count, Vectors: vectors, Metadata: metas}, nil
}

func writeString(w io.Writer, s string) error {
	if len(s) > 65535 {
		return fmt.Errorf("emojify: string too long to encode (%d bytes): %q", len(s), s)
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

func readString(r io.Reader) (string, error) {
	var n uint16
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func quantScale(v []float32) float32 {
	var maxAbs float32
	for _, f := range v {
		a := f
		if a < 0 {
			a = -a
		}
		if a > maxAbs {
			maxAbs = a
		}
	}
	if maxAbs == 0 {
		return 1
	}
	return maxAbs / 127
}

func quantize(f, scale float32) int8 {
	q := f / scale
	if q > 127 {
		q = 127
	}
	if q < -127 {
		q = -127
	}
	if q >= 0 {
		return int8(q + 0.5)
	}
	return int8(q - 0.5)
}
