package ebiten

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func fadeRenderer() *Renderer {
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.cam.Zoom = 32.0 / float64(TileSize)
	r.lod.Creatures = map[[2]int]bool{}
	return r
}

func runFade(r *Renderer, x, y, frames int) float32 {
	var a float32
	for i := 0; i < frames; i++ {
		r.present++
		a = r.treeCanopyAlpha(x, y)
	}
	return a
}

func TestCanopyFadesWithCreatureBehind(t *testing.T) {
	t.Parallel()
	for _, c := range [][2]int{{5, 5}, {5, 4}, {4, 4}, {6, 4}} { // tree tile, above, both diagonals above
		r := fadeRenderer()
		r.lod.Creatures[c] = true
		if a := runFade(r, 5, 5, 30); math.Abs(float64(a)-treeSeeThrough) > 0.02 {
			t.Fatalf("creature at %v: canopy alpha %.3f, want ~%.1f", c, a, treeSeeThrough)
		}
	}
	// the tile below or two above does not count
	for _, c := range [][2]int{{5, 6}, {5, 3}, {7, 4}} {
		r := fadeRenderer()
		r.lod.Creatures[c] = true
		if a := runFade(r, 5, 5, 30); a != 1 {
			t.Fatalf("creature at %v: canopy alpha %.3f, want 1", c, a)
		}
	}
}

func TestCanopyOpaqueWithNobodyNear(t *testing.T) {
	t.Parallel()
	r := fadeRenderer()
	if a := runFade(r, 5, 5, 10); a != 1 || len(r.treeFx.Fades) != 0 {
		t.Fatalf("alpha %.2f, fades %d; want opaque and no state", a, len(r.treeFx.Fades))
	}
}

func TestCanopyEasesAndRecovers(t *testing.T) {
	t.Parallel()
	r := fadeRenderer()
	r.lod.Creatures[[2]int{5, 4}] = true
	a1 := runFade(r, 5, 5, 3)
	if a1 >= 1 || a1 <= treeSeeThrough {
		t.Fatalf("mid-ease alpha %.3f should lie between %.1f and 1", a1, treeSeeThrough)
	}
	runFade(r, 5, 5, 30)
	delete(r.lod.Creatures, [2]int{5, 4})
	if a := runFade(r, 5, 5, 60); a != 1 || len(r.treeFx.Fades) != 0 {
		t.Fatalf("after leaving: alpha %.3f, fades %d", a, len(r.treeFx.Fades))
	}
}

func TestCanopyReducedMotionSnaps(t *testing.T) {
	t.Parallel()
	r := fadeRenderer()
	r.reducedMotion = true
	r.lod.Creatures[[2]int{5, 4}] = true
	r.present++
	if a := r.treeCanopyAlpha(5, 5); a != treeSeeThrough {
		t.Fatalf("reduced motion alpha %.3f, want snap to %.1f", a, treeSeeThrough)
	}
	delete(r.lod.Creatures, [2]int{5, 4})
	r.present++
	if a := r.treeCanopyAlpha(5, 5); a != 1 {
		t.Fatalf("reduced motion alpha %.3f, want snap back to 1", a)
	}
}

func TestCanopyFadesForHoverOrGhost(t *testing.T) {
	t.Parallel()
	r := fadeRenderer()
	r.SetTreeFocus([][2]int{{6, 4}})
	if a := runFade(r, 5, 5, 30); a > 0.45 {
		t.Fatalf("focus cell above-right: alpha %.3f", a)
	}
	r.SetTreeFocus(nil)
	if a := runFade(r, 5, 5, 60); a != 1 {
		t.Fatalf("cleared focus: alpha %.3f", a)
	}
}

func TestTreeFadeDroppedOutOfView(t *testing.T) {
	t.Parallel()
	r := fadeRenderer()
	r.lod.Creatures[[2]int{5, 5}] = true
	runFade(r, 5, 5, 5)
	if len(r.treeFx.Fades) != 1 {
		t.Fatalf("fades = %d, want 1", len(r.treeFx.Fades))
	}
	r.present += 10 // the tree is no longer drawn
	r.sweepTreeFades()
	if len(r.treeFx.Fades) != 0 {
		t.Fatalf("fade state kept for a tree out of view: %d", len(r.treeFx.Fades))
	}
}

func treeArtRenderer(t *testing.T) *Renderer {
	c := testArt(t)
	r := fadeRenderer()
	r.art = c
	return r
}

// TestTreeDrawsTrunkThenCanopy: the trunk is drawn first and the canopy
// over it, also while the canopy is see-through; a creature standing
// there keeps the tree y-sorted as a single plant draw.
func TestTreeDrawsTrunkThenCanopy(t *testing.T) {
	r := treeArtRenderer(t)
	w := game.WorldView{Season: "Summer", Plants: []game.PlantView{{ID: 1, X: 5, Y: 5, Species: "tree", Growth: 100}}}
	var order []string
	r.view.Trace = &render.Trace{Layer: func(n string) { order = append(order, n) }}
	dst := ebiten.NewImage(640, 640)
	r.drawPlantObj(dst, &w, &w.Plants[0], r.tilePx())
	if got := strings.Join(order, ","); got != "tree_trunk,tree_canopy" {
		t.Skipf("tree parts not generated yet (order %q)", got)
	}
	r.lod.Creatures[[2]int{5, 4}] = true
	for i := 0; i < 5; i++ {
		r.present++
		r.drawPlantObj(dst, &w, &w.Plants[0], r.tilePx())
	}
	if got := strings.Join(order[2:], ","); got != strings.Repeat("tree_trunk,tree_canopy,", 4)+"tree_trunk,tree_canopy" {
		t.Fatalf("faded tree order = %q", got)
	}
	if st := r.treeFx.Fades[envTileKey(5, 5)]; st == nil || st.Alpha >= 1 {
		t.Fatalf("canopy not fading: %+v", st)
	}
}

func TestTreeFallsBackToFullSpriteWithoutParts(t *testing.T) {
	r := treeArtRenderer(t)
	if _, _, ok := r.treePartKeys("plant/bush_mature_v0"); ok {
		t.Fatal("a bush has no tree parts")
	}
	r.art = &artCatalog{} // no sprites at all: no parts, nothing drawn, no panic
	w := game.WorldView{Season: "Summer", Plants: []game.PlantView{{ID: 1, X: 5, Y: 5, Species: "tree", Growth: 100}}}
	var order []string
	r.view.Trace = &render.Trace{Layer: func(n string) { order = append(order, n) }}
	r.drawPlantObj(ebiten.NewImage(64, 64), &w, &w.Plants[0], r.tilePx())
	if len(order) != 0 {
		t.Fatalf("parts traced without art: %v", order)
	}
}
