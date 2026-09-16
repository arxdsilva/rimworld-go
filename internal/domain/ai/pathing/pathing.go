// Package pathing owns deterministic tile pathfinding.
package pathing

// Query is the walkability surface a search needs. grid.Grid satisfies it.
type Query interface {
	Width() int
	Height() int
	Walkable(x, y int) bool
}

// Point is a tile coordinate on a path.
type Point struct {
	X, Y int
}

var neighbors = [8][2]int{
	{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1},
}

// Find returns a walkable 8-connected path from (sx,sy) to (tx,ty).
// The slice excludes the start tile and includes the destination.
// An empty slice with ok=true means the pawn is already there.
// Tie-break: lower f, then lower g, then lower y, then lower x.
func Find(q Query, sx, sy, tx, ty int) ([]Point, bool) {
	if q == nil || !inBounds(q, sx, sy) || !q.Walkable(tx, ty) {
		return nil, false
	}
	if sx == tx && sy == ty {
		return nil, true
	}

	w, h := q.Width(), q.Height()
	n := w * h
	idx := func(x, y int) int { return y*w + x }

	const inf = int(^uint(0) >> 1)
	gScore := make([]int, n)
	came := make([]int, n)
	closed := make([]bool, n)
	for i := range gScore {
		gScore[i] = inf
		came[i] = -1
	}

	start := idx(sx, sy)
	goal := idx(tx, ty)
	gScore[start] = 0

	h0 := chebyshev(sx, sy, tx, ty)
	open := heap{}
	open.push(node{f: h0, g: 0, x: sx, y: sy})

	for open.len() > 0 {
		cur := open.pop()
		ci := idx(cur.x, cur.y)
		if closed[ci] {
			continue
		}
		if ci == goal {
			return reconstruct(came, w, start, goal), true
		}
		closed[ci] = true

		for _, d := range neighbors {
			nx, ny := cur.x+d[0], cur.y+d[1]
			if !q.Walkable(nx, ny) {
				continue
			}
			if d[0] != 0 && d[1] != 0 {
				if !q.Walkable(cur.x+d[0], cur.y) || !q.Walkable(cur.x, cur.y+d[1]) {
					continue
				}
			}
			ni := idx(nx, ny)
			if closed[ni] {
				continue
			}
			ng := cur.g + 1
			if ng >= gScore[ni] {
				continue
			}
			gScore[ni] = ng
			came[ni] = ci
			open.push(node{
				f: ng + chebyshev(nx, ny, tx, ty),
				g: ng,
				x: nx,
				y: ny,
			})
		}
	}
	return nil, false
}

func inBounds(q Query, x, y int) bool {
	return x >= 0 && y >= 0 && x < q.Width() && y < q.Height()
}

func chebyshev(ax, ay, bx, by int) int {
	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}
	dy := ay - by
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

func reconstruct(came []int, w, start, goal int) []Point {
	cur := goal
	var rev []Point
	for cur != start && cur >= 0 {
		rev = append(rev, Point{X: cur % w, Y: cur / w})
		cur = came[cur]
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

type node struct {
	f, g, x, y int
}

// less reports whether a should be popped before b (stable A* tie-break).
func (a node) less(b node) bool {
	if a.f != b.f {
		return a.f < b.f
	}
	if a.g != b.g {
		return a.g < b.g
	}
	if a.y != b.y {
		return a.y < b.y
	}
	return a.x < b.x
}

type heap []node

func (h heap) len() int { return len(h) }

func (h *heap) push(n node) {
	*h = append(*h, n)
	h.up(len(*h) - 1)
}

func (h *heap) pop() node {
	old := *h
	n := old[0]
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	if len(*h) > 0 {
		(*h)[0] = last
		h.down(0)
	}
	return n
}

func (h heap) up(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if !h[i].less(h[p]) {
			return
		}
		h[i], h[p] = h[p], h[i]
		i = p
	}
}

func (h heap) down(i int) {
	n := len(h)
	for {
		l := 2*i + 1
		if l >= n {
			return
		}
		min := l
		if r := l + 1; r < n && h[r].less(h[l]) {
			min = r
		}
		if !h[min].less(h[i]) {
			return
		}
		h[i], h[min] = h[min], h[i]
		i = min
	}
}

// Dist is Chebyshev distance, used for cheap job scoring.
func Dist(ax, ay, bx, by int) int {
	return chebyshev(ax, ay, bx, by)
}

// Adjacent reports whether two tiles share an edge or corner.
func Adjacent(ax, ay, bx, by int) bool {
	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}
	dy := ay - by
	if dy < 0 {
		dy = -dy
	}
	return dx <= 1 && dy <= 1 && (dx+dy) > 0
}
