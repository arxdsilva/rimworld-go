package grid

import "math/bits"

func popcount(w uint64) int      { return bits.OnesCount64(w) }
func trailingZeros(w uint64) int { return bits.TrailingZeros64(w) }
