package env

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

func TestTuftCopiesFollowGrowth(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		g    uint8
		want int
	}{{0, 1}, {24, 1}, {25, 2}, {49, 2}, {50, 4}, {74, 4}, {75, 6}, {89, 6}, {90, 9}, {100, 9}} {
		if got := TuftCopies(c.g); got != c.want {
			t.Fatalf("growth %d: %d copies, want %d", c.g, got, c.want)
		}
		if n := len(GrassCopies(3, 4, c.g, 9, false, nil)); n != c.want {
			t.Fatalf("growth %d: grassCopies made %d, want %d", c.g, n, c.want)
		}
		if n := len(GrassCopies(3, 4, c.g, 4, false, nil)); n != min(c.want, 4) {
			t.Fatalf("growth %d capped at 4: %d copies", c.g, n)
		}
	}
}

// TestGrassCopiesDeterministic: a tile's copies (mirror, variant, place)
// are a pure function of the tile; tiles differ.
func TestGrassCopiesDeterministic(t *testing.T) {
	t.Parallel()
	mirrored, plain := 0, 0
	variants := map[uint32]bool{}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			a := GrassCopies(x, y, 100, 9, false, nil)
			b := GrassCopies(x, y, 100, 9, false, nil)
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("tile (%d,%d) copy %d differs: %+v vs %+v", x, y, i, a[i], b[i])
				}
				if a[i].Mirror {
					mirrored++
				} else {
					plain++
				}
				variants[a[i].Variant%6] = true
			}
		}
	}
	if mirrored < 300 || plain < 300 {
		t.Fatalf("mirroring not mixed: %d mirrored, %d plain", mirrored, plain)
	}
	if len(variants) != 6 {
		t.Fatalf("variants used %v", variants)
	}
}

// TestGrassScaleRanges: short copies scale 0.45–0.65, tall 0.7–0.9 at full
// growth; tall patches cluster (ValueNoise) and rich soil is tall.
func TestGrassScaleRanges(t *testing.T) {
	t.Parallel()
	for y := 0; y < 30; y++ {
		for x := 0; x < 30; x++ {
			for _, tall := range []bool{false, true} {
				for _, c := range GrassCopies(x, y, 100, 9, tall, nil) {
					lo, hi := 0.45, 0.65
					if c.Tall {
						lo, hi = 0.7, 0.9
					}
					if c.Scale < lo-1e-9 || c.Scale > hi+1e-9 {
						t.Fatalf("tall=%v copy scale %.3f outside %.2f–%.2f", c.Tall, c.Scale, lo, hi)
					}
					if c.Tall && !tall {
						t.Fatal("tall copy on a short tile")
					}
				}
			}
		}
	}
	soil, rich := terrainCodeOf("soil"), terrainCodeOf("rich_soil")
	if !GrassTallAt(rich, 3, 3) {
		t.Fatal("rich soil is not tall")
	}
	same, pairs, tall := 0, 0, 0
	for y := 0; y < 120; y++ {
		for x := 0; x < 120; x++ {
			a := GrassTallAt(soil, x, y)
			if a {
				tall++
			}
			if GrassTallAt(soil, x+1, y) == a {
				same++
			}
			pairs++
		}
	}
	if tall == 0 || tall > pairs/2 {
		t.Fatalf("%d of %d soil tiles tall", tall, pairs)
	}
	if float64(same)/float64(pairs) < 0.85 {
		t.Fatalf("tall patches checkerboard: neighbours agree %.2f", float64(same)/float64(pairs))
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
	m := GrassMask(w, nil)
	for _, c := range [][2]int{{0, 0}, {1, 0}, {3, 3}, {2, 2}} {
		if GrassVisible(m, w, "soil", c[0], c[1]) {
			t.Fatalf("tuft at covered %v", c)
		}
	}
	if !GrassVisible(m, w, "soil", 2, 0) || !GrassVisible(m, w, "rich_soil", 0, 1) {
		t.Fatal("uncovered grass must show")
	}
	if GrassVisible(m, w, "stone", 2, 0) {
		t.Fatal("no grass on stone")
	}
	if m[3*4+1]&GrassMarked == 0 || m[0]&GrassMarked != 0 {
		t.Fatalf("cut mark wrong: %v", m)
	}
	// reuse: no growth when the buffer already fits
	if m2 := GrassMask(w, m); &m2[0] != &m[0] {
		t.Fatal("buffer not reused")
	}
	// empty grass data falls back to the old rule
	e := game.WorldView{}
	for x := 0; x < 20; x++ {
		if GrassVisible(nil, e, "soil", x, 0) != ((sprite.TileHash(x, 0)>>8)%GrassEvery == 0) {
			t.Fatal("fallback differs from old rule")
		}
	}
}

func terrainCodeOf(name string) uint8 {
	for i, n := range terrainNames {
		if n == name {
			return uint8(i)
		}
	}
	panic(name)
}
