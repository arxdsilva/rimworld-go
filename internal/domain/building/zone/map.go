package zone

import "sort"

// Map is the per-tile zone layer (ZONE-001): every tile belongs to at most
// one zone, and a zone is one 4-connected region of tiles of one kind.
// Painting skips tiles already zoned, merges the new tiles with touching
// zones of the same kind (grow zones: the same crop) and clearing splits a
// zone cut in two. All decisions run in id or row-major order, so the map
// is deterministic; per-zone cell lists keep lookups off the whole grid.

// Kind is what a zone is for.
type Kind uint8

// Zone kinds (0 is no kind).
const (
	KindStockpile Kind = iota + 1
	KindGrow
	KindPen
)

// ID names a zone; 0 is no zone.
type ID uint16

// Zone is one zone's settings. Cells is its tile count (read-only: Map
// keeps it).
type Zone struct {
	ID    ID
	Kind  Kind
	Crop  uint8 // grow zones: plant.Species
	Food  bool  // stockpile filters
	Wood  bool
	Stone bool
	Cells int
}

// compatible: two zones of these settings merge when they touch.
func compatible(a, b Zone) bool {
	return a.Kind == b.Kind && (a.Kind != KindGrow || a.Crop == b.Crop)
}

// Rect is an inclusive tile rectangle (build it with Normalize).
type Rect struct{ MinX, MinY, MaxX, MaxY int }

// RectOf normalizes two corners into a Rect.
func RectOf(x1, y1, x2, y2 int) Rect {
	minX, minY, maxX, maxY := Normalize(x1, y1, x2, y2)
	return Rect{minX, minY, maxX, maxY}
}

type rec struct {
	z    Zone
	idx  []int // row-major tile indexes, sorted
	box  Rect
	dirt bool
}

// Map is the zone layer of one w x h map.
type Map struct {
	w, h  int
	cell  []ID
	zones []*rec // by ID (nil: none); ids are small and never reused
	ids   []ID   // live zone ids, sorted
	next  ID     // last id handed out; ids are never reused
}

// NewMap is an empty zone layer.
func NewMap(w, h int) *Map {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &Map{w: w, h: h, cell: make([]ID, w*h)}
}

// rec is zone id's record, nil when there is none.
func (m *Map) rec(id ID) *rec {
	if int(id) < len(m.zones) {
		return m.zones[id]
	}
	return nil
}

// setRec records r as zone id (nil forgets it).
func (m *Map) setRec(id ID, r *rec) {
	if int(id) >= len(m.zones) {
		if r == nil {
			return
		}
		m.zones = append(m.zones, make([]*rec, int(id)+1-len(m.zones))...)
	}
	m.zones[id] = r
}

func (m *Map) in(x, y int) bool { return x >= 0 && y >= 0 && x < m.w && y < m.h }

// At is the zone on tile (x, y).
func (m *Map) At(x, y int) (ID, bool) {
	if !m.in(x, y) {
		return 0, false
	}
	id := m.cell[y*m.w+x]
	return id, id != 0
}

// Zone is zone id's settings.
func (m *Map) Zone(id ID) (Zone, bool) {
	r := m.rec(id)
	if r == nil {
		return Zone{}, false
	}
	return r.z, true
}

// Zones lists every zone by id.
func (m *Map) Zones() []Zone {
	out := make([]Zone, len(m.ids))
	for i, id := range m.ids {
		out[i] = m.rec(id).z
	}
	return out
}

// Each calls fn for every zone in id order until fn returns false.
func (m *Map) Each(fn func(Zone) bool) {
	for _, id := range m.ids {
		if !fn(m.rec(id).z) {
			return
		}
	}
}

// Cells lists zone id's tiles, row-major.
func (m *Map) Cells(id ID) [][2]int {
	r := m.rec(id)
	if r == nil {
		return nil
	}
	out := make([][2]int, len(r.idx))
	for i, c := range r.idx {
		out[i] = [2]int{c % m.w, c / m.w}
	}
	return out
}

// EachCell calls fn for zone id's tiles in row-major order until fn
// returns false (no allocation).
func (m *Map) EachCell(id ID, fn func(x, y int) bool) {
	r := m.rec(id)
	if r == nil {
		return
	}
	for _, c := range r.idx {
		if !fn(c%m.w, c/m.w) {
			return
		}
	}
}

// Bounds is zone id's bounding box.
func (m *Map) Bounds(id ID) (Rect, bool) {
	r := m.rec(id)
	if r == nil {
		return Rect{}, false
	}
	if r.dirt {
		r.box, r.dirt = m.bounds(r.idx), false
	}
	return r.box, true
}

func (m *Map) bounds(idx []int) Rect {
	b := Rect{MinX: m.w, MinY: m.h, MaxX: -1, MaxY: -1}
	for _, c := range idx {
		x, y := c%m.w, c/m.w
		b.MinX, b.MaxX = min(b.MinX, x), max(b.MaxX, x)
		b.MinY, b.MaxY = min(b.MinY, y), max(b.MaxY, y)
	}
	return b
}

// clip clamps r to the map; ok is false when nothing is left.
func (m *Map) clip(r Rect) (Rect, bool) {
	r.MinX, r.MinY = max(r.MinX, 0), max(r.MinY, 0)
	r.MaxX, r.MaxY = min(r.MaxX, m.w-1), min(r.MaxY, m.h-1)
	return r, r.MinX <= r.MaxX && r.MinY <= r.MaxY
}

func (m *Map) newZone(z Zone) *rec {
	m.next++
	z.ID, z.Cells = m.next, 0
	r := &rec{z: z, dirt: true}
	m.setRec(z.ID, r)
	i := sort.Search(len(m.ids), func(i int) bool { return m.ids[i] >= z.ID })
	m.ids = append(m.ids, 0)
	copy(m.ids[i+1:], m.ids[i:])
	m.ids[i] = z.ID
	return r
}

func (m *Map) drop(id ID) {
	m.setRec(id, nil)
	i := sort.Search(len(m.ids), func(i int) bool { return m.ids[i] >= id })
	if i < len(m.ids) && m.ids[i] == id {
		m.ids = append(m.ids[:i], m.ids[i+1:]...)
	}
}

// absorb moves every tile of zone from into zone into.
func (m *Map) absorb(into, from ID) {
	dst, src := m.rec(into), m.rec(from)
	for _, c := range src.idx {
		m.cell[c] = into
	}
	dst.idx = append(dst.idx, src.idx...)
	sort.Ints(dst.idx)
	dst.z.Cells, dst.dirt = len(dst.idx), true
	m.drop(from)
}

// Paint zones the free tiles of r that pass ok (nil: all) as kind with
// proto's settings. Painted tiles join every touching zone they are
// compatible with (same kind; grow zones also the same crop), merging those
// zones into the lowest id, which keeps its settings; painted tiles that
// touch none start a new zone with proto's settings. Tiles of other zones
// are skipped. Paint returns the zone holding the first painted tile, 0
// when nothing was painted.
func (m *Map) Paint(kind Kind, proto Zone, r Rect, ok func(x, y int) bool) ID {
	if kind == 0 {
		return 0
	}
	proto.Kind = kind
	r, inside := m.clip(r)
	if !inside {
		return 0
	}
	// claim: the free tiles, row-major, marked in a local set
	var claimed []int
	mark := map[int]int{} // tile -> component index (-1: unvisited)
	for y := r.MinY; y <= r.MaxY; y++ {
		for x := r.MinX; x <= r.MaxX; x++ {
			c := y*m.w + x
			if m.cell[c] != 0 || ok != nil && !ok(x, y) {
				continue
			}
			claimed = append(claimed, c)
			mark[c] = -1
		}
	}
	if len(claimed) == 0 {
		return 0
	}
	// components of the painted tiles and the compatible zones they touch
	type comp struct {
		cells []int
		ids   []ID
	}
	var comps []comp
	for _, start := range claimed {
		if mark[start] >= 0 {
			continue
		}
		ci := len(comps)
		cp := comp{}
		mark[start] = ci
		stack := []int{start}
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			cp.cells = append(cp.cells, c)
			x, y := c%m.w, c/m.w
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if !m.in(nx, ny) {
					continue
				}
				n := ny*m.w + nx
				if v, mine := mark[n]; mine {
					if v < 0 {
						mark[n] = ci
						stack = append(stack, n)
					}
					continue
				}
				if id := m.cell[n]; id != 0 && compatible(m.rec(id).z, proto) {
					cp.ids = appendID(cp.ids, id)
				}
			}
		}
		comps = append(comps, cp)
	}
	// components touching the same zone join; a zone lists its components
	parent := make([]int, len(comps))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	owner := map[ID]int{}
	for i, cp := range comps {
		for _, id := range cp.ids {
			if j, seen := owner[id]; seen {
				a, b := find(i), find(j)
				if a != b {
					parent[max(a, b)] = min(a, b)
				}
			} else {
				owner[id] = i
			}
		}
	}
	// each group, in order of its first painted tile, lands in one zone
	first := ID(0)
	done := map[int]bool{}
	for i := range comps {
		root := find(i)
		if done[root] {
			continue
		}
		done[root] = true
		var cells []int
		var ids []ID
		for j := range comps {
			if find(j) == root {
				cells = append(cells, comps[j].cells...)
				for _, id := range comps[j].ids {
					ids = appendID(ids, id)
				}
			}
		}
		sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
		var target *rec
		if len(ids) == 0 {
			target = m.newZone(proto)
		} else {
			target = m.rec(ids[0])
			for _, id := range ids[1:] {
				m.absorb(ids[0], id)
			}
		}
		for _, c := range cells {
			m.cell[c] = target.z.ID
		}
		target.idx = append(target.idx, cells...)
		sort.Ints(target.idx)
		target.z.Cells, target.dirt = len(target.idx), true
		if first == 0 {
			first = target.z.ID
		}
	}
	return first
}

func appendID(ids []ID, id ID) []ID {
	for _, o := range ids {
		if o == id {
			return ids
		}
	}
	return append(ids, id)
}

// Clear unzones every tile of r. A zone cut into pieces splits: the piece
// holding its first tile (row-major) keeps the id, the others become new
// zones with the same settings; emptied zones are deleted.
func (m *Map) Clear(r Rect) {
	r, inside := m.clip(r)
	if !inside {
		return
	}
	var hit []ID
	for y := r.MinY; y <= r.MaxY; y++ {
		for x := r.MinX; x <= r.MaxX; x++ {
			c := y*m.w + x
			if id := m.cell[c]; id != 0 {
				hit = appendID(hit, id)
				m.cell[c] = 0
			}
		}
	}
	sort.Slice(hit, func(a, b int) bool { return hit[a] < hit[b] })
	for _, id := range hit {
		z := m.rec(id)
		kept := z.idx[:0]
		for _, c := range z.idx {
			if m.cell[c] == id {
				kept = append(kept, c)
			}
		}
		z.idx, z.z.Cells, z.dirt = kept, len(kept), true
		if len(kept) == 0 {
			m.drop(id)
			continue
		}
		m.split(id)
	}
}

// split breaks zone id into its 4-connected pieces.
func (m *Map) split(id ID) {
	z := m.rec(id)
	piece := m.flood(id, z.idx[0])
	if len(piece) == len(z.idx) {
		return
	}
	in := make(map[int]bool, len(piece))
	for _, c := range piece {
		in[c] = true
	}
	var rest []int
	for _, c := range z.idx {
		if !in[c] {
			rest = append(rest, c)
		}
	}
	sort.Ints(piece)
	z.idx, z.z.Cells, z.dirt = piece, len(piece), true
	// the rest moves to a new zone (row-major first tile first), then
	// splits again if it is itself in pieces
	nz := m.newZone(z.z)
	for _, c := range rest {
		m.cell[c] = nz.z.ID
	}
	nz.idx, nz.z.Cells = rest, len(rest)
	m.split(nz.z.ID)
}

// flood lists the tiles of zone id 4-connected to tile start.
func (m *Map) flood(id ID, start int) []int {
	seen := map[int]bool{start: true}
	out := []int{}
	stack := []int{start}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, c)
		x, y := c%m.w, c/m.w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if !m.in(nx, ny) {
				continue
			}
			n := ny*m.w + nx
			if m.cell[n] == id && !seen[n] {
				seen[n] = true
				stack = append(stack, n)
			}
		}
	}
	return out
}

// Set changes zone id's settings (filters, crop); its id, kind and tiles
// stay. A grow zone switched to the crop of a touching zone merges with it
// (the lower id survives).
func (m *Map) Set(id ID, z Zone) {
	r := m.rec(id)
	if r == nil {
		return
	}
	z.ID, z.Kind, z.Cells = r.z.ID, r.z.Kind, r.z.Cells
	r.z = z
	var ids []ID
	for _, c := range r.idx {
		x, y := c%m.w, c/m.w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if nid, ok := m.At(x+d[0], y+d[1]); ok && nid != id && compatible(m.rec(nid).z, z) {
				ids = appendID(ids, nid)
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	ids = append(ids, id)
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	for _, o := range ids[1:] {
		m.absorb(ids[0], o)
	}
}

// Saved is a Map as plain data.
type Saved struct {
	W, H  int
	Cells []ID
	Zones []Zone
	Next  ID
}

// Export copies the map out for saving.
func (m *Map) Export() Saved {
	return Saved{W: m.w, H: m.h, Cells: append([]ID(nil), m.cell...), Zones: m.Zones(), Next: m.next}
}

// Import restores an exported map. Tiles naming an unknown zone, and
// zones without tiles, are dropped.
func Import(s Saved) *Map {
	m := NewMap(s.W, s.H)
	m.next = s.Next
	for _, z := range s.Zones {
		if z.ID == 0 || z.Kind == 0 || m.rec(z.ID) != nil {
			continue
		}
		m.setRec(z.ID, &rec{z: z, dirt: true})
		m.ids = append(m.ids, z.ID)
		if z.ID > m.next {
			m.next = z.ID
		}
	}
	sort.Slice(m.ids, func(a, b int) bool { return m.ids[a] < m.ids[b] })
	for c := 0; c < len(m.cell) && c < len(s.Cells); c++ {
		id := s.Cells[c]
		r := m.rec(id)
		if r == nil {
			continue
		}
		m.cell[c] = id
		r.idx = append(r.idx, c)
	}
	for _, id := range append([]ID(nil), m.ids...) {
		r := m.rec(id)
		r.z.Cells = len(r.idx)
		if r.z.Cells == 0 {
			m.drop(id)
		}
	}
	return m
}
