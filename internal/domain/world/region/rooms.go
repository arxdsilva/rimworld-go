package region

import (
	"slices"

	"rimworld-go/internal/domain/world/room"
)

// Counts are what a room-region (or a whole room) holds by its cells
// alone; buildings inside are counted by the caller.
type Counts struct {
	Cells      int32 // open cells
	Smooth     int32 // cells floored by their own ground
	Walls      int32 // cell sides facing a boundary cell
	StoneWalls int32 // ... of which the boundary counts as stone
}

func (c *Counts) add(o Counts) {
	c.Cells += o.Cells
	c.Smooth += o.Smooth
	c.Walls += o.Walls
	c.StoneWalls += o.StoneWalls
}

// Rooms is the room partition: boundary cells (rock, walls, doors) split
// the map; a room is a component of room-regions, outdoors when it
// touches the map edge. Room ids ascend with each room's smallest cell
// index, which is exactly room.Detect's numbering. Feed it every cell's
// inputs with SetCell; Update brings the rooms up to date.
type Rooms struct {
	p      partition
	stone  []uint64 // boundary cell counts as a stone wall
	smooth []uint64 // open cell floored by its ground
	ck     []roomChunk
	first  bool // no Update yet

	rooms     []roomRec
	regKeys   []key   // rooms' regions, CSR (roomRec.r0..r1)
	prevCells []int32 // per room of the previous Update: cells
	uf        unionFind
	agg       []roomRec // per root scratch
	roots     []int32
	rootRoom  []int32
}

type roomChunk struct {
	counts []Counts // per local region
	edge   []bool   // per local region: touches the map edge
	room   []int32  // per local region: room id or room.Outdoors
	prev   []int32  // room before the last Update (valid unless fresh)
	fresh  bool     // re-flooded by the last Update
}

type roomRec struct {
	min    int32 // smallest cell index
	edge   bool
	counts Counts
	r0, r1 int32 // regions in regKeys
	same   int32 // previous room with exactly these cells, or -1
}

// NewRooms is a w×h room map with every cell open.
func NewRooms(w, h int) *Rooms {
	r := &Rooms{p: newPartition(w, h, false), first: true}
	r.stone = make([]uint64, len(r.p.blocked))
	r.smooth = make([]uint64, len(r.p.blocked))
	r.ck = make([]roomChunk, r.p.geo.Count())
	return r
}

func bit(s []uint64, i int) bool { return s[i>>6]&(1<<(uint(i)&63)) != 0 }

func setBit(s []uint64, i int, on bool) bool {
	w, b := &s[i>>6], uint64(1)<<(uint(i)&63)
	if (*w&b != 0) == on {
		return false
	}
	*w ^= b
	return true
}

// SetCell sets cell (x, y)'s inputs: a room boundary, a boundary that
// counts as stone, open ground that counts as floored. It reports whether
// the boundary changed.
func (r *Rooms) SetCell(x, y int, boundary, stone, smooth bool) bool {
	i := y*r.p.geo.W + x
	c := r.p.geo.Of(x, y)
	bChanged := setBit(r.p.blocked, i, boundary)
	sChanged := setBit(r.stone, i, stone)
	if setBit(r.smooth, i, smooth) {
		r.p.markDirty(c)
	}
	if bChanged || sChanged {
		// the cell's sides count for its neighbours' regions too
		r.p.markDirty(c)
		w, h := r.p.geo.W, r.p.geo.H
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if nx, ny := x+d[0], y+d[1]; nx >= 0 && ny >= 0 && nx < w && ny < h {
				if nc := r.p.geo.Of(nx, ny); nc != c {
					r.p.markDirty(nc)
				}
			}
		}
	}
	return bChanged
}

// Update re-floods the dirty chunks and rebuilds the rooms; it reports
// whether anything changed. Same and EachChanged then describe the change
// since the previous Update (none when it reports false).
func (r *Rooms) Update() bool {
	if !r.p.refresh() {
		if !r.first { // nothing changed since: every room is itself
			for c := range r.ck {
				ck := &r.ck[c]
				ck.fresh = false
				ck.prev = append(ck.prev[:0], ck.room...)
			}
			r.prevCells = r.prevCells[:0]
			for id := range r.rooms {
				r.rooms[id].same = int32(id)
				r.prevCells = append(r.prevCells, r.rooms[id].counts.Cells)
			}
		}
		return false
	}
	for c := range r.ck {
		ck := &r.ck[c]
		ck.fresh = r.first || r.p.flooded.Has(c)
		if ck.fresh {
			r.count(c)
			ck.prev = ck.prev[:0]
		} else {
			ck.prev = append(ck.prev[:0], ck.room...)
		}
	}
	r.prevCells = r.prevCells[:0]
	for _, rm := range r.rooms {
		r.prevCells = append(r.prevCells, rm.counts.Cells)
	}
	r.assemble()
	r.first = false
	return true
}

// count recomputes chunk c's per-region cell counts.
func (r *Rooms) count(c int) {
	p := &r.p
	w, h := p.geo.W, p.geo.H
	n := len(p.chunks[c].flags)
	ck := &r.ck[c]
	ck.counts = slices.Grow(ck.counts[:0], n)[:n]
	clear(ck.counts)
	ck.edge = slices.Grow(ck.edge[:0], n)[:n]
	clear(ck.edge)
	x0, y0, x1, y1 := p.geo.Bounds(c)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*w + x
			l := p.local[i]
			if l == noLocal {
				continue
			}
			cnt := &ck.counts[l]
			cnt.Cells++
			if bit(r.smooth, i) {
				cnt.Smooth++
			}
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				ck.edge[l] = true
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h {
					continue
				}
				if j := ny*w + nx; p.isBlocked(j) {
					cnt.Walls++
					if bit(r.stone, j) {
						cnt.StoneWalls++
					}
				}
			}
		}
	}
}

// assemble joins room-regions into rooms and numbers them.
func (r *Rooms) assemble() {
	p := &r.p
	n := p.n
	r.uf = slices.Grow(r.uf[:0], n)[:n]
	for i := range r.uf {
		r.uf[i] = int32(i)
	}
	p.eachEdge(r.uf.union)
	r.agg = slices.Grow(r.agg[:0], n)[:n]
	for i := range r.agg {
		r.agg[i] = roomRec{min: -1}
	}
	for c := range p.chunks {
		cr := &p.chunks[c]
		for l := range cr.flags {
			root := r.uf.find(p.base[c] + int32(l))
			a := &r.agg[root]
			if a.min < 0 || cr.minCell[l] < a.min {
				a.min = cr.minCell[l]
			}
			a.edge = a.edge || r.ck[c].edge[l]
			a.counts.add(r.ck[c].counts[l])
			a.r1++ // regions in the component
		}
	}
	r.roots = r.roots[:0]
	for root := range r.agg {
		if r.uf[root] == int32(root) && !r.agg[root].edge {
			r.roots = append(r.roots, int32(root))
		}
	}
	slices.SortFunc(r.roots, func(a, b int32) int { return int(r.agg[a].min - r.agg[b].min) })
	r.rootRoom = slices.Grow(r.rootRoom[:0], n)[:n]
	for i := range r.rootRoom {
		r.rootRoom[i] = room.Outdoors
	}
	r.rooms = r.rooms[:0]
	off := int32(0)
	for id, root := range r.roots {
		r.rootRoom[root] = int32(id)
		a := r.agg[root]
		r.rooms = append(r.rooms, roomRec{min: a.min, counts: a.counts, r0: off, r1: off, same: -1})
		off += a.r1
	}
	r.regKeys = slices.Grow(r.regKeys[:0], int(off))[:off]
	for c := range p.chunks {
		ck := &r.ck[c]
		nl := len(p.chunks[c].flags)
		ck.room = slices.Grow(ck.room[:0], nl)[:nl]
		for l := 0; l < nl; l++ {
			id := r.rootRoom[r.uf.find(p.base[c]+int32(l))]
			ck.room[l] = id
			if id >= 0 {
				rm := &r.rooms[id]
				r.regKeys[rm.r1] = mkKey(c, l)
				rm.r1++
			}
		}
	}
	if r.first {
		return
	}
	// a room is the same as an old one when every region of it kept its
	// cells and its room, and the old room had no other cells
	for id := range r.rooms {
		rm := &r.rooms[id]
		same := int32(-1)
		for k, rk := range r.regKeys[rm.r0:rm.r1] {
			ck := &r.ck[rk.chunk()]
			if ck.fresh {
				same = -1
				break
			}
			pr := ck.prev[rk.local()]
			if pr < 0 || k > 0 && pr != same {
				same = -1
				break
			}
			same = pr
		}
		if same >= 0 && int(same) < len(r.prevCells) && r.prevCells[same] == rm.counts.Cells {
			rm.same = same
		}
	}
}

// Width and Height are the map's size in tiles.
func (r *Rooms) Width() int { return r.p.geo.W }

// Height is the map's height in tiles.
func (r *Rooms) Height() int { return r.p.geo.H }

// Count is the number of (enclosed) rooms.
func (r *Rooms) Count() int { return len(r.rooms) }

// At is cell (x, y)'s room id, room.Outdoors or room.Wall, as room.Detect
// would answer.
func (r *Rooms) At(x, y int) int {
	l := r.p.local[y*r.p.geo.W+x]
	if l == noLocal {
		return room.Wall
	}
	return int(r.ck[r.p.geo.Of(x, y)].room[l])
}

// Counts is room id's cell counts.
func (r *Rooms) Counts(id int) Counts { return r.rooms[id].counts }

// Same is the room of the previous Update with exactly room id's cells,
// or -1 (a new or changed room; always -1 after the first Update).
func (r *Rooms) Same(id int) int { return int(r.rooms[id].same) }

// EachCell calls fn with every cell index of room id.
func (r *Rooms) EachCell(id int, fn func(i int)) {
	rm := &r.rooms[id]
	for _, k := range r.regKeys[rm.r0:rm.r1] {
		r.eachRegionCell(k.chunk(), k.local(), fn)
	}
}

func (r *Rooms) eachRegionCell(c, l int, fn func(i int)) {
	w := r.p.geo.W
	x0, y0, x1, y1 := r.p.geo.Bounds(c)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if i := y*w + x; int(r.p.local[i]) == l {
				fn(i)
			}
		}
	}
}

// EachChanged calls fn with every cell whose At answer may differ from the
// previous Update's: all cells of re-flooded chunks, and the cells of
// regions whose room id changed. After the first Update it is every cell.
func (r *Rooms) EachChanged(fn func(i int)) {
	w := r.p.geo.W
	for c := range r.ck {
		ck := &r.ck[c]
		x0, y0, x1, y1 := r.p.geo.Bounds(c)
		if ck.fresh {
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					fn(y*w + x)
				}
			}
			continue
		}
		for l, id := range ck.room {
			if id != ck.prev[l] {
				r.eachRegionCell(c, l, fn)
			}
		}
	}
}
