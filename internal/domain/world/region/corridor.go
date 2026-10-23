package region

// Long paths (NG-1 §6.5, E3): a route over the region graph — regions as
// nodes at their cells' mean position, octile distances between them —
// picks a corridor (the regions on the route plus one ring of their
// neighbours) that a tile search then stays inside.

// regionGraph is the region adjacency (both directions), CSR, built on
// demand after the regions change.
// corridorRings is how many rings of neighbouring regions widen the
// corridor around the route (two keep long paths within a few percent of
// optimal: TestCorridorPathValid).
const corridorRings = 2

type regionGraph struct {
	ok    bool
	gen   uint32  // rebuilds
	start []int32 // per region: its neighbours are adj[start[r]:start[r+1]]
	adj   []int32
	mid   []uint32 // per adj entry: the border's middle tile, x<<16 | y
	keys  []key    // per region: chunk and local id
}

// corridorScratch is the region search's reusable state.
type corridorScratch struct {
	stamp  []uint32 // per region: g/came valid this search
	gen    uint32
	g      []int32
	came   []int32
	pos    []uint32 // per region: where the best route enters it
	closed []bool
	open   []rnode
	mark   []uint32 // per region: in the corridor (mark == markGen)
	markG  uint32
	marked []int32 // regions in the corridor being built
	allow  []uint64
}

type rnode struct{ f, g, r int32 }

func (a rnode) less(b rnode) bool {
	if a.f != b.f {
		return a.f < b.f
	}
	if a.g != b.g {
		return a.g < b.g
	}
	return a.r < b.r
}

func (m *Map) graph() *regionGraph {
	gr := &m.gr
	if gr.ok {
		return gr
	}
	p := &m.p
	n := p.n
	gr.keys = gr.keys[:0]
	for c := range p.chunks {
		for l := range p.chunks[c].flags {
			gr.keys = append(gr.keys, mkKey(c, l))
		}
	}
	deg := make([]int32, n+1)
	p.eachEdge(func(a, b int32) { deg[a+1]++; deg[b+1]++ })
	for i := 1; i <= n; i++ {
		deg[i] += deg[i-1]
	}
	gr.start = append(gr.start[:0], deg...)
	gr.adj = make([]int32, deg[n])
	gr.mid = make([]uint32, deg[n])
	fill := deg[:n]
	p.eachEdgeMid(func(a, b int32, mid uint32) {
		gr.adj[fill[a]], gr.mid[fill[a]] = b, mid
		fill[a]++
		gr.adj[fill[b]], gr.mid[fill[b]] = a, mid
		fill[b]++
	})
	gr.ok = true
	gr.gen++
	return gr
}

func octile(ax, ay, bx, by int) int32 {
	dx, dy := ax-bx, ay-by
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx < dy {
		dx, dy = dy, dx
	}
	return int32(10*(dx-dy) + 14*dy)
}

// Corridor routes layer l from (sx, sy) to the walkable (tx, ty) over the
// region graph and returns the cells a tile search may use: every region
// on the route and its neighbours open to l (a row-major bitset, valid
// until the next call). ok is false when no route exists.
func (m *Map) Corridor(l Layer, sx, sy, tx, ty int) (allow []uint64, ok bool) {
	w, h := m.p.geo.W, m.p.geo.H
	if uint(sx) >= uint(w) || uint(sy) >= uint(h) {
		return nil, false
	}
	lt := m.Label(l, tx, ty)
	if lt < 0 {
		return nil, false
	}
	gr := m.graph()
	cs := &m.cs
	n := m.p.n
	if len(cs.stamp) < n {
		cs.stamp = make([]uint32, n)
		cs.g = make([]int32, n)
		cs.came = make([]int32, n)
		cs.pos = make([]uint32, n)
		cs.closed = make([]bool, n)
		cs.mark = make([]uint32, n)
		cs.gen, cs.markG = 0, 0
	}
	if cs.gen++; cs.gen == 0 {
		clear(cs.stamp)
		cs.gen = 1
	}
	goal := m.p.regionAt(tx, ty)
	cs.open = cs.open[:0]
	push := func(r, g, from int32, at uint32) {
		if cs.stamp[r] != cs.gen {
			cs.stamp[r], cs.g[r], cs.came[r], cs.closed[r] = cs.gen, 1<<30, -1, false
		}
		if cs.closed[r] || g >= cs.g[r] {
			return
		}
		cs.g[r], cs.came[r], cs.pos[r] = g, from, at
		cs.open = heapPush(cs.open, rnode{f: g + octile(int(at>>16), int(at&0xFFFF), tx, ty), g: g, r: r})
	}
	if m.label1(l, sx, sy) == lt {
		push(m.p.regionAt(sx, sy), 0, -1, uint32(sx)<<16|uint32(sy))
	} else {
		for _, d := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			nx, ny := sx+d[0], sy+d[1]
			if uint(nx) < uint(w) && uint(ny) < uint(h) && m.label1(l, nx, ny) == lt {
				push(m.p.regionAt(nx, ny), octile(sx, sy, nx, ny), -1, uint32(nx)<<16|uint32(ny))
			}
		}
	}
	found := false
	for len(cs.open) > 0 {
		var cur rnode
		cs.open, cur = heapPop(cs.open)
		r := cur.r
		if cs.closed[r] {
			continue
		}
		if r == goal {
			found = true
			break
		}
		cs.closed[r] = true
		rx, ry := int(cs.pos[r]>>16), int(cs.pos[r]&0xFFFF)
		for k := gr.start[r]; k < gr.start[r+1]; k++ {
			nb := gr.adj[k]
			if m.label[l][nb] != lt {
				continue
			}
			mid := gr.mid[k]
			push(nb, cur.g+octile(rx, ry, int(mid>>16), int(mid&0xFFFF)), r, mid)
		}
	}
	if !found {
		return nil, false
	}
	// the corridor: route regions and their open neighbours
	words := (w*h + 63) / 64
	if len(cs.allow) != words {
		cs.allow = make([]uint64, words)
	} else {
		clear(cs.allow) // a few µs even at 500², less than unmarking region by region
	}
	cs.marked = cs.marked[:0]
	if cs.markG++; cs.markG == 0 {
		clear(cs.mark)
		cs.markG = 1
	}
	mark := func(r int32) {
		if cs.mark[r] != cs.markG {
			cs.mark[r] = cs.markG
			cs.marked = append(cs.marked, r)
		}
	}
	for r := goal; r >= 0; r = cs.came[r] {
		mark(r)
	}
	for ring := 0; ring < corridorRings; ring++ {
		route := len(cs.marked)
		for _, r := range cs.marked[:route] {
			for _, nb := range gr.adj[gr.start[r]:gr.start[r+1]] {
				if m.label[l][nb] == lt {
					mark(nb)
				}
			}
		}
	}
	for _, r := range cs.marked {
		m.setCells(r, cs.allow)
	}
	return cs.allow, true
}

// setCells sets global region r's cells in the row-major bitset allow, a
// chunk row (at most 16 cells, so at most two words) at a time.
func (m *Map) setCells(r int32, allow []uint64) {
	k := m.gr.keys[r]
	c, l := k.chunk(), uint16(k.local())
	w := m.p.geo.W
	x0, y0, x1, y1 := m.p.geo.Bounds(c)
	for y := y0; y < y1; y++ {
		i := y*w + x0
		var row uint64
		for b, v := range m.p.local[i : y*w+x1] {
			if v == l {
				row |= 1 << uint(b)
			}
		}
		if row == 0 {
			continue
		}
		s := uint(i) & 63
		allow[i>>6] |= row << s
		if s+uint(x1-x0) > 64 {
			allow[i>>6+1] |= row >> (64 - s)
		}
	}
}

// eachCell calls fn with every cell of global region r.
func (m *Map) eachCell(r int32, fn func(i int)) {
	k := m.gr.keys[r]
	c, l := k.chunk(), uint16(k.local())
	w := m.p.geo.W
	x0, y0, x1, y1 := m.p.geo.Bounds(c)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if i := y*w + x; m.p.local[i] == l {
				fn(i)
			}
		}
	}
}

func heapPush(h []rnode, n rnode) []rnode {
	h = append(h, n)
	i := len(h) - 1
	for i > 0 {
		p := (i - 1) / 2
		if !h[i].less(h[p]) {
			break
		}
		h[i], h[p] = h[p], h[i]
		i = p
	}
	return h
}

func heapPop(h []rnode) ([]rnode, rnode) {
	top := h[0]
	last := len(h) - 1
	h[0] = h[last]
	h = h[:last]
	i := 0
	for {
		l := 2*i + 1
		if l >= len(h) {
			break
		}
		m := l
		if r := l + 1; r < len(h) && h[r].less(h[l]) {
			m = r
		}
		if !h[m].less(h[i]) {
			break
		}
		h[i], h[m] = h[m], h[i]
		i = m
	}
	return h, top
}
