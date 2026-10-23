package region

import (
	"math/bits"
	"slices"

	"rimworld-go/internal/domain/world/chunk"
)

// Flags mark a special cell: each flagged open cell is a region of its
// own, so movement layers can shut it.
type Flags uint8

// Cell flags.
const (
	Door  Flags = 1 << iota // a door: shut to raiders and wild animals
	Fence                   // a fence post: shut to every animal
	Gate                    // a fence gate: shut to wild animals
	Trap                    // a spike trap: shut to sappers
)

const (
	noLocal    = 0xFFFF // a blocked cell
	unassigned = 0xFFFE // flood scratch
)

// key names a region by its chunk and chunk-local index; it survives the
// renumbering of other chunks' regions.
type key uint32

func mkKey(c, local int) key { return key(uint32(c)<<16 | uint32(local)) }
func (k key) chunk() int     { return int(k >> 16) }
func (k key) local() int     { return int(k & 0xFFFF) }

// chunkRegions is one chunk's regions and the region-graph edges it owns:
// those from its cells to their east and south neighbours.
type chunkRegions struct {
	flags   []Flags  // per local region
	minCell []int32  // per local region: smallest cell index
	center  []uint32 // per local region: its cells' mean position, x<<16 | y
	edges   []uint64 // key pairs (a<<32 | b), sorted, unique
	mid     []uint32 // per edge: the shared border's middle tile, x<<16 | y
}

// partition is the per-chunk flood of open cells (bit clear in blocked).
// A cell with nonzero flags is a 1-cell region.
type partition struct {
	geo     chunk.Grid
	blocked []uint64
	flags   []Flags // nil: no special cells
	local   []uint16
	chunks  []chunkRegions
	dirty   chunk.Set
	pending bool // some chunk is dirty
	relink  chunk.Set
	flooded chunk.Set // chunks the last refresh re-flooded
	stack   []int32
	linkTmp []edgeCell

	// global numbering (renumbered by number())
	base []int32 // per chunk: global id of its local region 0
	n    int     // regions
}

func newPartition(w, h int, withFlags bool) partition {
	geo := chunk.New(w, h)
	p := partition{
		geo:     geo,
		blocked: make([]uint64, (w*h+63)/64),
		local:   make([]uint16, w*h),
		chunks:  make([]chunkRegions, geo.Count()),
		dirty:   chunk.NewSet(geo.Count()),
		relink:  chunk.NewSet(geo.Count()),
		flooded: chunk.NewSet(geo.Count()),
		base:    make([]int32, geo.Count()+1),
	}
	if withFlags {
		p.flags = make([]Flags, w*h)
	}
	for c := range p.chunks {
		p.markDirty(c)
	}
	return p
}

func (p *partition) markDirty(c int) {
	p.dirty.Add(c)
	p.pending = true
}

func (p *partition) isBlocked(i int) bool { return p.blocked[i>>6]&(1<<(uint(i)&63)) != 0 }

// setBlockedWords diffs words (bit set: blocked) against the current
// copy and marks the chunks of every changed cell dirty.
func (p *partition) setBlockedWords(words []uint64) {
	n := min(len(words), len(p.blocked))
	for wi := 0; wi < n; wi++ {
		x := p.blocked[wi] ^ words[wi]
		if x == 0 {
			continue
		}
		p.blocked[wi] = words[wi]
		for x != 0 {
			b := bits.TrailingZeros64(x)
			x &= x - 1
			if i := wi<<6 + b; i < len(p.local) {
				p.markDirty(p.geo.OfIndex(i))
			}
		}
	}
	// bits past the map in the last word are never read
}

// setBlocked sets one cell's blocked bit, marking its chunk dirty on a
// change.
func (p *partition) setBlocked(i int, blocked bool) {
	w, bit := &p.blocked[i>>6], uint64(1)<<(uint(i)&63)
	if (*w&bit != 0) == blocked {
		return
	}
	*w ^= bit
	p.markDirty(p.geo.OfIndex(i))
}

// setFlags sets one cell's flags, marking its chunk dirty on a change.
func (p *partition) setFlags(i int, f Flags) {
	if p.flags[i] == f {
		return
	}
	p.flags[i] = f
	p.markDirty(p.geo.OfIndex(i))
}

// refresh re-floods and relinks the dirty chunks in index order and
// reports whether anything changed.
func (p *partition) refresh() bool {
	if !p.pending {
		return false
	}
	p.pending = false
	p.flooded.Clear()
	p.dirty.Each(func(c int) {
		p.flooded.Add(c)
		p.flood(c)
		p.relink.Add(c)
		p.relink.Add(p.geo.Left(c))
		p.relink.Add(p.geo.Up(c))
	})
	p.dirty.Clear()
	p.relink.Each(p.link)
	p.relink.Clear()
	p.number()
	return true
}

// flood partitions chunk c's open cells into 4-connected regions, in
// row-major order of each region's first cell.
func (p *partition) flood(c int) {
	w := p.geo.W
	x0, y0, x1, y1 := p.geo.Bounds(c)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*w + x
			if p.isBlocked(i) {
				p.local[i] = noLocal
			} else {
				p.local[i] = unassigned
			}
		}
	}
	cr := &p.chunks[c]
	cr.flags, cr.minCell, cr.center = cr.flags[:0], cr.minCell[:0], cr.center[:0]
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*w + x
			if p.local[i] != unassigned {
				continue
			}
			id := uint16(len(cr.flags))
			cr.minCell = append(cr.minCell, int32(i))
			p.local[i] = id
			if p.flags != nil && p.flags[i] != 0 {
				cr.flags = append(cr.flags, p.flags[i])
				cr.center = append(cr.center, uint32(x)<<16|uint32(y))
				continue
			}
			cr.flags = append(cr.flags, 0)
			p.stack = append(p.stack[:0], int32(i))
			sx, sy, n := 0, 0, 0
			for len(p.stack) > 0 {
				j := int(p.stack[len(p.stack)-1])
				p.stack = p.stack[:len(p.stack)-1]
				jx, jy := j%w, j/w
				sx, sy, n = sx+jx, sy+jy, n+1
				if jx > x0 {
					p.claim(j-1, id)
				}
				if jx+1 < x1 {
					p.claim(j+1, id)
				}
				if jy > y0 {
					p.claim(j-w, id)
				}
				if jy+1 < y1 {
					p.claim(j+w, id)
				}
			}
			cr.center = append(cr.center, uint32((sx+n/2)/n)<<16|uint32((sy+n/2)/n))
		}
	}
}

func (p *partition) claim(j int, id uint16) {
	if p.local[j] != unassigned || p.flags != nil && p.flags[j] != 0 {
		return
	}
	p.local[j] = id
	p.stack = append(p.stack, int32(j))
}

// link rebuilds the edges chunk c owns: open cell pairs (a cell and its
// east or south neighbour) in different regions.
func (p *partition) link(c int) {
	w, h := p.geo.W, p.geo.H
	x0, y0, x1, y1 := p.geo.Bounds(c)
	cr := &p.chunks[c]
	tmp := p.linkTmp[:0]
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*w + x
			la := p.local[i]
			if la == noLocal {
				continue
			}
			ka := mkKey(c, int(la))
			if x+1 < w {
				if lb := p.local[i+1]; lb != noLocal {
					cb := c
					if x+1 == x1 {
						cb = c + 1
					}
					if kb := mkKey(cb, int(lb)); kb != ka {
						tmp = append(tmp, edgeCell{uint64(ka)<<32 | uint64(kb), int32(2*x + 1), int32(2 * y)})
					}
				}
			}
			if y+1 < h {
				if lb := p.local[i+w]; lb != noLocal {
					cb := c
					if y+1 == y1 {
						cb = c + p.geo.CW
					}
					if kb := mkKey(cb, int(lb)); kb != ka {
						tmp = append(tmp, edgeCell{uint64(ka)<<32 | uint64(kb), int32(2 * x), int32(2*y + 1)})
					}
				}
			}
		}
	}
	slices.SortFunc(tmp, func(a, b edgeCell) int {
		switch {
		case a.e < b.e:
			return -1
		case a.e > b.e:
			return 1
		}
		return 0
	})
	cr.edges, cr.mid = cr.edges[:0], cr.mid[:0]
	for k := 0; k < len(tmp); {
		e := tmp[k].e
		sx, sy, n := 0, 0, 0
		for ; k < len(tmp) && tmp[k].e == e; k++ {
			sx, sy, n = sx+int(tmp[k].x2), sy+int(tmp[k].y2), n+1
		}
		cr.edges = append(cr.edges, e)
		// the shared border's middle, rounded to a tile
		cr.mid = append(cr.mid, uint32((sx+n)/(2*n))<<16|uint32((sy+n)/(2*n)))
	}
	p.linkTmp = tmp
}

// edgeCell is one border crossing found by link: its region pair and the
// doubled coordinates of the crossing's midpoint.
type edgeCell struct {
	e      uint64
	x2, y2 int32
}

// number assigns global region ids: chunk by chunk, local order within.
func (p *partition) number() {
	n := int32(0)
	for c := range p.chunks {
		p.base[c] = n
		n += int32(len(p.chunks[c].flags))
	}
	p.base[len(p.chunks)] = n
	p.n = int(n)
}

// gid is a region key's global id.
func (p *partition) gid(k key) int32 { return p.base[k.chunk()] + int32(k.local()) }

// regionAt is the global region id of cell (x, y) (on the map), or -1
// when blocked.
func (p *partition) regionAt(x, y int) int32 {
	l := p.local[y*p.geo.W+x]
	if l == noLocal {
		return -1
	}
	return p.base[p.geo.Of(x, y)] + int32(l)
}

// eachEdge calls fn with the global ids of every region-graph edge, chunk
// by chunk.
func (p *partition) eachEdge(fn func(a, b int32)) {
	for c := range p.chunks {
		for _, e := range p.chunks[c].edges {
			fn(p.gid(key(e>>32)), p.gid(key(uint32(e))))
		}
	}
}

// eachEdgeMid is eachEdge with each edge's border middle (x<<16 | y).
func (p *partition) eachEdgeMid(fn func(a, b int32, mid uint32)) {
	for c := range p.chunks {
		cr := &p.chunks[c]
		for k, e := range cr.edges {
			fn(p.gid(key(e>>32)), p.gid(key(uint32(e))), cr.mid[k])
		}
	}
}

// unionFind labels components: parent[r] < 0 marks a region left out.
type unionFind []int32

func (u unionFind) find(a int32) int32 {
	for u[a] != a {
		u[a] = u[u[a]]
		a = u[a]
	}
	return a
}

// union joins a and b; the smaller root wins (labels are each
// component's smallest region id).
func (u unionFind) union(a, b int32) {
	ra, rb := u.find(a), u.find(b)
	switch {
	case ra < rb:
		u[rb] = ra
	case rb < ra:
		u[ra] = rb
	}
}
