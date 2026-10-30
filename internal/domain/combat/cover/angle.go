package cover

// Cover by angle and range (CBT-2). wiki: Cover: cover works best against
// shots coming straight at it, by the angle between the shot and the
// cover as seen from the target (< 15 deg 100 %, 15-27 80 %, 27-40 60 %,
// 40-52 40 %, 52-65 20 %, > 65 0 %), and at point-blank range it is only a
// third as good with the shooter right in front of it, two thirds with
// the shooter one tile from it.

// angleBands are the wiki's angle limits as tangents (thousandths) with
// the share of the cover that holds below each.
var angleBands = [...]struct{ tan, p int }{
	{268, 1000}, // tan 15 deg
	{510, 800},  // tan 27 deg
	{839, 600},  // tan 40 deg
	{1280, 400}, // tan 52 deg
	{2145, 200}, // tan 65 deg
}

// AngleFactor is the share (permille) of the cover on (cx, cy) that
// holds against a shot from (sx, sy) at a target on (tx, ty): by the angle
// at the target between the shooter and the cover (integer arithmetic:
// the tangent is compared, never computed).
func AngleFactor(sx, sy, tx, ty, cx, cy int) int {
	ux, uy := int64(cx-tx), int64(cy-ty) // towards the cover
	vx, vy := int64(sx-tx), int64(sy-ty) // towards the shooter
	dot := ux*vx + uy*vy
	if dot <= 0 || ux == 0 && uy == 0 {
		return 0 // the cover is beside or behind the target
	}
	cross := ux*vy - uy*vx
	c2, d2 := cross*cross*1_000_000, dot*dot
	for _, b := range angleBands {
		if c2 < int64(b.tan)*int64(b.tan)*d2 {
			return b.p
		}
	}
	return 0
}

// Point-blank partial cover (permille of the cover). wiki: Cover
const (
	FrontFactor = 333 // the shooter stands right in front of the cover
	NearFactor  = 667 // the shooter is one tile from it
)

// RangeFactor is the share (permille) of the cover on (cx, cy) that holds
// against a shooter on (sx, sy): a third when it stands beside the cover,
// two thirds a tile further, else all of it.
func RangeFactor(sx, sy, cx, cy int) int {
	switch max(abs(sx-cx), abs(sy-cy)) {
	case 0, 1:
		return FrontFactor
	case 2:
		return NearFactor
	}
	return 1000
}

// Effective is the cover (permille) a target on (tx, ty) has against a
// shot from (sx, sy): every cover on the eight tiles around it, each at
// its value scaled by its angle and the shooter's range, combined as
// independent catches (two low walls at an angle each take their share),
// with the strongest one and its tile (the one a caught shot strikes).
// Same tile or no cover: (0, None, tx, ty).
func Effective(g Grid, sx, sy, tx, ty int) (p int, best Kind, bx, by int) {
	bx, by = tx, ty
	if sx == tx && sy == ty {
		return 0, None, bx, by
	}
	pass, bp := 1000, 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			cx, cy := tx+dx, ty+dy
			if dx == 0 && dy == 0 || cx == sx && cy == sy {
				continue
			}
			k := g.CoverAt(cx, cy)
			v := Permille(k)
			if v == 0 {
				continue
			}
			e := v * AngleFactor(sx, sy, tx, ty, cx, cy) / 1000 * RangeFactor(sx, sy, cx, cy) / 1000
			if e <= 0 {
				continue
			}
			pass = pass * (1000 - e) / 1000
			if e > bp {
				best, bp, bx, by = k, e, cx, cy
			}
		}
	}
	return 1000 - pass, best, bx, by
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
