// vector.go
package emojify

// dot is the cosine-similarity building block for unit-normalised vectors:
// rank.go, embedder_static.go, and embedder_onnx.go all need it, and no
// build ever compiles the latter two together — this file carries no build
// tag so whichever one is active still links.
func dot(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// normalize returns v scaled to unit L2 norm (v is returned unchanged if it's all zero).
func normalize(v []float32) []float32 {
	var sumSq float32
	for _, f := range v {
		sumSq += f * f
	}
	if sumSq == 0 {
		return v
	}
	norm := sqrt32(sumSq)
	out := make([]float32, len(v))
	for i, f := range v {
		out[i] = f / norm
	}
	return out
}

// sqrt32 avoids a float64 round-trip through math.Sqrt on this package's hot
// path; Newton's method converges to float32 precision well within 20 iterations.
func sqrt32(f float32) float32 {
	if f == 0 {
		return 0
	}
	x := f
	for i := 0; i < 20; i++ {
		x = 0.5 * (x + f/x)
	}
	return x
}
