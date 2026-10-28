package overlay

import (
	"math"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestZoneEdgeMask(t *testing.T) {
	t.Parallel()
	z := rectZone(1, game.ZoneStockpile, 0, 0, 2, 2)
	set := cellSet(z.Cells...)
	cases := []struct {
		x, y, want int
	}{
		{1, 1, 0},
		{0, 0, ZoneN | ZoneW},
		{2, 0, ZoneN | ZoneE},
		{0, 2, ZoneS | ZoneW},
		{2, 2, ZoneS | ZoneE},
		{1, 0, ZoneN},
	}
	for _, c := range cases {
		if got := ZoneEdgeMask(set, c.x, c.y); got != c.want {
			t.Errorf("(%d,%d) mask = %d, want %d", c.x, c.y, got, c.want)
		}
	}
	if got := ZoneEdgeMask(cellSet([2]int{5, 5}), 5, 5); got != 15 {
		t.Fatalf("single tile mask = %d, want 15", got)
	}
}

func TestZoneEdgeRunsLShape(t *testing.T) {
	t.Parallel()
	cells := [][2]int{{0, 0}, {1, 0}, {0, 1}}
	runs := ZoneEdgeRuns(cellSet(cells...), cells)
	if len(runs) != 6 {
		t.Fatalf("an L outlines as 6 runs, got %d: %+v", len(runs), runs)
	}
}

func TestZoneLabelAnchor(t *testing.T) {
	t.Parallel()
	lShape := [][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}, {0, 1}, {0, 2}, {0, 3}, {0, 4}}
	for _, c := range []struct {
		name           string
		cells          [][2]int
		ax, ay         float64
		minX, maxX, my int
	}{
		{"odd rectangle: its centre cell's centre", rectZone(1, game.ZoneGrow, 4, 4, 6, 6).Cells, 5.5, 5.5, 4, 6, 6},
		{"even rectangle: its exact centre", rectZone(1, game.ZoneGrow, 4, 4, 7, 5).Cells, 6, 5, 4, 7, 5},
		{"a single cell", [][2]int{{9, 3}}, 9.5, 3.5, 9, 9, 3},
		// the bounding box centre (2.5, 2.5) is outside the L; the label
		// goes on the nearest cell, the first of the tied ones
		{"L shape: the cell nearest the bounding box centre", lShape, 2.5, 0.5, 0, 4, 4},
	} {
		minX, _, maxX, maxY, ax, ay := ZoneLabelAnchor(c.cells)
		if ax != c.ax || ay != c.ay || minX != c.minX || maxX != c.maxX || maxY != c.my {
			t.Errorf("%s: anchor (%g,%g) box x %d..%d maxY %d, want (%g,%g) x %d..%d maxY %d",
				c.name, ax, ay, minX, maxX, maxY, c.ax, c.ay, c.minX, c.maxX, c.my)
		}
	}
	// the anchor of a non-rectangular zone is always inside the zone
	set := cellSet(lShape...)
	_, _, _, _, ax, ay := ZoneLabelAnchor(lShape)
	if !set[[2]int{int(math.Floor(ax)), int(math.Floor(ay))}] {
		t.Errorf("L shape anchor (%g,%g) is outside the zone", ax, ay)
	}
}

func cellSet(cells ...[2]int) map[[2]int]bool {
	m := map[[2]int]bool{}
	for _, c := range cells {
		m[c] = true
	}
	return m
}

func rectZone(id uint16, kind string, minX, minY, maxX, maxY int) game.ZoneView {
	z := game.ZoneView{ID: id, Kind: kind, MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			z.Cells = append(z.Cells, [2]int{x, y})
		}
	}
	return z
}
