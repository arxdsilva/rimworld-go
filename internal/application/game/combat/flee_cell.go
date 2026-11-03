package combat

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/combat/weapon"
)

// When straight away is shut, the cells a fleeing creature may look through.
const (
	fleeSearchR   = 8
	fleeSearchMax = 300
)

// FleeCell is a breadth-first walk (4-neighbours, so every cell it reaches
// is walkable to) over q from (px, py), within fleeSearchR and at most
// fleeSearchMax cells, for the cell other than the start farthest from
// (tx, ty) (the first found on a tie). It allocates nothing.
func FleeCell(q pathing.Query, px, py, tx, ty int) (int, int, bool) {
	const side = 2*fleeSearchR + 1
	var seen [side * side]bool
	var queue [side * side]int16
	w, h := q.Width(), q.Height()
	at := func(x, y int) int { return (y-py+fleeSearchR)*side + x - px + fleeSearchR }
	seen[at(px, py)] = true
	queue[0] = int16(at(px, py))
	head, tail := 0, 1
	bx, by, bestD, found := px, py, -1, false
	for head < tail && head < fleeSearchMax {
		i := int(queue[head])
		head++
		cx, cy := px+i%side-fleeSearchR, py+i/side-fleeSearchR
		if cx != px || cy != py {
			if d := (cx-tx)*(cx-tx) + (cy-ty)*(cy-ty); d > bestD {
				bx, by, bestD, found = cx, cy, d, true
			}
		}
		for _, n := range [4][2]int{{cx + 1, cy}, {cx - 1, cy}, {cx, cy + 1}, {cx, cy - 1}} {
			if n[0] < 0 || n[1] < 0 || n[0] >= w || n[1] >= h || weapon.Distance(n[0], n[1], px, py) > fleeSearchR {
				continue
			}
			if j := at(n[0], n[1]); !seen[j] && q.Walkable(n[0], n[1]) {
				seen[j] = true
				queue[tail] = int16(j)
				tail++
			}
		}
	}
	return bx, by, found
}
