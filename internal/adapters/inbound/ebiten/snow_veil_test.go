package ebiten

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
)

// snowyWorld is a 64x64 open map in winter whose per-tile snow lies only
// on the 4x4 block x 2..5, y 1..4 (depth 200), with no map figure.
func snowyWorld() game.WorldView {
	w := game.WorldView{Weather: "clear", OutdoorTemp: 50, Season: "Winter", TerrainW: 64,
		Terrain: make([]uint8, 64*64), RoomGrid: make([]int, 64*64), Snow: make([]uint8, 64*64), SnowVer: 1}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	for y := 1; y <= 4; y++ {
		for x := 2; x <= 5; x++ {
			w.Snow[y*64+x] = 200
		}
	}
	return w
}

// snowTexel is the snow (row 0) or wet veil (row 1, PLAY-022) alpha at
// texel (sx, sy) of 4 a tile (snowPx) of chunk-local tile (x, y), from
// the last bake in sc.pix or sc.wpix (a direct bake is at 4 texels a
// tile; the wet veil is 0 where no image was written).
func snowTexel(sc *env.SnowCache, row, x, y, sx, sy int) uint8 {
	if row == 0 {
		tx, ty := env.SnowPad+x*env.SnowPx+sx, env.SnowPad+y*env.SnowPx+sy
		return sc.Pix[4*(ty*env.SnowImgW+tx)+3]
	}
	px := max(sc.WantPx, env.SnowPx)
	iw := env.WetImgW(px)
	tx, ty := env.SnowPad+x*px+sx*px/env.SnowPx, env.SnowPad+y*px+sy*px/env.SnowPx
	if len(sc.Wpix) < 4*iw*iw {
		return 0
	}
	return sc.Wpix[4*(ty*iw+tx)+3]
}

func TestGroundCoverVeilFollowsTileSnow(t *testing.T) {
	t.Parallel()
	w := snowyWorld()
	r := New(newEnvSim(w))
	r.art = envArt()
	r.cam.OffsetX, r.cam.OffsetY = 0, 0 // the map's top-left corner
	screen := ebiten.NewImage(640, 400)
	r.drawGroundCover(screen, w, 640, 400, r.cam.Tile())
	if x0, y0, x1, y1 := r.pipe().Env.VisibleTiles(r.liveView()); x0 > 1 || y0 > 0 || x1 < 7 || y1 < 6 {
		t.Fatalf("view %d,%d..%d,%d misses the block", x0, y0, x1, y1)
	}
	sc := &r.snowc
	if sc.Bakes == 0 || sc.Draws == 0 {
		t.Fatalf("snowy block: %d bakes, %d draws", sc.Bakes, sc.Draws)
	}
	var c env.SnowChunk
	r.bakeSnowChunk(&c, w, 64, 64, 0, 0)
	if !c.Any {
		t.Fatal("chunk (0,0) holds snow but reads bare")
	}
	if snowTexel(sc, 0, 3, 2, 2, 2) == 0 {
		t.Error("a snowy tile's centre is bare")
	}
	l, rt := snowTexel(sc, 0, 6, 2, 0, 2), snowTexel(sc, 0, 6, 2, 3, 2)
	if l == 0 || rt >= l {
		t.Errorf("fringe tile: left %d right %d, want snow fading out", l, rt)
	}
	if snowTexel(sc, 0, 8, 2, 2, 2) != 0 || snowTexel(sc, 0, 20, 20, 2, 2) != 0 {
		t.Error("bare ground carries snow")
	}
	// a bare layer draws nothing
	w.Snow, w.SnowVer = make([]uint8, 64*64), 2
	r.drawGroundCover(screen, w, 640, 400, r.cam.Tile())
	for id, ch := range sc.Chunks {
		if ch.Any {
			t.Fatalf("bare layer: chunk %d still holds snow", id)
		}
	}
	if sc.Draws != 0 {
		t.Fatalf("a bare layer draws %d snow passes", sc.Draws)
	}
	// an older sim with only the map figure veils every open tile
	w.Snow, w.SnowDepth = nil, 0.8
	r.drawGroundCover(screen, w, 640, 400, r.cam.Tile())
	if sc.Draws == 0 {
		t.Fatal("uniform snow draws nothing")
	}
	r.bakeSnowChunk(&c, w, 64, 64, 0, 0)
	if snowTexel(sc, 0, 20, 20, 2, 2) == 0 {
		t.Error("uniform snow leaves open ground bare")
	}
}

func TestSnowCacheRebakesOnlyChangedChunks(t *testing.T) {
	t.Parallel()
	w := snowyWorld()
	r := New(newEnvSim(w))
	r.art = envArt()
	r.cam.OffsetX, r.cam.OffsetY = 0, 0
	screen := ebiten.NewImage(640, 400)
	tile := r.cam.Tile()
	draw := func() int {
		n := r.snowc.Bakes
		r.drawGroundCover(screen, w, 640, 400, tile)
		return r.snowc.Bakes - n
	}
	if draw() == 0 {
		t.Fatal("first draw baked nothing")
	}
	if n := draw(); n != 0 {
		t.Fatalf("same frame rebaked %d chunks", n)
	}
	w.SnowVer++ // a new version with the same bytes: compared, not rebaked
	if n := draw(); n != 0 {
		t.Fatalf("equal snow rebaked %d chunks", n)
	}
	w.Snow = append([]uint8(nil), w.Snow...)
	w.Snow[3*64+3] = 90
	w.SnowVer++
	if n := draw(); n != 1 {
		t.Fatalf("one tile changed rebaked %d chunks, want 1", n)
	}
	w.StructVer++
	if n := draw(); n != len(r.snowc.Chunks) || n == 0 {
		t.Fatalf("StructVer rebaked %d of %d chunks", n, len(r.snowc.Chunks))
	}
}

func TestSnowLayerDrawsLit(t *testing.T) {
	t.Parallel()
	w := snowyWorld()
	w.Wetness = 0.5
	r := New(newEnvSim(w))
	r.art = envArt()
	r.cam.OffsetX, r.cam.OffsetY = 0, 0
	screen := ebiten.NewImage(640, 400)
	before := r.lit.Calls
	r.drawGroundCover(screen, w, 640, 400, r.cam.Tile())
	if r.snowc.Draws == 0 || r.lit.Calls-before < r.snowc.Draws {
		t.Fatalf("lit calls %d for %d snow draws", r.lit.Calls-before, r.snowc.Draws)
	}
}

// BenchmarkGroundCover is the snow and wet ground cover of a 500² world
// with a snowy quarter, at each LOD's tile size. bakes/op is 0 once warm.
func BenchmarkGroundCover(b *testing.B) {
	const side = 500
	w := game.WorldView{Weather: "clear", OutdoorTemp: 50, Season: "Winter", TerrainW: side, Wetness: 0.5,
		Terrain: make([]uint8, side*side), RoomGrid: make([]int, side*side), Snow: make([]uint8, side*side), SnowVer: 1}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	for y := 0; y < side/2; y++ {
		for x := 0; x < side/2; x++ {
			w.Snow[y*side+x] = 180
		}
	}
	for _, px := range []float64{64, 32, 16, 4} {
		b.Run(fmt.Sprintf("tile%v", px), func(b *testing.B) {
			sim := &stubSim{w: side, h: side, world: w}
			r := New(sim)
			r.art = envArt()
			r.cam.Zoom = px / 64
			screen := ebiten.NewImage(1280, 800)
			tile := r.cam.Tile()
			r.cam.OffsetX, r.cam.OffsetY = 100*tile, 100*tile
			r.drawGroundCover(screen, w, 1280, 800, tile)
			b0, d0 := r.snowc.Bakes, 0
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.drawGroundCover(screen, w, 1280, 800, tile)
				d0 += r.snowc.Draws
			}
			b.ReportMetric(float64(r.snowc.Bakes-b0)/float64(b.N), "bakes/op")
			b.ReportMetric(float64(d0)/float64(b.N), "draws/op")
		})
	}
}
