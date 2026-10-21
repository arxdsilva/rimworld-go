package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// BenchmarkShadows is one frame's shadow pass (casters, layer, roof clip
// and composite) with 1000 casters, 200 wall tiles and 300 roofed tiles
// in view at zoom 0.5. ART-010 W5 budget: at most 0.6 ms/op (CPU side).
func BenchmarkShadows(b *testing.B) {
	const mw, mh = 40, 25
	w := game.WorldView{TerrainW: mw, Terrain: make([]uint8, mw*mh), RoomGrid: make([]int, mw*mh), RoofCells: make([]uint8, mw*mh)}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	for i := 0; i < 300; i++ { // a roofed block
		w.RoofCells[(2+i/20)*mw+10+i%20] = roofBuilt
	}
	for i := 0; i < 200; i++ { // wall runs
		w.Buildings = append(w.Buildings, game.BuildingView{Kind: "wall", X: i % 40, Y: 18 + i/40, W: 1, H: 1})
	}
	var pawns []game.PawnView
	for i := 0; i < 1000; i++ {
		x, y := (i*7)%mw, (i*3)%mh
		switch {
		case i%10 == 0:
			pawns = append(pawns, game.PawnView{X: x, Y: y})
		case i%2 == 0:
			w.Plants = append(w.Plants, game.PlantView{Species: "tree", X: x, Y: y, Growth: 80})
		default:
			w.Items = append(w.Items, game.ItemView{Kind: "wood", X: x, Y: y})
		}
	}
	r := New(&stubSim{w: mw, h: mh})
	if root, err := LocateArt(); err == nil {
		if art, err := loadArtCatalog(root); err == nil {
			r.art = art
		}
	}
	if r.art == nil {
		r.art = envArt()
	}
	r.cam.zoom = 0.5
	r.drawPawns = pawns
	r.lit.light = testLight(0.5, 0.45, 0.3)
	screen := ebiten.NewImage(1280, 800)
	tile := r.tilePx()
	r.drawShadows(screen, w, 1280, 800, tile)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.drawShadows(screen, w, 1280, 800, tile)
	}
}
