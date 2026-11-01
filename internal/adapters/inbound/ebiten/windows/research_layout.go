package windows

import (
	"math"
	"slices"
	"sort"
)

// The research tree's layout (RSCH-2, MAIN_UI §24 "dynamically manageable
// graph layout"): a layered drawing of the prerequisite DAG, computed from
// the projects alone, so a project added to the table finds its own place.
//
//   - Columns: each tech level is a band of columns, in level order. Inside
//     a band a project sits one column right of its deepest prerequisite of
//     the same level; a prerequisite from an earlier level puts it in the
//     band's first column.
//   - Rows: the order inside each column comes from barycentre sweeps (each
//     project moves toward the mean row of its prerequisites, then of its
//     dependents), keeping the order with the fewest line crossings. Rows
//     are then placed so a project lines up with its prerequisites where
//     there is room, which keeps chains straight.
//
// The layout is pure and deterministic: the same projects give the same
// picture. The window computes it once per change of the research list
// (researchState.refresh), never per frame.

// treeBand is one tech level's columns, [Col0, Col1).
type treeBand struct {
	Level      int
	Col0, Col1 int
}

// treeGraph is the layout: each node's column and row, the grid's size and
// the tech-level bands.
type treeGraph struct {
	Col, Row   []int
	Cols, Rows int
	Bands      []treeBand
}

// layoutSweeps is how many down-and-up barycentre rounds the ordering runs
// (it stops early once no line crosses another).
const layoutSweeps = 8

// layoutTree lays out n = len(level) nodes: level[i] is node i's tech level
// and pre[i] its drawn prerequisites (node indices; out-of-range indices and
// cycles are ignored).
func layoutTree(level []int, pre [][]int) treeGraph {
	n := len(level)
	g := treeGraph{Col: make([]int, n), Row: make([]int, n)}
	if n == 0 {
		return g
	}
	depth := bandDepths(level, pre)
	g.Bands, g.Cols = bandColumns(level, depth)
	start := map[int]int{}
	for _, b := range g.Bands {
		start[b.Level] = b.Col0
	}
	for i := range n {
		g.Col[i] = start[level[i]] + depth[i]
	}
	byCol := make([][]int, g.Cols)
	for i := range n { // the table's order is the first guess
		byCol[g.Col[i]] = append(byCol[g.Col[i]], i)
	}
	post := make([][]int, n)
	for i, ps := range pre {
		for _, p := range ps {
			if p >= 0 && p < n && p != i {
				post[p] = append(post[p], i)
			}
		}
	}
	orderColumns(byCol, g.Col, pre, post)
	g.Rows = placeRows(byCol, g.Col, pre, g.Row)
	return g
}

// bandDepths is each node's column inside its tech level's band: 0 without
// a prerequisite of the same level, else one more than the deepest such.
func bandDepths(level []int, pre [][]int) []int {
	n := len(level)
	depth := make([]int, n)
	state := make([]uint8, n) // 0 unseen, 1 on the path, 2 done
	var visit func(i int) int
	visit = func(i int) int {
		switch state[i] {
		case 1:
			return 0 // a cycle: cut it here
		case 2:
			return depth[i]
		}
		state[i] = 1
		d := 0
		for _, p := range pre[i] {
			if p >= 0 && p < n && p != i && level[p] == level[i] {
				d = max(d, visit(p)+1)
			}
		}
		depth[i], state[i] = d, 2
		return d
	}
	for i := range n {
		visit(i)
	}
	return depth
}

// bandColumns are the bands of the levels present, in level order, each as
// wide as its deepest node, and the total column count.
func bandColumns(level, depth []int) ([]treeBand, int) {
	width := map[int]int{}
	for i, l := range level {
		width[l] = max(width[l], depth[i]+1)
	}
	levels := make([]int, 0, len(width))
	for l := range width {
		levels = append(levels, l)
	}
	slices.Sort(levels)
	bands := make([]treeBand, 0, len(levels))
	col := 0
	for _, l := range levels {
		bands = append(bands, treeBand{Level: l, Col0: col, Col1: col + width[l]})
		col += width[l]
	}
	return bands, col
}

// orderColumns reorders each column by barycentre sweeps and leaves the
// order with the fewest crossings in byCol.
func orderColumns(byCol [][]int, col []int, pre, post [][]int) {
	n := len(col)
	pos := make([]float64, n)
	setPos := func(c int) {
		for k, i := range byCol[c] {
			pos[i] = float64(k)
		}
	}
	for c := range byCol {
		setPos(c)
	}
	key := make([]float64, n)
	sortBy := func(c int, nbr [][]int) {
		for _, i := range byCol[c] {
			sum, cnt := 0.0, 0
			for _, j := range nbr[i] {
				if j >= 0 && j < n && j != i {
					sum += pos[j]
					cnt++
				}
			}
			key[i] = pos[i]
			if cnt > 0 {
				key[i] = sum / float64(cnt)
			}
		}
		sort.SliceStable(byCol[c], func(a, b int) bool { return key[byCol[c][a]] < key[byCol[c][b]] })
		setPos(c)
	}
	rowOf := func(i int) float64 { return pos[i] }
	best := cloneCols(byCol)
	bestX := countCrossings(col, rowOf, pre)
	for it := 0; it < layoutSweeps && bestX > 0; it++ {
		for c := 1; c < len(byCol); c++ {
			sortBy(c, pre)
		}
		for c := len(byCol) - 2; c >= 0; c-- {
			sortBy(c, post)
		}
		if x := countCrossings(col, rowOf, pre); x < bestX {
			best, bestX = cloneCols(byCol), x
		}
	}
	for c := range byCol {
		copy(byCol[c], best[c])
	}
}

// cloneCols is a deep copy of the columns.
func cloneCols(byCol [][]int) [][]int {
	out := make([][]int, len(byCol))
	for c := range byCol {
		out[c] = slices.Clone(byCol[c])
	}
	return out
}

// placeRows gives each node a row, column by column from the left, in the
// columns' order: as near the mean row of its prerequisites as the nodes
// above it allow (a node without one packs under the last). It returns the
// row count.
func placeRows(byCol [][]int, col []int, pre [][]int, row []int) int {
	n := len(col)
	rows := 0
	for c := range byCol {
		next := 0
		for _, i := range byCol[c] {
			sum, cnt := 0, 0
			for _, p := range pre[i] {
				if p >= 0 && p < n && col[p] < c {
					sum += row[p]
					cnt++
				}
			}
			want := next
			if cnt > 0 {
				want = int(math.Round(float64(sum) / float64(cnt)))
			}
			row[i] = max(want, next)
			next = row[i] + 1
		}
		rows = max(rows, next)
	}
	return rows
}

// countCrossings counts the pairs of prerequisite lines that cross, each
// line straight from its prerequisite's column and row to its dependent's.
// Lines that only meet at a node do not count.
func countCrossings(col []int, row func(int) float64, pre [][]int) int {
	type seg struct{ x0, x1, y0, y1 float64 }
	var segs []seg
	n := len(col)
	for i, ps := range pre {
		for _, p := range ps {
			if p >= 0 && p < n && col[p] < col[i] {
				segs = append(segs, seg{float64(col[p]), float64(col[i]), row(p), row(i)})
			}
		}
	}
	at := func(s seg, x float64) float64 { return s.y0 + (s.y1-s.y0)*(x-s.x0)/(s.x1-s.x0) }
	x := 0
	for a := range segs {
		for b := a + 1; b < len(segs); b++ {
			s, t := segs[a], segs[b]
			lo, hi := max(s.x0, t.x0), min(s.x1, t.x1)
			if lo >= hi {
				continue
			}
			d0, d1 := at(s, lo)-at(t, lo), at(s, hi)-at(t, hi)
			if d0*d1 < 0 {
				x++
			}
		}
	}
	return x
}

// crossings is the finished layout's crossing count.
func (g treeGraph) crossings(pre [][]int) int {
	return countCrossings(g.Col, func(i int) float64 { return float64(g.Row[i]) }, pre)
}
