package fire

import "rimworld-go/pkg/randx"

// SpreadRoll is the roll (0..999999) for the fire on cell src setting
// cell dst alight on tick now, from the fire stream's seed.
func SpreadRoll(seed, now uint64, src, dst int) int {
	return int(randx.SplitMix64(seed^randx.SplitMix64(now^uint64(src)<<24^uint64(dst)<<44)) % 1_000_000)
}
