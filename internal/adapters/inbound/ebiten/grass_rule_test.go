package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// TestGrassMatchesRendererRule pins the domain's grass rule to the formula
// the renderer used before grass became simulation state.
func TestGrassMatchesRendererRule(t *testing.T) {
	for _, tt := range []struct {
		t    terrain.Type
		name string
	}{{terrain.Soil, "soil"}, {terrain.RichSoil, "rich_soil"}, {terrain.Stone, "stone"}, {terrain.Gravel, "gravel"}} {
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				old := (tt.name == "soil" || tt.name == "rich_soil") && (tileHash(x, y)>>8)%grassEvery == 0
				if got := plant.GrassAt(tt.t, x, y); got != old {
					t.Fatalf("%s (%d,%d): GrassAt=%v renderer=%v", tt.name, x, y, got, old)
				}
			}
		}
	}
}

func TestGrassSkipsCoveredTiles(t *testing.T) {
	w := game.WorldView{TerrainW: 4, Grass: make([]uint8, 16)}
	for i := range w.Grass {
		w.Grass[i] = 1
	}
	w.Buildings = []game.BuildingView{{X: 0, Y: 0, W: 2, H: 1}}
	w.Frames = []game.FrameView{{X: 3, Y: 3}}
	w.Plants = []game.PlantView{{X: 2, Y: 2}}
	w.GrassCut = [][2]int{{1, 3}}
	m := grassMask(w, nil)
	for _, c := range [][2]int{{0, 0}, {1, 0}, {3, 3}, {2, 2}} {
		if grassVisible(m, w, "soil", c[0], c[1]) {
			t.Fatalf("tuft at covered %v", c)
		}
	}
	if !grassVisible(m, w, "soil", 2, 0) || !grassVisible(m, w, "rich_soil", 0, 1) {
		t.Fatal("uncovered grass must show")
	}
	if grassVisible(m, w, "stone", 2, 0) {
		t.Fatal("no grass on stone")
	}
	if m[3*4+1]&grassMarked == 0 || m[0]&grassMarked != 0 {
		t.Fatalf("cut mark wrong: %v", m)
	}
	// reuse: no growth when the buffer already fits
	if m2 := grassMask(w, m); &m2[0] != &m[0] {
		t.Fatal("buffer not reused")
	}
	// empty grass data falls back to the old rule
	e := game.WorldView{}
	for x := 0; x < 20; x++ {
		if grassVisible(nil, e, "soil", x, 0) != ((tileHash(x, 0)>>8)%grassEvery == 0) {
			t.Fatal("fallback differs from old rule")
		}
	}
}
