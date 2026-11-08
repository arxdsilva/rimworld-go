package forage

// Rings visits the tiles at Chebyshev distance 0..r from (px, py), ring by
// ring, each ring in a fixed order (the north and south rows west to east,
// then the west and east columns north to south), until fn says stop. It
// is the grazers' grass search order (game.nearestGrass).
func Rings(px, py, r int, fn func(x, y int) bool) {
	if fn(px, py) {
		return
	}
	for d := 1; d <= r; d++ {
		for x := px - d; x <= px+d; x++ {
			if fn(x, py-d) || fn(x, py+d) {
				return
			}
		}
		for y := py - d + 1; y <= py+d-1; y++ {
			if fn(px-d, y) || fn(px+d, y) {
				return
			}
		}
	}
}
