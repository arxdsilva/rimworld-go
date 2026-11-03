package social

import "rimworld-go/pkg/randx"

// pairHash is symmetric in a and b: it hashes the canonical (min,max).
func pairHash(seed uint64, a, b ID) uint64 {
	if a > b {
		a, b = b, a
	}
	return randx.SplitMix64(randx.SplitMix64(a^seed) ^ randx.SplitMix64(b+0x1234567))
}

// Compatibility is how well a and b get on, permille: a stable random
// term (Irwin-Hall of four uniforms, centred, sigma about 1000) plus an
// age term. It is symmetric: f(a,b) == f(b,a).
func Compatibility(seed uint64, a, b ID, ageA, ageB int) int32 {
	h := pairHash(seed, a, b)
	sum := int64(h&0xFFFF) + int64(h>>16&0xFFFF) + int64(h>>32&0xFFFF) + int64(h>>48&0xFFFF)
	random := (sum*2 - irwinMeanTwice) * 1000 / (2 * irwinSigmaTimes)
	return int32(random) + ageTerm(ageA, ageB)
}

func ageTerm(ageA, ageB int) int32 {
	d := ageA - ageB
	if d < 0 {
		d = -d
	}
	gap := d * compatAgeStep
	if gap > compatAgeCap {
		gap = compatAgeCap
	}
	return int32(compatAgeMax * (1000 - gap) / 1000)
}
