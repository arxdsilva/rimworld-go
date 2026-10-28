package ebiten

import (
	"fmt"
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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

func TestSnowDepthPerTile(t *testing.T) {
	t.Parallel()
	e := &envState{terrW: 4, snowCells: []uint8{0, 255, 0, 0, 51, 0, 0, 0}, snow: 0.7}
	if d := e.snowDepthAt(1, 0); d != 1 {
		t.Fatalf("tile depth %v", d)
	}
	if d := e.snowDepthAt(2, 1); d != 0 || e.snowDepthAt(-1, 0) != 0 || e.snowDepthAt(0, 5) != 0 {
		t.Fatal("bare or off-map tiles read 0")
	}
	// the corner between (0,0), (1,0), (0,1), (1,1) is their mean
	if c := e.snowCorner(1, 1); math.Abs(c-(255+51)/4.0/255) > 1e-9 {
		t.Fatalf("corner %v", c)
	}
	// a map corner averages the one tile it touches
	if c := e.snowCorner(0, 0); c != 0 {
		t.Fatalf("map corner %v", c)
	}
	if c := e.snowCorner(-5, -5); c != 0 {
		t.Fatalf("far corner %v", c)
	}
	// without the layer every tile reads the map figure
	u := &envState{terrW: 4, snow: 0.7}
	if u.snowDepthAt(3, 1) != 0.7 || u.snowCorner(2, 2) != 0.7 {
		t.Fatal("no layer: the map figure")
	}
}

// snowTexel is the snow (row 0) or wet veil (row 1, PLAY-022) alpha at
// texel (sx, sy) of 4 a tile (snowPx) of chunk-local tile (x, y), from
// the last bake in sc.pix or sc.wpix (a direct bake is at 4 texels a
// tile; the wet veil is 0 where no image was written).
func snowTexel(sc *snowCache, row, x, y, sx, sy int) uint8 {
	if row == 0 {
		tx, ty := snowPad+x*snowPx+sx, snowPad+y*snowPx+sy
		return sc.pix[4*(ty*snowImgW+tx)+3]
	}
	px := max(sc.wantPx, snowPx)
	iw := wetImgW(px)
	tx, ty := snowPad+x*px+sx*px/snowPx, snowPad+y*px+sy*px/snowPx
	if len(sc.wpix) < 4*iw*iw {
		return 0
	}
	return sc.wpix[4*(ty*iw+tx)+3]
}

func TestGroundCoverVeilFollowsTileSnow(t *testing.T) {
	t.Parallel()
	w := snowyWorld()
	r := New(newEnvSim(w))
	r.art = envArt()
	r.cam.OffsetX, r.cam.OffsetY = 0, 0 // the map's top-left corner
	screen := ebiten.NewImage(640, 400)
	r.drawGroundCover(screen, w, 640, 400, r.tilePx())
	if x0, y0, x1, y1 := r.visibleTiles(); x0 > 1 || y0 > 0 || x1 < 7 || y1 < 6 {
		t.Fatalf("view %d,%d..%d,%d misses the block", x0, y0, x1, y1)
	}
	sc := &r.snowc
	if sc.bakes == 0 || sc.draws == 0 {
		t.Fatalf("snowy block: %d bakes, %d draws", sc.bakes, sc.draws)
	}
	var c snowChunk
	r.bakeSnowChunk(&c, w, 64, 64, 0, 0)
	if !c.any {
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
	r.drawGroundCover(screen, w, 640, 400, r.tilePx())
	for id, ch := range sc.chunks {
		if ch.any {
			t.Fatalf("bare layer: chunk %d still holds snow", id)
		}
	}
	if sc.draws != 0 {
		t.Fatalf("a bare layer draws %d snow passes", sc.draws)
	}
	// an older sim with only the map figure veils every open tile
	w.Snow, w.SnowDepth = nil, 0.8
	r.drawGroundCover(screen, w, 640, 400, r.tilePx())
	if sc.draws == 0 {
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
	tile := r.tilePx()
	draw := func() int {
		n := r.snowc.bakes
		r.drawGroundCover(screen, w, 640, 400, tile)
		return r.snowc.bakes - n
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
	if n := draw(); n != len(r.snowc.chunks) || n == 0 {
		t.Fatalf("StructVer rebaked %d of %d chunks", n, len(r.snowc.chunks))
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
	r.drawGroundCover(screen, w, 640, 400, r.tilePx())
	if r.snowc.draws == 0 || r.lit.Calls-before < r.snowc.draws {
		t.Fatalf("lit calls %d for %d snow draws", r.lit.Calls-before, r.snowc.draws)
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
			tile := r.tilePx()
			r.cam.OffsetX, r.cam.OffsetY = 100*tile, 100*tile
			r.drawGroundCover(screen, w, 1280, 800, tile)
			b0, d0 := r.snowc.bakes, 0
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.drawGroundCover(screen, w, 1280, 800, tile)
				d0 += r.snowc.draws
			}
			b.ReportMetric(float64(r.snowc.bakes-b0)/float64(b.N), "bakes/op")
			b.ReportMetric(float64(d0)/float64(b.N), "draws/op")
		})
	}
}
