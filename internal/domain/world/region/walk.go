package region

// Layer is a movement layer: which special cells it may not enter, on top
// of the walkability grid.
type Layer uint8

// Movement layers.
const (
	Colonist Layer = iota // the grid: doors, fences, gates, traps open
	Hostile               // doors shut (raiders)
	Sapper                // doors and traps shut
	Tame                  // fences shut; doors and gates open
	Wild                  // doors, fences and gates shut
	Layers                // number of layers
)

var layerBlocks = [Layers]Flags{
	Colonist: 0,
	Hostile:  Door,
	Sapper:   Door | Trap,
	Tame:     Fence,
	Wild:     Door | Fence | Gate,
}

// Blocks is the set of flags layer l may not enter.
func (l Layer) Blocks() Flags { return layerBlocks[l] }

// Map is the walkability region map. Feed it the grid's blocked bits
// (SetBlockedWords) and the special cells' flags (SetFlags); queries
// bring it up to date first.
type Map struct {
	p      partition
	label  [Layers][]int32 // per global region: component, or -1 (shut)
	stale  bool
	rflags []Flags
	gr     regionGraph     // corridor.go
	cs     corridorScratch // corridor.go
}

// New is a w×h map with every cell open and unflagged.
func New(w, h int) *Map {
	return &Map{p: newPartition(w, h, true), stale: true}
}

// Width and Height are the map's size in tiles.
func (m *Map) Width() int { return m.p.geo.W }

// Height is the map's height in tiles.
func (m *Map) Height() int { return m.p.geo.H }

// SetBlockedWords diffs a blocked-cell bitset (bit i set: cell i not
// walkable, as grid.BlockedBits) against the map's copy.
func (m *Map) SetBlockedWords(words []uint64) { m.p.setBlockedWords(words) }

// SetBlocked sets one cell's walkability.
func (m *Map) SetBlocked(x, y int, blocked bool) { m.p.setBlocked(y*m.p.geo.W+x, blocked) }

// SetFlags sets one cell's special flags.
func (m *Map) SetFlags(x, y int, f Flags) { m.p.setFlags(y*m.p.geo.W+x, f) }

// FlagsAt is cell (x, y)'s flags.
func (m *Map) FlagsAt(x, y int) Flags { return m.p.flags[y*m.p.geo.W+x] }

// Update re-floods dirty chunks and relabels the layers. Queries call it.
func (m *Map) Update() {
	if m.p.refresh() {
		m.stale = true
		m.gr.ok = false
	}
	if m.stale {
		m.relabel()
		m.stale = false
	}
}

// relabel computes each layer's components over the region graph by
// union-find; a component's label is its smallest region id.
func (m *Map) relabel() {
	p := &m.p
	n := p.n
	m.rflags = m.rflags[:0]
	for c := range p.chunks {
		m.rflags = append(m.rflags, p.chunks[c].flags...)
	}
	for l := Layer(0); l < Layers; l++ {
		if cap(m.label[l]) < n {
			m.label[l] = make([]int32, n)
		}
		u := unionFind(m.label[l][:n])
		bl := l.Blocks()
		for r := range u {
			if m.rflags[r]&bl != 0 {
				u[r] = -1
			} else {
				u[r] = int32(r)
			}
		}
		p.eachEdge(func(a, b int32) {
			if u[a] >= 0 && u[b] >= 0 {
				u.union(a, b)
			}
		})
		for r := range u {
			if u[r] >= 0 {
				u[r] = u.find(int32(r))
			}
		}
		m.label[l] = u
	}
}

// Label is cell (x, y)'s component in layer l: -1 when the cell is off
// the map, blocked, or shut to the layer. Two cells with the same label
// >= 0 are connected by a walk of 4-adjacent open cells.
func (m *Map) Label(l Layer, x, y int) int32 {
	if uint(x) >= uint(m.p.geo.W) || uint(y) >= uint(m.p.geo.H) {
		return -1
	}
	if m.p.pending || m.stale {
		m.Update()
	}
	return m.label1(l, x, y)
}

// label1 is Label for an on-map cell of an up-to-date map.
func (m *Map) label1(l Layer, x, y int) int32 {
	r := m.p.regionAt(x, y)
	if r < 0 {
		return -1
	}
	return m.label[l][r]
}

// Region is cell (x, y)'s global region id (-1: blocked or off the map).
// Ids change whenever any chunk is re-flooded.
func (m *Map) Region(x, y int) int32 {
	if uint(x) >= uint(m.p.geo.W) || uint(y) >= uint(m.p.geo.H) {
		return -1
	}
	m.Update()
	return m.p.regionAt(x, y)
}

// Reach answers pathing.Find's question without searching: from (sx, sy)
// (which need not be walkable: a mover may stand on a shut cell) can a
// walk reach (tx, ty) in layer l? It is false when the start is off the
// map or the target is not walkable in l, true when they are the same
// cell; otherwise a walkable start reaches its own component, an
// unwalkable one the components of its walkable 4-neighbours (a diagonal
// step off it needs both orthogonals open anyway).
func (m *Map) Reach(l Layer, sx, sy, tx, ty int) bool {
	w, h := m.p.geo.W, m.p.geo.H
	if uint(sx) >= uint(w) || uint(sy) >= uint(h) {
		return false
	}
	lt := m.Label(l, tx, ty)
	if lt < 0 {
		return false
	}
	if sx == tx && sy == ty {
		return true
	}
	if ls := m.label1(l, sx, sy); ls >= 0 {
		return ls == lt
	}
	for _, d := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
		nx, ny := sx+d[0], sy+d[1]
		if uint(nx) < uint(w) && uint(ny) < uint(h) && m.label1(l, nx, ny) == lt {
			return true
		}
	}
	return false
}

// Regions is the number of regions (after Update).
func (m *Map) Regions() int {
	m.Update()
	return m.p.n
}
