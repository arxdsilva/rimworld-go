package randx

// Keyed is a counter-based stream for one key (seed, tick, stage, entity,
// sub): its draws depend only on the key and how many came before, never
// on which goroutine draws or in what order other keys draw, so a stage
// running on several lanes draws what a serial run would. A value type:
// copy it to fork a stream, and drawing allocates nothing. Its output is
// part of the simulation, so it must never change.
type Keyed struct {
	key, n uint64
}

// NewKeyed is the stream for the key: SplitMix64 folded over each part.
func NewKeyed(seed, tick, stage, entity, sub uint64) Keyed {
	k := SplitMix64(seed)
	for _, v := range [...]uint64{tick, stage, entity, sub} {
		k = SplitMix64(k ^ v)
	}
	return Keyed{key: k}
}

// Uint64 is the next draw: splitmix64's output for the key's n-th step.
func (k *Keyed) Uint64() uint64 {
	k.n++
	return Mix64(k.key + k.n*0x9E3779B97F4A7C15)
}

// IntN is a uniform draw in [0, n), by rejection (no modulo bias). It
// panics if n <= 0.
func (k *Keyed) IntN(n int) int {
	if n <= 0 {
		panic("randx: Keyed.IntN with n <= 0")
	}
	un := uint64(n)
	floor := -un % un // 2^64 mod n: draws below it would favour the low values
	for {
		if v := k.Uint64(); v >= floor {
			return int(v % un)
		}
	}
}

// Float64 is a uniform draw in [0, 1) (53 random bits).
func (k *Keyed) Float64() float64 { return float64(k.Uint64()>>11) / (1 << 53) }
