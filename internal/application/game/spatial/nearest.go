package spatial

import (
	"cmp"

	"rimworld-go/pkg/mathx"
)

// Nearest is the entry of b nearest (x, y) within Chebyshev radius r that
// at places (ok false: skip it), with its tile; ties go to the lower
// entry. It looks ring by ring, a bucket wide and doubling, so a near hit
// visits a few buckets (EAI-023: raiders' loot and burners' marks).
func Nearest[T cmp.Ordered](b *Grid[T], x, y, r int, at func(T) (int, int, bool)) (best T, bx, by int, ok bool) {
	bestD := r + 1
	for rr := min(BucketSize, r); ; rr = min(2*rr, r) {
		b.Each(x, y, rr, func(e T) bool {
			if px, py, in := at(e); in {
				if d := max(mathx.Abs(px-x), mathx.Abs(py-y)); d <= r && (d < bestD || d == bestD && e < best) {
					best, bx, by, bestD, ok = e, px, py, d, true
				}
			}
			return true
		})
		if ok && bestD <= rr || rr >= r {
			return best, bx, by, ok
		}
	}
}
