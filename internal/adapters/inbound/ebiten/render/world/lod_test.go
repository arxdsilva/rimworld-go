package world

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

func TestTierFor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tile float64
		want Tier
	}{{64, Live}, {48, Live}, {47.9, Chunk}, {32, Chunk}, {16, Chunk}, {15.9, Map}, {6.4, Map}} {
		if got := TierFor(c.tile); got != c.want {
			t.Fatalf("TierFor(%v) = %d, want %d", c.tile, got, c.want)
		}
	}
}

// TestCreaturesKeepMinimumSize: below CreatureMinTile a creature is drawn
// at that size, on its own tile's footprint (centre and bottom kept).
func TestCreaturesKeepMinimumSize(t *testing.T) {
	t.Parallel()
	if LodTile(6) != CreatureMinTile || LodTile(40) != 40 {
		t.Fatal("LodTile")
	}
	wx, wy := CreatureAt(10, 20, 8, 16) // drawn twice the tile size
	// footprint centre x and bottom stay at tile (10, 20)'s
	if cx := (wx + 0.5*2); cx != 10.5 {
		t.Fatalf("centre x %v, want 10.5", cx)
	}
	if bottom := wy + 2; bottom != 21 {
		t.Fatalf("bottom %v, want 21", bottom)
	}
	if x, y := CreatureAt(3, 4, 32, 16); x != 3 || y != 4 {
		t.Fatal("no shift at or above the minimum")
	}
}

// TestLODNearAndInView: Near pads the view (south by its own pad); InView
// counts every tile until a frame has set the view.
func TestLODNearAndInView(t *testing.T) {
	t.Parallel()
	var l LOD
	if !l.InView(500, -40, 0, 0) {
		t.Fatal("no view set yet: every tile is in view")
	}
	l.VX0, l.VY0, l.VX1, l.VY1, l.ViewSet = 10, 10, 20, 15, true
	for _, c := range []struct {
		x, y, pad, padS int
		want            bool
	}{{10, 10, 0, 0, true}, {9, 10, 0, 0, false}, {9, 10, 1, 0, true}, {15, 17, 1, 1, false}, {15, 17, 1, 2, true}, {15, 8, 1, 4, false}} {
		if got := l.InView(c.x, c.y, c.pad, c.padS); got != c.want {
			t.Errorf("InView(%d, %d, %d, %d) = %v", c.x, c.y, c.pad, c.padS, got)
		}
	}
	if img := l.WhiteImg(); img == nil || l.WhiteImg() != img {
		t.Fatal("one white texel, reused")
	}
}

// TestChunkCacheReusesTheLeastRecent: a missing chunk bakes only within
// budget; a fresh one is kept; a stale one shows until it may rebake;
// past chunkCacheMax the least recently drawn image is taken over.
func TestChunkCacheReusesTheLeastRecent(t *testing.T) {
	t.Parallel()
	var cc ChunkCache
	if b, bake := cc.get(1, 7, false); b != nil || bake {
		t.Fatal("no budget: a missing chunk draws nothing")
	}
	b, bake := cc.get(1, 7, true)
	if b == nil || !bake {
		t.Fatal("a missing chunk bakes")
	}
	cc.done(b, 7)
	if got, bake := cc.get(1, 7, true); got != b || bake || cc.Bakes != 1 {
		t.Fatalf("fresh chunk rebaked (%v, %d bakes)", bake, cc.Bakes)
	}
	if got, bake := cc.get(1, 8, false); got != b || bake {
		t.Fatal("a stale chunk shows as it is without budget")
	}
	if _, bake := cc.get(1, 8, true); !bake {
		t.Fatal("a stale chunk rebakes with budget")
	}
	cc.done(b, 8)
	for c := 2; c <= chunkCacheMax; c++ {
		cc.Frame++
		nb, _ := cc.get(c, 1, true)
		cc.done(nb, 1)
	}
	if cc.held != chunkCacheMax {
		t.Fatalf("%d images held, want %d", cc.held, chunkCacheMax)
	}
	cc.Frame++
	nb, bake := cc.get(chunkCacheMax+1, 1, true)
	if nb != b || !bake || cc.ByChunk[1] != nil {
		t.Fatal("the least recently drawn chunk's image is reused")
	}
}

// TestMapBakePages: a map wider than a page splits into pages, each tile
// at its page's pixel past the padding.
func TestMapBakePages(t *testing.T) {
	t.Parallel()
	var m MapBake
	m.reset(mapPageTiles+16, 64)
	if len(m.Pages) != 2 || m.pcw != 2 || len(m.Baked) != ((mapPageTiles+16+15)/16)*4 {
		t.Fatalf("%d pages, %d per row, %d chunks", len(m.Pages), m.pcw, len(m.Baked))
	}
	img, px, py := m.page(mapPageTiles+3, 5)
	if img == nil || m.Pages[1] != img || px != 3*mapBakePx+bakePad || py != 5*mapBakePx+bakePad {
		t.Fatalf("tile on page 1 at (%v, %v)", px, py)
	}
	if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != 16*mapBakePx+2*bakePad || h != 64*mapBakePx+2*bakePad {
		t.Fatalf("second page %d×%d", w, h)
	}
	m.reset(8, 8)
	if len(m.Pages) != 1 || m.Pages[0] != nil {
		t.Fatal("reset drops the old pages")
	}
	_ = ebiten.NewImage // pages are ebiten images
}
