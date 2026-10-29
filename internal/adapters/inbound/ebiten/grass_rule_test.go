package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// TestGrassMatchesRendererRule pins the renderer's grass ground to the
// simulation's (PLAY-010): a tuft shows on any tile the sim seeds with
// grass, i.e. every fertile terrain (plant.GrassGround outside the legacy
// map), and the legacy seeding stays a subset of it.
func TestGrassMatchesRendererRule(t *testing.T) {
	for code, name := range terrain.Names() {
		tt := terrain.Type(code)
		if got, want := env.GrassGroundCode[code], plant.GrassGround(tt, false); got != want {
			t.Fatalf("%s: renderer grass ground %v, sim %v", name, got, want)
		}
		w := game.WorldView{TerrainW: 1, Terrain: []uint8{uint8(code)}, Grass: []uint8{1}}
		if got := env.GrassVisible(env.GrassMask(w, nil), w, name, 0, 0); got != plant.GrassGround(tt, false) {
			t.Fatalf("%s: seeded grass visible=%v", name, got)
		}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if plant.GrassAt(tt, x, y) && !env.GrassGroundCode[code] {
					t.Fatalf("%s (%d,%d): legacy grass on non-grass ground", name, x, y)
				}
			}
		}
	}
}
