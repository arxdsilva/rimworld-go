package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	lightpkg "rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// testLight is a hand-built light: shadows fall away from a light at
// azimuth az (screen space, 0 = east) and elevation el (radians), clamped
// at 3 tiles, with the given alpha.
func testLight(az, el, alpha float64) lightpkg.State {
	lx, ly := math.Cos(az)*math.Cos(el), 0.5*math.Sin(az)*math.Cos(el)
	n := math.Hypot(lx, ly)
	return lightpkg.State{
		SunL:      [3]float32{float32(lx), float32(ly), float32(math.Sin(el))},
		ShadowDir: [2]float64{-lx / n, -ly / n},
		ShadowLen: func(h float64) float64 {
			return math.Min(math.Max(h/math.Tan(el), 0.25*h), 3)
		},
		ShadowAlpha: alpha,
	}
}

// shadowWorld is a 20x12 open map with the given roof cells set (index ->
// flags); every other tile is outdoors.
func shadowWorld(roofs map[int]uint8) game.WorldView {
	w := game.WorldView{TerrainW: 20, Terrain: make([]uint8, 20*12), RoomGrid: make([]int, 20*12), Hour: 12, DayOfYear: 15}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	if roofs != nil {
		w.RoofCells = make([]uint8, 20*12)
		for i, c := range roofs {
			w.RoofCells[i] = c
		}
	}
	return w
}

func shadowRenderer(t *testing.T, w game.WorldView) *Renderer {
	t.Helper()
	r := New(&stubSim{w: 20, h: 12, world: w})
	r.art = envArt()
	r.env.View = env.Rect{X0: 0, Y0: 0, X1: 20, Y1: 12}
	return r
}

// TestOnlyColonistsOutdoorsCastShadows: in whole frames (the real
// pipeline, sun, low sun and moon) a colonist outdoors casts one shadow
// under its feet; the same colonist under a roof casts none; an animal, a
// prisoner and a raider cast none; and walls, rock, a tree, items and a
// bridge cast none at all (user decision, 2026-11-01).
func TestOnlyColonistsOutdoorsCastShadows(t *testing.T) {
	t.Parallel()
	roofs := map[int]uint8{}
	for x := 13; x < 18; x++ { // a roofed room around (15, 2)
		for y := 1; y <= 3; y++ {
			roofs[y*20+x] = overlay.RoofBuilt
		}
	}
	scenery := func(w *game.WorldView) {
		w.Buildings = []game.BuildingView{{Kind: "wall", X: 2, Y: 2, W: 1, H: 1}, {Kind: "wall", X: 3, Y: 2, W: 1, H: 1},
			{Kind: "bridge", X: 5, Y: 9, W: 1, H: 1}, lampBuilding(20, 8, 5)}
		w.Plants = []game.PlantView{{Species: "tree", X: 9, Y: 2, Growth: 80}}
		w.Items = []game.ItemView{{Kind: "wood", X: 10, Y: 3}, {Kind: "steel", X: 11, Y: 8}}
		w.Terrain[0] = rockCode(t)
	}
	colonist := game.PawnView{ID: 1, Name: "Ada", X: 4, Y: 6, Status: "alive"}
	for _, c := range []struct {
		name  string
		hour  float64
		light lightpkg.State
	}{
		{"noon sun", 12, lightpkg.LightAt(12, 15, "clear", 0)},
		{"low sun", 7, testLight(0.4, 0.3, 0.32)},
		{"moon", 23, lightpkg.LightAt(23, 15, "clear", 0)},
	} {
		frame := func(pawns []game.PawnView, animals []game.AnimalView, withScenery bool) *lightpkg.Shadows {
			w := shadowWorld(roofs)
			w.Hour = c.hour
			w.Animals = animals
			if withScenery {
				scenery(&w)
			}
			r := shadowRenderer(t, w)
			r.game.(*stubSim).pawns = pawns
			r.drawArt(ebiten.NewImage(640, 400), 640, 400, r.cam.Tile())
			r.lit.Light = c.light // a hand-built light for the low sun: redraw just the pass in it
			r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.cam.Tile())
			return &r.env.Shadow
		}
		// a colonist outdoors: one shadow, under the feet
		st := frame([]game.PawnView{colonist}, nil, true)
		if st.Drawn != 1 || len(st.Feet) != 1 {
			t.Fatalf("%s: a colonist outdoors drew %d shadows (%d feet), want 1", c.name, st.Drawn, len(st.Feet))
		}
		if f := st.Feet[0]; f.X != 4+creature.PawnFeetX || f.Y != 6+creature.PawnFeetY {
			t.Fatalf("%s: the shadow is at (%.2f,%.2f), not under the feet", c.name, f.X, f.Y)
		}
		// the same colonist in the roofed room: none
		in := colonist
		in.X, in.Y = 15, 2
		if st := frame([]game.PawnView{in}, nil, true); st.Drawn != 0 || len(st.Feet) != 0 {
			t.Fatalf("%s: a colonist under a roof drew %d shadows", c.name, st.Drawn)
		}
		// an animal, a prisoner and a raider outdoors, with walls, rock,
		// trees, items and a bridge: nothing casts
		prisoner, raider := colonist, colonist
		prisoner.ID, prisoner.Prisoner = 2, true
		raider.ID, raider.X, raider.Hostile = 3, 7, true
		animals := []game.AnimalView{{ID: 7, Species: "dustbuck", X: 6, Y: 7}}
		if st := frame([]game.PawnView{prisoner, raider}, animals, true); st.Drawn != 0 || len(st.Feet) != 0 || st.Flushes != 0 {
			t.Fatalf("%s: animals, prisoners, raiders or scenery drew %d shadows in %d draws", c.name, st.Drawn, st.Flushes)
		}
	}
}

// rockCode is a rough rock wall's terrain code.
func rockCode(t *testing.T) uint8 {
	t.Helper()
	for c := 0; c < 256; c++ {
		if spritepkg.IsRockWallCode(uint8(c)) {
			return uint8(c)
		}
	}
	t.Fatal("no rock terrain code")
	return 0
}
