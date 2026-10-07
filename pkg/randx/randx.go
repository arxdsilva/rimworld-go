// Package randx provides seeded, explicit random number generation for
// deterministic simulation code. Never use math/rand's global functions in
// authoritative gameplay.
package randx

import "math/rand/v2"

// Rand is a deterministic random source seeded explicitly by the caller.
type Rand struct {
	pcg *rand.PCG
	src *rand.Rand
}

// New returns a Rand deterministically seeded from seed. The same seed
// always produces the same sequence.
func New(seed uint64) *Rand {
	p := rand.NewPCG(seed, seed)
	return &Rand{pcg: p, src: rand.New(p)}
}

// MarshalBinary captures the generator state (saved games).
func (r *Rand) MarshalBinary() ([]byte, error) { return r.pcg.MarshalBinary() }

// UnmarshalBinary restores state captured by MarshalBinary; the sequence
// continues exactly where it left off.
func (r *Rand) UnmarshalBinary(b []byte) error {
	if r.pcg == nil {
		r.pcg = rand.NewPCG(0, 0)
		r.src = rand.New(r.pcg)
	}
	return r.pcg.UnmarshalBinary(b)
}

// IntN returns a pseudo-random number in [0, n). It panics if n <= 0.
func (r *Rand) IntN(n int) int {
	return r.src.IntN(n)
}
