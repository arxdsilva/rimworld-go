// Package spatial holds the per-map cell structures the simulation reads
// in hot loops: the lazily filled byte layer and the bucket grid.
package spatial

// Unset marks a Layer cell not yet computed.
const Unset = 0xFF

// Layer is one byte per map cell, filled on demand and emptied wholesale
// when the versions its values derive from move (never saved). A value
// of Unset or more is not cacheable: the caller computes it each time.
type Layer struct {
	vals []uint8
	ver  [2]uint64
	src  any
}

// Sync empties the layer when n, ver or src (the store the values came
// from; a pointer) differ from the last call.
func (l *Layer) Sync(n int, ver [2]uint64, src any) {
	if len(l.vals) == n && l.ver == ver && l.src == src {
		return
	}
	if len(l.vals) != n {
		l.vals = make([]uint8, n)
	}
	for i := range l.vals {
		l.vals[i] = Unset
	}
	l.ver, l.src = ver, src
}

// Get is cell i's cached value (Unset: not computed).
func (l *Layer) Get(i int) uint8 { return l.vals[i] }

// Put caches v at cell i when it fits.
func (l *Layer) Put(i, v int) {
	if v >= 0 && v < Unset {
		l.vals[i] = uint8(v)
	}
}
