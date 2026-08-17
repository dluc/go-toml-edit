package tomledit

import (
	"runtime"
	"testing"
)

// bytesAllocated reports how many bytes f allocated on the heap. Byte counts
// (not allocation counts) are what distinguishes the quadratic parser from the
// linear one: the old rawFromTokenRange re-copied the whole token span at every
// nesting level, so the BYTES grew quadratically while Go's amortized slice
// growth kept the allocation COUNT nearly linear.
func bytesAllocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// allocRatioLimit is the largest depth-doubling allocation ratio we accept.
// Linear parsing measures ~2.14; the quadratic implementation measured ~4.06.
const allocRatioLimit = 2.5

func TestParseAllocationsScaleLinearly(t *testing.T) {
	small := []byte(deepArrayTOML(1000))
	large := []byte(deepArrayTOML(2000))

	// Warm up so lazily-initialized package state is not billed to the first run.
	_, _ = Parse(small)
	_, _ = Parse(large)

	a := bytesAllocated(func() { _, _ = Parse(small) })
	b := bytesAllocated(func() { _, _ = Parse(large) })
	ratio := float64(b) / float64(a)
	t.Logf("Parse bytes allocated: depth1000=%d depth2000=%d ratio=%.2f", a, b, ratio)
	if ratio > allocRatioLimit {
		t.Errorf("Parse allocation ratio for 2x depth = %.2f, want <= %.2f (quadratic parse regression)", ratio, allocRatioLimit)
	}
}

func TestBytesAllocationsScaleLinearly(t *testing.T) {
	small, err := Parse([]byte(deepArrayTOML(1000)))
	if err != nil {
		t.Fatalf("Parse(depth 1000) returned error: %v", err)
	}
	large, err := Parse([]byte(deepArrayTOML(2000)))
	if err != nil {
		t.Fatalf("Parse(depth 2000) returned error: %v", err)
	}
	_ = small.Bytes()
	_ = large.Bytes()

	a := bytesAllocated(func() { _ = small.Bytes() })
	b := bytesAllocated(func() { _ = large.Bytes() })
	ratio := float64(b) / float64(a)
	t.Logf("Bytes bytes allocated: depth1000=%d depth2000=%d ratio=%.2f", a, b, ratio)
	if ratio > allocRatioLimit {
		t.Errorf("Bytes allocation ratio for 2x depth = %.2f, want <= %.2f (quadratic render regression)", ratio, allocRatioLimit)
	}
}
