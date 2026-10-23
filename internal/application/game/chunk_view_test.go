package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/chunk"
)

// TestChunkViewMatchesFullLists: the chunk lists partition World()'s
// plants, items, buildings and frames by anchor tile, in list order, and
// PlantsIn is the full list filtered by the rectangle.
func TestChunkViewMatchesFullLists(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 70, Height: 45})
	for x := 12; x < 36; x += 3 { // walls either side of chunk borders
		g.BuildNow("wall", "wood", "south", x, 15)
		g.BuildNow("wall", "wood", "south", x, 16)
	}
	g.SetViewCache(true)
	g.Step(30)
	w := g.World()
	cg := chunk.New(g.Width(), g.Height())
	if len(w.Plants) == 0 || len(w.Items) == 0 || len(w.Buildings) == 0 {
		t.Fatalf("colony too empty to test: %d plants %d items %d buildings", len(w.Plants), len(w.Items), len(w.Buildings))
	}
	type list struct {
		name string
		n    int
		at   func(i int) (int, int)
		of   func(v ChunkView) []int32
	}
	for _, l := range []list{
		{"plants", len(w.Plants), func(i int) (int, int) { return w.Plants[i].X, w.Plants[i].Y }, func(v ChunkView) []int32 { return v.Plants }},
		{"items", len(w.Items), func(i int) (int, int) { return w.Items[i].X, w.Items[i].Y }, func(v ChunkView) []int32 { return v.Items }},
		{"buildings", len(w.Buildings), func(i int) (int, int) { return w.Buildings[i].X, w.Buildings[i].Y }, func(v ChunkView) []int32 { return v.Buildings }},
		{"frames", len(w.Frames), func(i int) (int, int) { return w.Frames[i].X, w.Frames[i].Y }, func(v ChunkView) []int32 { return v.Frames }},
	} {
		seen := make([]int, l.n)
		for cy := 0; cy < cg.CH; cy++ {
			for cx := 0; cx < cg.CW; cx++ {
				idx := l.of(g.ChunkLayer(cx, cy))
				if !slices.IsSorted(idx) {
					t.Fatalf("%s of chunk (%d,%d) not in list order: %v", l.name, cx, cy, idx)
				}
				for _, i := range idx {
					x, y := l.at(int(i))
					if x>>chunk.Shift != cx || y>>chunk.Shift != cy {
						t.Fatalf("%s %d at (%d,%d) listed in chunk (%d,%d)", l.name, i, x, y, cx, cy)
					}
					seen[i]++
				}
			}
		}
		for i, n := range seen {
			if x, y := l.at(i); n != 1 && x >= 0 && y >= 0 && x < g.Width() && y < g.Height() {
				t.Fatalf("%s %d at (%d,%d) listed %d times", l.name, i, x, y, n)
			}
		}
	}
	if v := g.ChunkLayer(-1, 0); v.Ver != 0 || v.Plants != nil {
		t.Fatal("a chunk off the map has a view")
	}
	for _, r := range [][4]int{{0, 0, 69, 44}, {10, 5, 30, 20}, {-5, -5, 3, 3}, {60, 40, 90, 90}, {20, 20, 20, 20}} {
		got := g.PlantsIn(r[0], r[1], r[2], r[3], nil)
		var want []int32
		for i, p := range w.Plants {
			if p.X >= r[0] && p.X <= r[2] && p.Y >= r[1] && p.Y <= r[3] {
				want = append(want, int32(i))
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("PlantsIn(%v) = %d plants, want %d", r, len(got), len(want))
		}
	}
}

// TestChunkVersionsTrackChanges: a chunk's ground version moves when its
// terrain, a building touching it or a neighbour's edge tile changes, and
// not when nothing it shows does; plant versions follow the plants.
func TestChunkVersionsTrackChanges(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 64, Height: 64})
	g.SetViewCache(true)
	g.Step(2)
	vers := func() []ChunkView {
		var out []ChunkView
		for cy := 0; cy < 4; cy++ {
			for cx := 0; cx < 4; cx++ {
				out = append(out, g.ChunkLayer(cx, cy))
			}
		}
		return out
	}
	a := vers()
	b := vers()
	for i := range a {
		if a[i].Ver != b[i].Ver || a[i].PlantVer != b[i].PlantVer {
			t.Fatal("versions moved with nothing changed")
		}
	}
	// rock in chunk (0,0) at its east edge: (0,0) and its east neighbour (1,0)
	if !g.SetGround(15, 2, 15, 2, "rock", "granite", "") {
		t.Fatal("SetGround")
	}
	g.Step(1) // the view cache sees dev hooks on the next tick
	c := vers()
	if c[0].Ver == a[0].Ver || c[1].Ver == a[1].Ver {
		t.Fatalf("edge rock: chunk (0,0) %d->%d, (1,0) %d->%d", a[0].Ver, c[0].Ver, a[1].Ver, c[1].Ver)
	}
	if c[15].Ver != a[15].Ver {
		t.Fatal("a far chunk's version moved")
	}
	cut := vers()
	if len(cut[10].Plants) == 0 {
		t.Skip("no plants to clear in chunk (2,2)")
	}
	g.ClearPlants(32, 32, 47, 47)
	g.Step(1)
	d := vers()
	if d[10].PlantVer == cut[10].PlantVer {
		t.Fatal("clearing a chunk's plants kept its plant version")
	}
}

// TestMinimapLayer: the layer has a cell per tile, is not resent when
// unchanged and is when the ground changes.
func TestMinimapLayer(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 40, Height: 30})
	g.SetViewCache(true)
	g.Step(1)
	v, ok := g.MinimapLayer(0)
	if !ok || v.W != 40 || v.H != 30 || len(v.Cells) != 40*30 || v.Ver == 0 {
		t.Fatalf("first layer: ok=%v %dx%d cells=%d ver=%d", ok, v.W, v.H, len(v.Cells), v.Ver)
	}
	if _, again := g.MinimapLayer(v.Ver); again {
		t.Fatal("an unchanged layer was sent again")
	}
	w := g.World()
	for i, c := range v.Cells {
		if int(c&MinimapTerrain) != int(w.Terrain[i]) {
			t.Fatalf("cell %d terrain %d, want %d", i, c&MinimapTerrain, w.Terrain[i])
		}
	}
	g.SetGround(0, 0, 3, 3, "rock", "granite", "")
	g.Step(1)
	n, ok := g.MinimapLayer(v.Ver)
	if !ok || n.Ver == v.Ver || TerrainNames()[n.Cells[0]&MinimapTerrain] != "rock" {
		t.Fatalf("after SetGround: ok=%v ver %d->%d", ok, v.Ver, n.Ver)
	}
	if &n.Cells[0] == &v.Cells[0] {
		t.Fatal("a layer handed out was written in place")
	}
}
