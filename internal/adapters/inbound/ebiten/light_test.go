package ebiten

import (
	"errors"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

const summerDay = 15

// roomWorld is a 7×4 map: outdoors at x 0 and 6, a lit room (id 0)
// under a built roof at x 1..2, an inner wall at x 3, an unlit room (id
// 1) under natural roof at x 4 and an outer wall at x 5.
func roomWorld() game.WorldView {
	const w, h = 7, 4
	v := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), RoomGrid: make([]int, w*h), RoofCells: make([]uint8, w*h),
		Rooms: []game.RoomView{{ID: 0, Lit: true}, {ID: 1}}}
	for i := range v.RoomGrid {
		v.RoomGrid[i] = -1
	}
	for y := 0; y < h; y++ {
		v.RoomGrid[y*w+1], v.RoomGrid[y*w+2] = 0, 0
		v.RoofCells[y*w+1], v.RoofCells[y*w+2] = 1, 1 // built
		v.RoomGrid[y*w+3] = -2                        // inner wall
		v.RoofCells[y*w+3] = 1
		v.RoomGrid[y*w+4] = 1
		v.RoofCells[y*w+4] = 2 // natural
		v.RoomGrid[y*w+5] = -2 // outer wall
		v.RoofCells[y*w+5] = 1
	}
	return v
}

// BenchmarkDrawArtLitAB is BenchmarkDrawArt's frame with the lit draw
// (lit) and with the unlit DrawImage fallback (unlit), side by side, so
// the lighting cost reads apart from machine load and other changes.
func BenchmarkDrawArtLitAB(b *testing.B) {
	root, err := artpkg.Locate()
	if err != nil {
		b.Skip("generated art not present:", err)
	}
	art, err := loadArtCatalog(root)
	if err != nil {
		b.Fatal(err)
	}
	g := game.NewColony(game.Config{Seed: 7, Width: 80, Height: 50})
	g.Step(600)
	for _, mode := range []string{"unlit", "lit"} {
		b.Run(mode, func(b *testing.B) {
			r := New(g)
			r.art = art
			r.cam.Zoom = 0.6
			if mode == "unlit" {
				r.lit.Err = errors.New("benchmark: unlit")
			}
			screen := ebiten.NewImage(1280, 800)
			tile := r.cam.Tile()
			frame := func() {
				r.present++
				r.advance()
				r.pipe().Env.SetView(r.liveView(), 1280, 800, tile)
				r.drawArt(screen, 1280, 800, tile)
			}
			for i := 0; i < 5; i++ {
				frame()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame()
			}
		})
	}
}
