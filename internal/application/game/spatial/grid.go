package spatial

import "rimworld-go/pkg/mathx"

// BucketSize is the side of one bucket in tiles.
const BucketSize = 8

// Grid is a compressed bucket list over the map: bucket b's entries are
// ents[start[b]:start[b+1]]. It is built in two passes over the same n
// inputs, in input order (Begin, Add for each input, Seal, Put for each
// input), so a bucket keeps its entries in input order and no per-entry
// callback is needed. Not saved.
type Grid[T any] struct {
	bw, bh int
	n      int
	start  []int32
	ents   []T
	tmp    []int32 // scratch: each input's bucket, then the fill cursors
}

// Reset empties the grid for a w x h map.
func (b *Grid[T]) Reset(w, h int) {
	b.bw, b.bh = (w+BucketSize-1)/BucketSize, (h+BucketSize-1)/BucketSize
	n := b.bw*b.bh + 1
	if cap(b.start) < n {
		b.start = make([]int32, n)
	}
	b.start = b.start[:n]
	clear(b.start)
	b.ents = b.ents[:0]
}

// Bucket is the bucket of tile (x, y), clamped onto the map.
func (b *Grid[T]) Bucket(x, y int) int {
	bx, by := mathx.Clamp(x/BucketSize, 0, b.bw-1), mathx.Clamp(y/BucketSize, 0, b.bh-1)
	return by*b.bw + bx
}

// Begin starts the first pass over n inputs (after Reset).
func (b *Grid[T]) Begin(n int) {
	b.n = n
	if cap(b.tmp) < n+len(b.start) {
		b.tmp = make([]int32, n+len(b.start))
	}
}

// Add counts input i into bucket bi (-1: not indexed).
func (b *Grid[T]) Add(i, bi int) {
	b.tmp[i] = int32(bi)
	if bi >= 0 {
		b.start[bi+1]++
	}
}

// Seal ends the first pass and sizes the entries.
func (b *Grid[T]) Seal() {
	for i := 1; i < len(b.start); i++ {
		b.start[i] += b.start[i-1]
	}
	total := int(b.start[len(b.start)-1])
	if cap(b.ents) < total {
		b.ents = make([]T, total)
	}
	b.ents = b.ents[:total]
	copy(b.tmp[b.n:b.n+len(b.start)-1], b.start[:len(b.start)-1])
}

// Put places input i's entry (second pass, same order as Add).
func (b *Grid[T]) Put(i int, e T) {
	if bi := b.tmp[i]; bi >= 0 {
		next := b.tmp[b.n:]
		b.ents[next[bi]] = e
		next[bi]++
	}
}

// Each calls fn for every entry in the buckets overlapping the square of
// Chebyshev radius r around (x, y); fn returns false to stop.
func (b *Grid[T]) Each(x, y, r int, fn func(T) bool) {
	if b.bw == 0 {
		return
	}
	bx0, by0 := mathx.Clamp((x-r)/BucketSize, 0, b.bw-1), mathx.Clamp((y-r)/BucketSize, 0, b.bh-1)
	bx1, by1 := mathx.Clamp((x+r)/BucketSize, 0, b.bw-1), mathx.Clamp((y+r)/BucketSize, 0, b.bh-1)
	for by := by0; by <= by1; by++ {
		for bx := bx0; bx <= bx1; bx++ {
			bi := by*b.bw + bx
			for _, e := range b.ents[b.start[bi]:b.start[bi+1]] {
				if !fn(e) {
					return
				}
			}
		}
	}
}

// Same reports whether o holds the same buckets and entries (the index
// self-check).
func Same[T comparable](a, o *Grid[T]) bool {
	if len(a.start) != len(o.start) || len(a.ents) != len(o.ents) {
		return false
	}
	for i, s := range a.start {
		if s != o.start[i] {
			return false
		}
	}
	for i, e := range a.ents {
		if e != o.ents[i] {
			return false
		}
	}
	return true
}
