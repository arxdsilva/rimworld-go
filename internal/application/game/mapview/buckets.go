package mapview

// Buckets is one list's per-chunk buckets in compressed rows: chunk c's
// indexes are idx[start[c]:start[c+1]].
type Buckets struct {
	start []int32
	idx   []int32
	pos   []int32 // bucket's fill cursor
}

// Of is chunk c's indexes (nil when c is out of range), capped so an
// append cannot reach the next chunk's.
func (l *Buckets) Of(c int) []int32 {
	if c < 0 || c+1 >= len(l.start) {
		return nil
	}
	s := l.idx[l.start[c]:l.start[c+1]]
	return s[:len(s):len(s)]
}

// Fill fills l with the indexes 0..n-1 by chunkOf (−1: none), in
// ascending order per chunk (a counting sort; reuses l's slices).
func (l *Buckets) Fill(chunks, n int, chunkOf func(i int) int) {
	l.start = Grow32(l.start, chunks+1)
	clear(l.start)
	for i := 0; i < n; i++ {
		if c := chunkOf(i); c >= 0 {
			l.start[c+1]++
		}
	}
	for c := 0; c < chunks; c++ {
		l.start[c+1] += l.start[c]
	}
	l.idx = Grow32(l.idx, int(l.start[chunks]))
	l.pos = Grow32(l.pos, chunks)
	copy(l.pos, l.start[:chunks])
	for i := 0; i < n; i++ {
		if c := chunkOf(i); c >= 0 {
			l.idx[l.pos[c]] = int32(i)
			l.pos[c]++
		}
	}
}

// Grow32 is s resized to n, reusing its array when it is big enough.
func Grow32(s []int32, n int) []int32 {
	if cap(s) < n {
		return make([]int32, n)
	}
	return s[:n]
}

// Grow64 is s resized to n, reusing its array when it is big enough.
func Grow64(s []uint64, n int) []uint64 {
	if cap(s) < n {
		return make([]uint64, n)
	}
	return s[:n]
}

// Mix folds v into h (a 64-bit multiply-xorshift; not cryptographic, only
// for change detection).
func Mix(h, v uint64) uint64 {
	h ^= v + 0x9e3779b97f4a7c15 + h<<6 + h>>2
	h *= 0xbf58476d1ce4e5b9
	return h ^ h>>31
}

// MixString folds s's bytes and length into h.
func MixString(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h = Mix(h, uint64(s[i]))
	}
	return Mix(h, uint64(len(s)))
}

// Bit is 1 for true, 0 for false.
func Bit(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
