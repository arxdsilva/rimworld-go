package game

// Minimap read model (NG-1 §2.12): one byte per tile saying what the
// minimap shows there, rebuilt only when the ground or where plants stand
// changes (the chunk read model's change count, chunk_view.go).

// Minimap cell bits above the terrain code (low 6 bits, indexing
// TerrainNames()).
const (
	MinimapTerrain   uint8 = 0x3f // mask of the terrain code
	MinimapPlant     uint8 = 0x40 // a plant stands here
	MinimapFloor     uint8 = 0x80 // a built floor
	MinimapStructure uint8 = 0xc0 // a building other than a floor
	MinimapKind      uint8 = 0xc0 // mask of the plant/floor/structure bits
)

// MinimapView is the minimap layer: Cells is W×H, row-major, each a
// terrain code with one of the Minimap* kinds; Ver changes whenever any
// cell may have. Cells is shared and read-only.
type MinimapView struct {
	Ver   uint64
	W, H  int
	Cells []uint8
}

type minimapState struct {
	gen   uint64
	ok    bool
	cells []uint8
}

// MinimapLayer is the minimap layer, or (a view holding only the current
// Ver, false) when it has not changed since the caller's ver.
func (g *Game) MinimapLayer(ver uint64) (MinimapView, bool) {
	s := g.chunks()
	if s.MinimapVer() == ver {
		return MinimapView{Ver: ver}, false
	}
	w := g.World()
	return s.Minimap(&w, ver)
}

// MinimapVer is the minimap layer's current version (never 0, so a
// caller's zero always fetches).
func (s *ChunkIndex) MinimapVer() uint64 { return s.gen + 1 }

// Minimap is Game.MinimapLayer over w, the view s was last updated with.
func (s *ChunkIndex) Minimap(w *WorldView, ver uint64) (MinimapView, bool) {
	m := &s.mini
	cur := s.MinimapVer()
	if ver == cur && m.ok {
		return MinimapView{Ver: cur}, false
	}
	if !m.ok || m.gen != cur || len(m.cells) != s.grid.W*s.grid.H {
		m.cells = minimapCells(w, s.grid.W, s.grid.H)
		m.gen, m.ok = cur, true
	}
	return MinimapView{Ver: cur, W: s.grid.W, H: s.grid.H, Cells: m.cells}, true
}

// minimapCells builds a fresh layer (never written once handed out).
func minimapCells(w *WorldView, mw, mh int) []uint8 {
	out := make([]uint8, mw*mh)
	if w.TerrainW == mw {
		for i := range out {
			if i < len(w.Terrain) {
				out[i] = w.Terrain[i] & MinimapTerrain
			}
		}
	}
	set := func(x, y int, k uint8) {
		if x >= 0 && y >= 0 && x < mw && y < mh {
			i := y*mw + x
			if out[i]&MinimapKind < k || k == MinimapStructure {
				out[i] = out[i]&MinimapTerrain | k
			}
		}
	}
	for i := range w.Plants {
		set(w.Plants[i].X, w.Plants[i].Y, MinimapPlant)
	}
	for i := range w.Buildings {
		b := &w.Buildings[i]
		k := MinimapStructure
		if b.Floor {
			k = MinimapFloor
		}
		for dy := 0; dy < max(b.H, 1); dy++ {
			for dx := 0; dx < max(b.W, 1); dx++ {
				set(b.X+dx, b.Y+dy, k)
			}
		}
	}
	return out
}
