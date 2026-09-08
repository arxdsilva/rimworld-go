// Package randx provides seeded, explicit random number generation for
// deterministic simulation code. Never use math/rand's global functions in
// authoritative gameplay.
package randx

import "math/rand/v2"

// Rand is a deterministic random source seeded explicitly by the caller.
type Rand struct {
	src *rand.Rand
}

// New returns a Rand deterministically seeded from seed. The same seed
// always produces the same sequence.
func New(seed uint64) *Rand {
	return &Rand{src: rand.New(rand.NewPCG(seed, seed))}
}

// IntN returns a pseudo-random number in [0, n). It panics if n <= 0.
func (r *Rand) IntN(n int) int {
	return r.src.IntN(n)
}
