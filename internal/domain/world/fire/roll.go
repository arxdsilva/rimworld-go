package fire

// Mix is splitmix64's finaliser: the hash every fire roll is drawn from
// (a fire draws nothing from the game's streams; moved from the game's
// fire.go by AX-5 for the game package's line budget).
func Mix(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ x>>30) * 0xBF58476D1CE4E5B9
	x = (x ^ x>>27) * 0x94D049BB133111EB
	return x ^ x>>31
}

// SpreadRoll is the roll (0..999999) for the fire on cell src setting
// cell dst alight on tick now, from the fire stream's seed.
func SpreadRoll(seed, now uint64, src, dst int) int {
	return int(Mix(seed^Mix(now^uint64(src)<<24^uint64(dst)<<44)) % 1_000_000)
}
