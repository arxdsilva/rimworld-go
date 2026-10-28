package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// BenchmarkShadows is one frame's shadow pass (casters, layer, roof clip,
// blur and composite) with 1000 casters (40 of them pawns casting their
// silhouette), 200 wall tiles and 300 roofed tiles in view at zoom 0.5.
// ART-012 budget: at most 0.9 ms/op (CPU side).
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
		case i%25 == 0:
			pawns = append(pawns, game.PawnView{ID: entity.EntityID(i + 1), X: x, Y: y})
		case i%2 == 0:
			w.Plants = append(w.Plants, game.PlantView{Species: "tree", X: x, Y: y, Growth: 80})
		default:
			w.Items = append(w.Items, game.ItemView{Kind: "wood", X: x, Y: y})
		}
	}
	r := benchShadowRenderer(mw, mh)
	r.cam.Zoom = 0.5
	r.view.Pawns = pawns
	r.lit.light = testLight(0.5, 0.45, 0.3)
	screen := ebiten.NewImage(1280, 800)
	tile := r.tilePx()
	r.drawShadows(screen, w, 1280, 800, tile)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.drawShadows(screen, w, 1280, 800, tile)
	}
	lay := r.env.shadow.layer.Bounds()
	b.ReportMetric(float64(lay.Dx()*lay.Dy()), "layer-px/op")
}

func BenchmarkShadowsHiDPI(b *testing.B) {
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
		case i%25 == 0:
			pawns = append(pawns, game.PawnView{ID: entity.EntityID(i + 1), X: x, Y: y})
		case i%2 == 0:
			w.Plants = append(w.Plants, game.PlantView{Species: "tree", X: x, Y: y, Growth: 80})
		default:
			w.Items = append(w.Items, game.ItemView{Kind: "wood", X: x, Y: y})
		}
	}
	r := benchShadowRenderer(mw, mh)
	r.cam.Zoom = 0.5
	r.cam.Dev = 2
	r.view.Pawns = pawns
	r.lit.light = testLight(0.5, 0.45, 0.3)
	screen := ebiten.NewImage(1280, 800)
	tile := r.tilePx()
	r.drawShadows(screen, w, 1280, 800, tile)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.drawShadows(screen, w, 1280, 800, tile)
	}
	lay := r.env.shadow.layer.Bounds()
	b.ReportMetric(float64(lay.Dx()*lay.Dy()), "layer-px/op")
}

// benchShadowRenderer is a renderer with the generated art (or none).
func benchShadowRenderer(mw, mh int) *Renderer {
	r := New(&stubSim{w: mw, h: mh})
	if root, err := LocateArt(); err == nil {
		if art, err := loadArtCatalog(root); err == nil {
			r.art = art
		}
	}
	if r.art == nil {
		r.art = envArt()
	}
	return r
}

// benchSpecies is a species with generated art (the roster is being
// re-rendered, so whichever is present).
func benchSpecies(c *artCatalog) string {
	for _, sp := range []string{"deer", "timber_wolf", "dustbuck", "ridgewolf"} {
		if c.Has("animal/" + sp + "_south") {
			return sp
		}
	}
	return "deer"
}

// BenchmarkCreatureShadows is the creature part of a frame's shadow pass
// at a low sun: 10 drafted pawns (every layer and a rifle) and 30 animals,
// each casting for the sun and the 2 campfires burning beside them, plus
// contact pools, the blur and the composite. ART-012 budget: at most
// 0.5 ms/op (CPU side).
func BenchmarkCreatureShadows(b *testing.B) {
	const mw, mh = 40, 25
	w := game.WorldView{TerrainW: mw, Terrain: make([]uint8, mw*mh), RoomGrid: make([]int, mw*mh), DayOfYear: 15}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	w.Buildings = []game.BuildingView{{ID: 1, Kind: "campfire", X: 13, Y: 10, W: 1, H: 1, Burning: true, Glow: true, GlowRadius: 5},
		{ID: 2, Kind: "campfire", X: 15, Y: 10, W: 1, H: 1, Burning: true, Glow: true, GlowRadius: 5}}
	// everyone stands within both fires' reach
	at := func(i int) (int, int) { return 12 + i%5, 8 + (i/5)%5 }
	r := benchShadowRenderer(mw, mh)
	species := benchSpecies(r.art)
	var pawns []game.PawnView
	for i := 0; i < 10; i++ {
		x, y := at(i)
		pawns = append(pawns, game.PawnView{ID: entity.EntityID(100 + i), X: x, Y: y, Weapon: "rifle", Drafted: true})
	}
	for i := 0; i < 30; i++ {
		x, y := at(10 + i)
		w.Animals = append(w.Animals, game.AnimalView{ID: entity.EntityID(200 + i), Species: species, X: x, Y: y, Health: 100})
	}
	r.cam.Zoom = 0.5
	r.view.Pawns = pawns
	r.view.W = w
	r.lit.light = testLight(0.5, 0.2, 0.3) // a low sun: point lights still cast
	screen := ebiten.NewImage(1280, 800)
	tile := r.tilePx()
	r.drawShadows(screen, w, 1280, 800, tile)
	if want := 40 * 3; r.env.shadow.casts < want && r.art.Has("pawn/body_south") && r.art.Has("animal/"+species+"_south") {
		b.Fatalf("drew %d silhouettes, want %d (sun and 2 campfires each)", r.env.shadow.casts, want)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.drawShadows(screen, w, 1280, 800, tile)
	}
	lay := r.env.shadow.layer.Bounds()
	b.ReportMetric(float64(lay.Dx()*lay.Dy()), "layer-px/op")
}
