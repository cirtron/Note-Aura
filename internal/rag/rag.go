// Package rag provides text chunking, embedding (de)serialization, and
// brute-force cosine similarity search over note chunks. At MVP scale a linear
// scan in Go is fast and keeps the stack CGO-free (no sqlite-vec extension).
package rag

import (
	"encoding/binary"
	"math"
	"sort"
	"strings"
)

// Chunk splits text into ~chunkRunes-sized pieces on paragraph/sentence
// boundaries, with a small overlap so context isn't lost at the seams.
func Chunk(text string, chunkRunes, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= chunkRunes {
		return []string{text}
	}
	var out []string
	for start := 0; start < len(runes); {
		end := start + chunkRunes
		if end >= len(runes) {
			out = append(out, strings.TrimSpace(string(runes[start:])))
			break
		}
		// Try to break on a newline or space near the end for cleaner chunks.
		brk := end
		for i := end; i > start+chunkRunes/2; i-- {
			if runes[i] == '\n' || runes[i] == ' ' {
				brk = i
				break
			}
		}
		out = append(out, strings.TrimSpace(string(runes[start:brk])))
		start = brk - overlap
		if start < 0 {
			start = 0
		}
		if brk == start { // safety against non-progress
			start = brk
		}
	}
	// Drop empties.
	filtered := out[:0]
	for _, c := range out {
		if strings.TrimSpace(c) != "" {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// EncodeEmbedding serializes a float32 vector as little-endian bytes for BLOB
// storage.
func EncodeEmbedding(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// DecodeEmbedding reverses EncodeEmbedding.
func DecodeEmbedding(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// Cosine returns cosine similarity of two equal-length vectors.
func Cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
}

// Candidate is a scored chunk for ranking.
type Candidate struct {
	NoteID int64
	Text   string
	Score  float32
}

// TopK ranks candidates by descending score and returns the best k.
func TopK(cands []Candidate, k int) []Candidate {
	sort.Slice(cands, func(i, j int) bool { return cands[i].Score > cands[j].Score })
	if len(cands) > k {
		cands = cands[:k]
	}
	return cands
}
