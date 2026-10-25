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

// Derive returns an independent stream for name under seed: the same
// (seed, name) always gives the same sequence, and different names give
// unrelated ones, so one pass's draws never shift another's.
func Derive(seed uint64, name string) *Rand {
	h := uint64(0xcbf29ce484222325) // FNV-1a over the name
	for i := 0; i < len(name); i++ {
		h ^= uint64(name[i])
		h *= 0x100000001b3
	}
	z := seed ^ h
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return New(z ^ (z >> 31))
}

// Float64 returns a pseudo-random number in [0, 1).
func (r *Rand) Float64() float64 { return r.src.Float64() }
