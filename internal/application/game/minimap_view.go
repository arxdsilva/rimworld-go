// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import "rimworld-go/internal/application/game/mapview"

// Minimap read model (NG-1 §2.12): one byte per tile saying what the
// minimap shows there, rebuilt only when the ground or where plants stand
// changes (the chunk read model's change count, chunk_view.go).

// Minimap cell labels are owned by mapview.
const (
	MinimapTerrain   = mapview.MinimapTerrain
	MinimapPlant     = mapview.MinimapPlant
	MinimapFloor     = mapview.MinimapFloor
	MinimapStructure = mapview.MinimapStructure
	MinimapKind      = mapview.MinimapKind
)

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

// minimapCells adapts native read-model types for mapview's pixel renderer.
func minimapCells(w *WorldView, mw, mh int) []uint8 {
	return mapview.MinimapCells(w.TerrainW, w.Terrain, mw, mh, w.Plants, w.Buildings,
		func(p *PlantView) (int, int) { return p.X, p.Y },
		func(b *BuildingView) (int, int, int, int, bool) {
			return b.X, b.Y, b.W, b.H, b.Floor
		},
	)
}
