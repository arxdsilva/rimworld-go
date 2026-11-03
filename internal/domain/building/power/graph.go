package power

import (
	"cmp"
	"slices"
)

// Node is one electrical building for Build: its id (an entity id,
// unique), role, footprint cells and, for a switch, whether it is closed
// (conducting). Pin is the conductor a device was reconnected to (PWR-4; 0:
// the nearest).
type Node struct {
	ID     uint64
	Role   Role
	Cells  [][2]int
	Closed bool
	Pin    uint64
}

// Link is the conductor a device is wired to: its node id and the
// conductor cell the wire is drawn to, the device cell it is drawn from
// (the footprint cell nearest it: Chebyshev, then straight-line distance,
// then y, then x) and their Chebyshev distance (0: a conductor lies under
// the device). Next is the conductor Reconnect moves it to (PWR-4): the
// nearest cell's owner in the next set in range, by ascending net id and
// wrapping (0: no other set in range).
type Link struct {
	To           uint64
	X, Y         int
	FromX, FromY int
	Dist         int
	Next         uint64
}

// Graph is the topology Build makes: the net every node is on and the
// conductor every wired device links to. Read only (Nets' slice included);
// a later Build does not change it.
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

// Net is node id's net: the smallest conductor id on it; 0 when id is on
// no net (a consumer with no conductor in range, a generator or battery
// with nothing touching or wired to it, an open switch, a conduit off the
// map or under an open switch, a node with no role).
func (g *Graph) Net(id uint64) uint64 {
	if e, ok := g.find(id); ok {
		return e.net
	}
	return 0
}

// Link is the wire of device id (ok false for a conduit, a switch, a
// generator or battery on a grid another member wires, or carried by a
// conduit or switch it touches, or a device with none in range).
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
	parent  []int32 // union-find over order positions: a conductor's set, or the conductor a non-conductor is wired to; -1: neither
	first   []int32 // by order position: its set's root before any wire; noCell: not a placed conductor
	owner   []int32 // by that root: the position wiring the set out, noCell or carried
	joined  []bool  // by order position: shares a set with another member, once wired
	cell    []int32 // per map cell: the smallest-id conductor's position, noCell or cutCell; all noCell between builds
	touched []int32 // cells set this build, reset at its end
}

const (
	noCell  = -1
	cutCell = -2 // under an open switch: nothing conducts here
	carried = -2 // owner: the set has a conduit or closed switch, so no wire
)

// Build joins nodes on a w×h map into nets. Conductors (conduits, closed
// switches, generators, batteries) on 4-neighbour or shared cells form a
// set. An open switch cuts: it is on no net itself, and nothing conducts on
// its cells, not even a conduit laid under it. A set with a conduit or closed
// switch needs no wire. A set of only generators and batteries wires out
// once: the smallest-id member whose Pin is usable owns the wire, else the
// smallest-id member with a conductor outside the set in range, and the wire
// merges the two sets; the other members have none. A consumer, or a device
// with no cell left to conduct on, is wired alone and joins without merging.
// A wire goes to the Pin if that conducts on an uncut cell within ConnectRange
// of the footprint, outside the set, else to the nearest conductor in range
// outside it (ties by distance, then straight line, then y, then x; two
// conductors on one cell: the smaller id). A single generator or battery with nothing wired either way
// is on no net; several touching stay their own net. Cells off the map are
// ignored. It is deterministic in the nodes given, whatever their order: every
// conductor is placed and every owner chosen, from the sets as they touch,
// before the first wire.
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

	// Open switches cut their cells first, so no conductor lands there.
	for _, i := range order {
		if n := &nodes[i]; n.Role == Switch && !n.Closed {
			for _, c := range n.Cells {
				if in(c[0], c[1]) {
					b.mark(c[1]*w+c[0], cutCell)
				}
			}
		}
	}
	// Conductors by id: a cell keeps the first (smallest id) one; a
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
	// 4-neighbour links, right and down from every conductor cell.
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

	first, owner := b.chooseOwners(w, h, nodes)
	elec := 0
	for _, i := range order {
		if nodes[i].Role != None {
			elec++
		}
	}
	// Wires: every owner, consumer and unplaced device. first and cell never
	// move from here on, so a wire does not depend on the wires before it.
	g := Graph{ents: make([]entry, 0, elec)}
	for p, i := range order {
		n := &nodes[i]
		if n.Role == None {
			continue
		}
		e := entry{id: n.ID}
		if n.Role.Device() && (first[p] == noCell || owner[first[p]] == int32(p)) {
			if q, l, ok := b.link(w, h, n, first[p], nodes); ok {
				l.To = nodes[order[q]].ID
				e.link, e.linked = l, true
				if parent[p] == noCell {
					parent[p] = q
				} else {
					union(parent, int32(p), q)
				}
			}
		}
		g.ents = append(g.ents, e)
	}
	joined := b.joined[:len(nodes)]
	clear(joined)
	markJoined(parent, joined)

	roots := 0
	for p := range parent[:len(nodes)] {
		if parent[p] == int32(p) {
			roots++
		}
	}
	if roots > 0 {
		g.nets = make([]uint64, 0, roots)
	}
	k := 0
	for p, i := range order {
		n := &nodes[i]
		if n.Role == None {
			continue
		}
		e := &g.ents[k]
		k++
		if parent[p] == noCell || (!joined[p] && n.Role.Device()) {
			continue
		}
		r := find(parent, int32(p))
		e.net = nodes[order[r]].ID
		if r == int32(p) {
			g.nets = append(g.nets, n.ID)
		}
		if e.linked {
			if q := b.next(w, h, n.Cells, r, parent); q != noCell {
				e.link.Next = nodes[order[q]].ID
			}
		}
	}
	for _, k := range b.touched {
		b.cell[k] = noCell
	}
	b.touched = b.touched[:0]
	return g
}

// conducts: a conduit, a generator, a battery, or a switch while closed.
func conducts(n *Node) bool {
	return n.Role == Transmitter || n.Role == Producer || n.Role == Storage || (n.Role == Switch && n.Closed)
}

// chooseOwners snapshots the sets as they touch, and picks who wires each
// set of only generators and batteries: the smallest id with a usable pin,
// else the smallest id with a conductor outside the set in range.
func (b *Builder) chooseOwners(w, h int, nodes []Node) (first, owner []int32) {
	parent := b.parent[:len(nodes)]
	first, owner = b.first[:len(nodes)], b.owner[:len(nodes)]
	for p := range first {
		first[p], owner[p] = noCell, noCell
		if parent[p] != noCell {
			first[p] = find(parent, int32(p))
		}
	}
	for p, i := range b.order {
		if first[p] != noCell && !nodes[i].Role.Device() {
			owner[first[p]] = carried
		}
	}
	for p, i := range b.order {
		if s := first[p]; s != noCell && owner[s] == noCell {
			if _, _, ok := b.pinned(w, h, &nodes[i], s, nodes); ok {
				owner[s] = int32(p)
			}
		}
	}
	for p, i := range b.order {
		if s := first[p]; s != noCell && owner[s] == noCell {
			if _, _, ok := b.nearest(w, h, nodes[i].Cells, s); ok {
				owner[s] = int32(p)
			}
		}
	}
	return first, owner
}

// markJoined flags every member of a set that has another: its root and
// the rest.
func markJoined(parent []int32, joined []bool) {
	for p := range joined {
		if parent[p] == noCell {
			continue
		}
		if r := find(parent, int32(p)); r != int32(p) {
			joined[p], joined[r] = true, true
		}
	}
}

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
		b.first = make([]int32, n)
		b.owner = make([]int32, n)
		b.joined = make([]bool, n)
		b.order = make([]int32, 0, n)
	}
}

func (b *Builder) mark(k int, v int32) {
	if b.cell[k] == noCell {
		b.touched = append(b.touched, int32(k))
	}
	b.cell[k] = v
}

// link finds node n's wire (set: its first; noCell for one that isn't a
// placed conductor): the order position of the chosen conductor, and the
// link without its To or Next.
func (b *Builder) link(w, h int, n *Node, set int32, nodes []Node) (int32, Link, bool) {
	q, l, ok := b.pinned(w, h, n, set, nodes)
	if !ok {
		q, l, ok = b.nearest(w, h, n.Cells, set)
	}
	if !ok {
		return 0, Link{}, false
	}
	bestE := -1
	for _, c := range n.Cells {
		if c[0] < 0 || c[1] < 0 || c[0] >= w || c[1] >= h || cheb(c[0]-l.X, c[1]-l.Y) != l.Dist {
			continue
		}
		dx, dy := c[0]-l.X, c[1]-l.Y
		e := dx*dx + dy*dy
		if bestE < 0 || e < bestE || (e == bestE && (c[1] < l.FromY || (c[1] == l.FromY && c[0] < l.FromX))) {
			bestE, l.FromX, l.FromY = e, c[0], c[1]
		}
	}
	return q, l, true
}

// pinned is n's pin while it can take the wire: it conducts, it isn't in
// n's own set, and a cell of it lies uncut and in range (the nearest:
// distance, straight line, y, x).
func (b *Builder) pinned(w, h int, n *Node, set int32, nodes []Node) (int32, Link, bool) {
	var l Link
	if n.Pin == 0 {
		return 0, l, false
	}
	q, ok := slices.BinarySearchFunc(b.order, n.Pin, func(i int32, id uint64) int { return cmp.Compare(nodes[i].ID, id) })
	if !ok || !conducts(&nodes[b.order[q]]) || (set != noCell && b.first[q] == set) {
		return 0, l, false
	}
	l.Dist = ConnectRange + 1
	bestE := 0
	for _, c := range nodes[b.order[q]].Cells {
		if c[0] < 0 || c[1] < 0 || c[0] >= w || c[1] >= h || b.cell[c[1]*w+c[0]] < 0 {
			continue
		}
		d, e := footDist(n.Cells, w, h, c[0], c[1])
		if d < l.Dist || (d == l.Dist && (e < bestE || (e == bestE && (c[1] < l.Y || (c[1] == l.Y && c[0] < l.X))))) {
			l.Dist, bestE, l.X, l.Y = d, e, c[0], c[1]
		}
	}
	return int32(q), l, l.Dist <= ConnectRange
}

// nearest is the conductor nearest a footprint within ConnectRange, outside
// set (distance, then straight line, then y, then x).
func (b *Builder) nearest(w, h int, cells [][2]int, set int32) (int32, Link, bool) {
	x0, y0, x1, y1, ok := reach(w, h, cells)
	if !ok {
		return 0, Link{}, false
	}
	best, bestE, bq := ConnectRange+1, 0, int32(noCell)
	var l Link
scan:
	for y := y0; y <= y1; y++ {
		row := b.cell[y*w:]
		for x := x0; x <= x1; x++ {
			q := row[x]
			if q < 0 || b.first[q] == set {
				continue
			}
			if d, e := footDist(cells, w, h, x, y); d < best || (d == best && e < bestE) {
				best, bestE, bq, l.X, l.Y = d, e, q, x, y
				if d == 0 {
					break scan // nothing nearer; scanning by y then x, nothing earlier
				}
			}
		}
	}
	l.Dist = best
	return bq, l, bq >= 0
}

// next is the order position of the conductor Reconnect moves a wire on the
// set rooted at cur to: in the next set in range by ascending root (so by
// net id), wrapping, the owner of that set's nearest cell (distance, then
// straight line, then y, then x); noCell with no other set in range. A generator or battery alone
// counts: wiring to it makes a grid. Sets must be final.
func (b *Builder) next(w, h int, cells [][2]int, cur int32, parent []int32) int32 {
	x0, y0, x1, y1, ok := reach(w, h, cells)
	if !ok {
		return noCell
	}
	after, first := hop{root: noCell, q: noCell}, hop{root: noCell, q: noCell}
	for y := y0; y <= y1; y++ {
		row := b.cell[y*w:]
		for x := x0; x <= x1; x++ {
			q := row[x]
			if q < 0 {
				continue
			}
			r := find(parent, q)
			if r == cur {
				continue
			}
			if d, e := footDist(cells, w, h, x, y); d <= ConnectRange {
				first.see(r, q, d, e)
				if r > cur {
					after.see(r, q, d, e)
				}
			}
		}
	}
	if after.q != noCell {
		return after.q
	}
	return first.q
}

// hop is the best cell seen for next: the lowest root, then the nearest,
// then the straightest, then the first scanned.
type hop struct {
	root, q int32
	d, e    int
}

func (h *hop) see(root, q int32, d, e int) {
	if h.q == noCell || root < h.root || (root == h.root && (d < h.d || (d == h.d && e < h.e))) {
		*h = hop{root, q, d, e}
	}
}

// reach is the on-map window within ConnectRange of a footprint's box (ok
// false: none of it on the map).
func reach(w, h int, cells [][2]int) (x0, y0, x1, y1 int, ok bool) {
	x0, y0, x1, y1 = w, h, -1, -1
	for _, c := range cells {
		if c[0] >= 0 && c[1] >= 0 && c[0] < w && c[1] < h {
			x0, y0, x1, y1 = min(x0, c[0]), min(y0, c[1]), max(x1, c[0]), max(y1, c[1])
		}
	}
	if x1 < 0 {
		return 0, 0, 0, 0, false
	}
	return max(x0-ConnectRange, 0), max(y0-ConnectRange, 0), min(x1+ConnectRange, w-1), min(y1+ConnectRange, h-1), true
}

// footDist is the Chebyshev distance from (x, y) to the nearest on-map
// cell of a footprint, and the squared straight-line distance to the
// nearest of the cells at that distance: of two conductors equally far, the
// wire takes the shorter line.
func footDist(cells [][2]int, w, h, x, y int) (d, e int) {
	d, e = int(^uint(0)>>1), 0
	for _, c := range cells {
		if c[0] >= 0 && c[1] >= 0 && c[0] < w && c[1] < h {
			dx, dy := c[0]-x, c[1]-y
			if k, s := cheb(dx, dy), dx*dx+dy*dy; k < d || (k == d && s < e) {
				d, e = k, s
			}
		}
	}
	return d, e
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
// its net, a wired non-conductor never being one), halving the path.
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
