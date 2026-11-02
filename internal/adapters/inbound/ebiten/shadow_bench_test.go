package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// BenchmarkShadows is one frame's colonist shadow pass (feet and one
// batched draw) with 40 colonists in view, a third of them under a 300
// tile roof, among 960 plants and items and 200 wall tiles that cast
// nothing, at zoom 0.5.
func BenchmarkShadows(b *testing.B) {
	const mw, mh = 40, 25
	w := game.WorldView{TerrainW: mw, Terrain: make([]uint8, mw*mh), RoomGrid: make([]int, mw*mh), RoofCells: make([]uint8, mw*mh), Hour: 12, DayOfYear: 15}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	for i := 0; i < 300; i++ { // a roofed block
		w.RoofCells[(2+i/20)*mw+10+i%20] = overlay.RoofBuilt
	}
	for i := 0; i < 200; i++ { // wall runs
		w.Buildings = append(w.Buildings, game.BuildingView{Kind: "wall", X: i % 40, Y: 18 + i/40, W: 1, H: 1})
	}
	var pawns []game.PawnView
	for i := 0; i < 1000; i++ {
		x, y := (i*7)%mw, (i*3)%mh
		switch {
		case i%25 == 0:
			pawns = append(pawns, game.PawnView{ID: entity.EntityID(i + 1), X: x, Y: y})
		case i%2 == 0:
			w.Plants = append(w.Plants, game.PlantView{Species: "tree", X: x, Y: y, Growth: 80})
		default:
			w.Items = append(w.Items, game.ItemView{Kind: "wood", X: x, Y: y})
		}
	}
	r := New(&stubSim{w: mw, h: mh, world: w})
	r.art = envArt()
	r.cam.Zoom = 0.5
	r.view.Pawns = pawns
	r.lit.Light = testLight(0.5, 0.45, 0.3)
	screen := ebiten.NewImage(1280, 800)
	tile := r.cam.Tile()
	r.drawShadows(screen, w, 1280, 800, tile)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.drawShadows(screen, w, 1280, 800, tile)
	}
	b.ReportMetric(float64(r.env.Shadow.Drawn), "blobs/op")
}
