package power

import (
	"cmp"
	"slices"
)

// Node is one electrical building for Build: its id (an entity id,
// unique), role, footprint cells and, for a switch, whether it is closed
// (conducting).
type Node struct {
	ID     uint64
	Role   Role
	Cells  [][2]int
	Closed bool
}

// Link is the transmitter a device joins: its node id and the transmitter
// cell the link line is drawn to, the device cell it is drawn from (the
// footprint cell nearest it: Chebyshev, then straight-line distance, then
// y, then x) and their Chebyshev distance (0: a transmitter lies under
// the device).
type Link struct {
	To           uint64
	X, Y         int
	FromX, FromY int
	Dist         int
}

// Graph is the topology Build makes: the net every node is on and the
// transmitter every device links to. Read only (Nets' slice included); a
// later Build does not change it.
type Graph struct {
	ents []entry // by id, ascending
	nets []uint64
}

type entry struct {
	id, net uint64
	link    Link
	linked  bool
}

func (g *Graph) find(id uint64) (*entry, bool) {
	i, ok := slices.BinarySearchFunc(g.ents, id, func(e entry, id uint64) int { return cmp.Compare(e.id, id) })
	if !ok {
		return nil, false
	}
	return &g.ents[i], true
}

// Net is node id's net: the smallest transmitter id on it; 0 when id is
// on no net (a device with no transmitter in range, an open switch, a
// conduit off the map or under an open switch, a node with no role).
func (g *Graph) Net(id uint64) uint64 {
	if e, ok := g.find(id); ok {
		return e.net
	}
	return 0
}

// Link is the transmitter device id joins (ok false for a transmitter, a
// switch or a device with none in range).
func (g *Graph) Link(id uint64) (Link, bool) {
	if e, ok := g.find(id); ok && e.linked {
		return e.link, true
	}
	return Link{}, false
}

// Nets lists every net id, ascending.
func (g *Graph) Nets() []uint64 { return g.nets }

// Builder builds graphs, keeping its scratch between builds. The zero
// value is ready to use; it is not safe for concurrent use.
type Builder struct {
	order   []int32 // node indices by id
	parent  []int32 // union-find over order positions; -1: not a placed transmitter
	cell    []int32 // per map cell: the smallest-id transmitter's position, noCell or cutCell; all noCell between builds
	touched []int32 // cells set this build, reset at its end
}

const (
	noCell  = -1
	cutCell = -2 // under an open switch: nothing conducts here
)

// Build joins nodes on a w×h map into nets. Transmitters (conduits and
// closed switches) on 4-neighbour cells share a net; an open switch cuts:
// it is on no net itself, and nothing conducts on its cells, not even a
// conduit laid under it. Each device (producer, consumer, storage) joins
// the nearest transmitter within ConnectRange of its footprint (ties by
// distance, then y, then x; two transmitters on one cell: the smaller
// id). Cells off the map are ignored. It is deterministic in the nodes
// given, whatever their order.
func (b *Builder) Build(w, h int, nodes []Node) Graph {
	w, h = max(w, 0), max(h, 0)
	if w == 0 || h == 0 {
		w, h = 0, 0
	}
	b.grow(w*h, len(nodes))
	order := b.order[:0]
	for i := range nodes {
		order = append(order, int32(i))
	}
	slices.SortFunc(order, func(a, c int32) int { return cmp.Compare(nodes[a].ID, nodes[c].ID) })
	b.order = order
	parent := b.parent[:len(nodes)]
	in := func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h }

	// Open switches cut their cells first, so no transmitter lands there.
	for _, i := range order {
		if n := &nodes[i]; n.Role == Switch && !n.Closed {
			for _, c := range n.Cells {
				if in(c[0], c[1]) {
					b.mark(c[1]*w+c[0], cutCell)
				}
			}
		}
	}
	// Transmitters by id: a cell keeps the first (smallest id) one; a
	// later one on the same cell joins it.
	for p, i := range order {
		parent[p] = noCell
		n := &nodes[i]
		if !conducts(n) {
			continue
		}
		for _, c := range n.Cells {
			if !in(c[0], c[1]) {
				continue
			}
			k := c[1]*w + c[0]
			switch q := b.cell[k]; {
			case q == cutCell:
			case q == noCell:
				if parent[p] == noCell {
					parent[p] = int32(p)
				}
				b.mark(k, int32(p))
			default:
				if parent[p] == noCell {
					parent[p] = int32(p)
				}
				union(parent, int32(p), q)
			}
		}
	}
	// 4-neighbour links, right and down from every transmitter cell.
	for _, k := range b.touched {
		q := b.cell[k]
		if q < 0 {
			continue
		}
		if x := int(k) % w; x+1 < w && b.cell[k+1] >= 0 {
			union(parent, q, b.cell[k+1])
		}
		if int(k)+w < w*h && b.cell[int(k)+w] >= 0 {
			union(parent, q, b.cell[int(k)+w])
		}
	}

	roots, elec := 0, 0
	for p, i := range order {
		if parent[p] == int32(p) {
			roots++
		}
		if nodes[i].Role != None {
			elec++
		}
	}
	g := Graph{ents: make([]entry, 0, elec)}
	if roots > 0 {
		g.nets = make([]uint64, 0, roots)
	}
	for p, i := range order {
		n := &nodes[i]
		e := entry{id: n.ID}
		switch n.Role {
		case Transmitter, Switch:
			if parent[p] != noCell {
				r := find(parent, int32(p))
				e.net = nodes[order[r]].ID
				if r == int32(p) {
					g.nets = append(g.nets, n.ID)
				}
			}
		case Producer, Consumer, Storage:
			if q, l, ok := b.link(w, h, n.Cells); ok {
				l.To = nodes[order[q]].ID
				e.link, e.linked = l, true
				e.net = nodes[order[find(parent, q)]].ID
			}
		default:
			continue
		}
		g.ents = append(g.ents, e)
	}
	for _, k := range b.touched {
		b.cell[k] = noCell
	}
	b.touched = b.touched[:0]
	return g
}

// conducts: a conduit, or a switch while closed.
func conducts(n *Node) bool { return n.Role == Transmitter || (n.Role == Switch && n.Closed) }

// grow sizes the scratch for a map of cells cells and n nodes, keeping
// every cell noCell.
func (b *Builder) grow(cells, n int) {
	if cap(b.cell) < cells {
		b.cell = make([]int32, cells)
		for k := range b.cell {
			b.cell[k] = noCell
		}
	}
	b.cell = b.cell[:cells]
	if cap(b.parent) < n {
		b.parent = make([]int32, n)
		b.order = make([]int32, 0, n)
	}
}

func (b *Builder) mark(k int, v int32) {
	if b.cell[k] == noCell {
		b.touched = append(b.touched, int32(k))
	}
	b.cell[k] = v
}

// link finds the transmitter a footprint joins: the order position of the
// one on the chosen cell, and the link without its To.
func (b *Builder) link(w, h int, cells [][2]int) (int32, Link, bool) {
	x0, y0, x1, y1 := w, h, -1, -1
	for _, c := range cells {
		if c[0] >= 0 && c[1] >= 0 && c[0] < w && c[1] < h {
			x0, y0, x1, y1 = min(x0, c[0]), min(y0, c[1]), max(x1, c[0]), max(y1, c[1])
		}
	}
	if x1 < 0 {
		return 0, Link{}, false
	}
	x0, y0 = max(x0-ConnectRange, 0), max(y0-ConnectRange, 0)
	x1, y1 = min(x1+ConnectRange, w-1), min(y1+ConnectRange, h-1)
	best, bq := ConnectRange+1, int32(noCell)
	var l Link
scan:
	for y := y0; y <= y1; y++ {
		row := b.cell[y*w:]
		for x := x0; x <= x1; x++ {
			q := row[x]
			if q < 0 {
				continue
			}
			if d := footDist(cells, w, h, x, y); d < best {
				best, bq, l.X, l.Y = d, q, x, y
				if d == 0 {
					break scan // nothing nearer; scanning by y then x, nothing earlier
				}
			}
		}
	}
	if bq < 0 {
		return 0, Link{}, false
	}
	l.Dist = best
	bestE := -1
	for _, c := range cells {
		if c[0] < 0 || c[1] < 0 || c[0] >= w || c[1] >= h || cheb(c[0]-l.X, c[1]-l.Y) != best {
			continue
		}
		dx, dy := c[0]-l.X, c[1]-l.Y
		e := dx*dx + dy*dy
		if bestE < 0 || e < bestE || (e == bestE && (c[1] < l.FromY || (c[1] == l.FromY && c[0] < l.FromX))) {
			bestE, l.FromX, l.FromY = e, c[0], c[1]
		}
	}
	return bq, l, true
}

// footDist is the Chebyshev distance from (x, y) to the nearest on-map
// cell of a footprint.
func footDist(cells [][2]int, w, h, x, y int) int {
	d := int(^uint(0) >> 1)
	for _, c := range cells {
		if c[0] >= 0 && c[1] >= 0 && c[0] < w && c[1] < h {
			d = min(d, cheb(c[0]-x, c[1]-y))
		}
	}
	return d
}

func cheb(dx, dy int) int {
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return max(dx, dy)
}

// find is p's root (the smallest order position, so the smallest id, on
// its net), halving the path.
func find(parent []int32, p int32) int32 {
	for parent[p] != p {
		parent[p] = parent[parent[p]]
		p = parent[p]
	}
	return p
}

// union joins a's and c's sets under the smaller root.
func union(parent []int32, a, c int32) {
	ra, rc := find(parent, a), find(parent, c)
	switch {
	case ra < rc:
		parent[rc] = ra
	case rc < ra:
		parent[ra] = rc
	}
}
