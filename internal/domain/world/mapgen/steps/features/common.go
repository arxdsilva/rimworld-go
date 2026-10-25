package features

import (
	"math"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

// rect is an inclusive cell rectangle.
type rect struct{ x0, y0, x1, y1 int }

// gap is the Euclidean distance between two rectangles (0 when they touch
// or overlap).
func (a rect) gap(b rect) float64 {
	dx := max(0, max(a.x0-b.x1, b.x0-a.x1))
	dy := max(0, max(a.y0-b.y1, b.y0-a.y1))
	return math.Hypot(float64(dx), float64(dy))
}

// openGround reports a cell a feature may stand on: walkable, dry, soft
// ground that is not rock, cave floor, road, track or already taken.
func openGround(c *mapgen.Ctx, x, y int) bool {
	if !c.In(x, y) {
		return false
	}
	i := c.Idx(x, y)
	f := c.F
	if f.Rock[i] || f.Cave[i] || f.Road[i] || f.Blocked[i] || f.Water[i].Wet() {
		return false
	}
	t := c.R.Terrain.At(x, y)
	return t.Walkable() && !terrain.IsWater(t) && t != terrain.Track && t != terrain.Stone
}

// existing lists the rectangles of every ruin and vent placed so far.
func existing(c *mapgen.Ctx) (ruins, vents []rect) {
	for _, r := range c.R.Ruins {
		ruins = append(ruins, rect{r.X0, r.Y0, r.X1, r.Y1})
	}
	for _, v := range c.R.Vents {
		vents = append(vents, rect{v.X, v.Y, v.X + 1, v.Y + 1})
	}
	return ruins, vents
}

// farFrom reports every rectangle in others at least d from r.
func farFrom(r rect, others []rect, d float64) bool {
	for _, o := range others {
		if r.gap(o) < d {
			return false
		}
	}
	return true
}

// shuffle is a Fisher-Yates shuffle on the given stream.
func shuffle[T any](r *randx.Rand, s []T) {
	for i := len(s) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

// clampInt limits v to [lo, hi].
func clampInt(v, lo, hi int) int { return min(hi, max(lo, v)) }

// ceilShare and floorShare are ceil/floor(n*p) with a float guard.
func ceilShare(n int, p float64) int  { return int(math.Ceil(float64(n)*p - 1e-9)) }
func floorShare(n int, p float64) int { return int(math.Floor(float64(n)*p + 1e-9)) }
