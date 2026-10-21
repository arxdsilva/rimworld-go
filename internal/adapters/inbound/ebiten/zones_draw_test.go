package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// rectZone builds a rectangular ZoneView for tests.
func rectZone(id uint16, kind string, minX, minY, maxX, maxY int) game.ZoneView {
	z := game.ZoneView{ID: id, Kind: kind, MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			z.Cells = append(z.Cells, [2]int{x, y})
		}
	}
	return z
}

func zonesOfKind(w game.WorldView, kind string) []game.ZoneView {
	var out []game.ZoneView
	for _, z := range w.Zones {
		if z.Kind == kind {
			out = append(out, z)
		}
	}
	return out
}

func cellSet(cells ...[2]int) map[[2]int]bool {
	m := map[[2]int]bool{}
	for _, c := range cells {
		m[c] = true
	}
	return m
}

func TestZoneEdgeMask(t *testing.T) {
	t.Parallel()
	z := rectZone(1, game.ZoneStockpile, 0, 0, 2, 2)
	set := cellSet(z.Cells...)
	cases := []struct {
		x, y, want int
	}{
		{1, 1, 0},
		{0, 0, zoneN | zoneW},
		{2, 0, zoneN | zoneE},
		{0, 2, zoneS | zoneW},
		{2, 2, zoneS | zoneE},
		{1, 0, zoneN},
	}
	for _, c := range cases {
		if got := zoneEdgeMask(set, c.x, c.y); got != c.want {
			t.Errorf("(%d,%d) mask = %d, want %d", c.x, c.y, got, c.want)
		}
	}
	if got := zoneEdgeMask(cellSet([2]int{5, 5}), 5, 5); got != 15 {
		t.Fatalf("single tile mask = %d, want 15", got)
	}
}

func TestZoneEdgeRunsMergeConsecutiveCells(t *testing.T) {
	t.Parallel()
	z := rectZone(1, game.ZoneGrow, 2, 3, 4, 3) // a 3x1 strip
	runs := zoneEdgeRuns(cellSet(z.Cells...), z.Cells)
	if len(runs) != 4 {
		t.Fatalf("a strip has 4 outline runs, got %d: %+v", len(runs), runs)
	}
	long := 0
	for _, r := range runs {
		if r.y0 == r.y1 && r.x1-r.x0 == 3 {
			long++
		}
	}
	if long != 2 {
		t.Fatalf("north and south edges must each be one run of 3: %+v", runs)
	}
	// two touching zones' union has no seam between them
	a := rectZone(1, game.ZoneStockpile, 0, 0, 1, 1)
	b := rectZone(2, game.ZoneStockpile, 2, 0, 3, 1)
	union := append(append([][2]int{}, a.Cells...), b.Cells...)
	if got := len(zoneEdgeRuns(cellSet(union...), union)); got != 4 {
		t.Fatalf("merged 4x2 block must outline as 4 runs, got %d", got)
	}
}

func TestZoneEdgeRunsLShape(t *testing.T) {
	t.Parallel()
	cells := [][2]int{{0, 0}, {1, 0}, {0, 1}}
	runs := zoneEdgeRuns(cellSet(cells...), cells)
	if len(runs) != 6 {
		t.Fatalf("an L outlines as 6 runs, got %d: %+v", len(runs), runs)
	}
}

func TestZoneGhostCellsColourPerCell(t *testing.T) {
	t.Parallel()
	can := func(x, y int) bool { return x != 1 }
	got := zoneGhostCells(2, 1, 0, 0, [4]int{-5, -5, 50, 50}, can) // dragged backwards
	if len(got) != 6 {
		t.Fatalf("3x2 drag = %d cells", len(got))
	}
	for _, c := range got {
		if c.ok != (c.x != 1) {
			t.Errorf("cell %+v: ok must follow CanZoneAt", c)
		}
	}
	if clipped := zoneGhostCells(0, 0, 100, 100, [4]int{10, 10, 12, 11}, can); len(clipped) != 6 {
		t.Fatalf("the ghost clips to the viewport, got %d", len(clipped))
	}
	if anyZoneCell(0, 0, 2, 2, func(x, y int) bool { return false }) {
		t.Fatal("no free cell expected")
	}
}

func TestZoneToolsMapToKinds(t *testing.T) {
	t.Parallel()
	want := map[string]string{"stockpile": game.ZoneStockpile, "grow": game.ZoneGrow, "pen": game.ZonePen, "mine": ""}
	for id, kind := range want {
		tool, ok := toolByID(id)
		if !ok || zoneKindOf(tool) != kind {
			t.Errorf("tool %s kind = %q, want %q", id, zoneKindOf(tool), kind)
		}
	}
}

func TestSecondZoneOverClaimedTilesHints(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.stockpile")
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 6, 6, 8, 7
	a.submitArchitect()
	g.Step(1)
	a.hint = ""
	a.archX0, a.archY0, a.archX1, a.archY1 = 6, 6, 8, 7
	a.submitArchitect()
	if a.hint != "No free tiles there" {
		t.Fatalf("hint = %q", a.hint)
	}
}

func TestZoneLabelOncePerZone(t *testing.T) {
	t.Parallel()
	z := rectZone(3, game.ZoneGrow, 4, 4, 6, 5)
	z.Crop = "potato"
	if zoneLabelText(z) != "Potato" || zoneLabelText(game.ZoneView{Kind: game.ZonePen}) != "Pen" {
		t.Fatal("label text")
	}
	g := (&Renderer{}).zoneGeomOf(z)
	if len(g.cells) != 6 {
		t.Fatalf("geom cells = %d", len(g.cells))
	}
}
