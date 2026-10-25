package ebiten

import (
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

func TestGroundCoverVeilFollowsTileSnow(t *testing.T) {
	t.Parallel()
	w := snowyWorld()
	r := New(newEnvSim(w))
	r.art = envArt()
	r.cam.offsetX, r.cam.offsetY = 0, 0 // the map's top-left corner
	screen := ebiten.NewImage(640, 400)
	r.drawGroundCover(screen, w, 640, 400, r.tilePx())
	if x0, y0, x1, y1 := r.visibleTiles(); x0 > 1 || y0 > 0 || x1 < 7 || y1 < 6 {
		t.Fatalf("view %d,%d..%d,%d misses the block", x0, y0, x1, y1)
	}
	n := r.env.veilQuads
	// the block's tiles and the ring the corners shade into, nothing else
	if n < 16 || n > 6*6 {
		t.Fatalf("veil quads %d, want the snowy block (16) and its fringe", n)
	}
	w.Snow = make([]uint8, 64*64)
	r.drawGroundCover(screen, w, 640, 400, r.tilePx())
	if r.env.veilQuads != 0 {
		t.Fatalf("a bare layer draws %d veil quads", r.env.veilQuads)
	}
	// an older sim with only the map figure veils every open tile in view
	w.Snow, w.SnowDepth = nil, 0.8
	r.drawGroundCover(screen, w, 640, 400, r.tilePx())
	if r.env.veilQuads <= n {
		t.Fatalf("uniform snow veils %d quads, want the whole view", r.env.veilQuads)
	}
}
