package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// PLAY-020: a sowing cell has one sower. A Sow job claims its tile, so a
// second grower picks another cell and a forced sow on it is refused.

// twoGrowers is clearGame plus Bo at (2,1), both with full needs, after
// a potato zone over x1,y1..x2,y2 is in place and both are idle.
func twoGrowers(t *testing.T, x1, y1, x2, y2 int) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g, ada := clearGame(t)
	bo, _ := g.SpawnNamedPawn(2, 1, "Bo")
	g.SetNeeds(bo, needs.Full, needs.Full)
	drawZone(t, g, "potato", x1, y1, x2, y2)
	g.Step(1)
	g.interrupt(ada, job.Cancelled)
	g.interrupt(bo, job.Cancelled)
	return g, ada, bo
}

func TestSowClaimsItsTile(t *testing.T) {
	g, ada, bo := twoGrowers(t, 5, 2, 8, 3)
	if !g.assignSow(ada) {
		t.Fatal("Ada found nothing to sow")
	}
	a := jobOf(g, ada)
	if a.Kind != job.Sow {
		t.Fatalf("Ada's job = %v, want Sow", a.Kind)
	}
	if owner, taken := g.reserve.TileOwner(a.TX, a.TY); !taken || owner != ada {
		t.Fatalf("Ada's sow cell %d,%d: owner %v taken %v, want Ada's claim", a.TX, a.TY, owner, taken)
	}
	if !g.assignSow(bo) {
		t.Fatal("Bo found nothing to sow")
	}
	b := jobOf(g, bo)
	if b.Kind != job.Sow || (b.TX == a.TX && b.TY == a.TY) {
		t.Fatalf("Bo's job %v at %d,%d, want Sow on another cell than Ada's %d,%d", b.Kind, b.TX, b.TY, a.TX, a.TY)
	}
}

func TestForcedSowRefusesAClaimedTile(t *testing.T) {
	g, ada, bo := twoGrowers(t, 5, 2, 8, 3)
	if !g.assignSow(ada) {
		t.Fatal("Ada found nothing to sow")
	}
	a := jobOf(g, ada)
	_, why := g.forcedJob(bo, command.OrderSow, a.TX, a.TY, construction.LayerObject)
	if !strings.HasPrefix(why, ReasonBusyTarget) {
		t.Fatalf("forced sow on Ada's cell: reason %q, want %q...", why, ReasonBusyTarget)
	}
	// a free cell is still fine
	_, why = g.forcedJob(bo, command.OrderSow, 8, 3, construction.LayerObject)
	if a.TX == 8 && a.TY == 3 {
		_, why = g.forcedJob(bo, command.OrderSow, 5, 2, construction.LayerObject)
	}
	if why != "" {
		t.Fatalf("forced sow on a free cell refused: %q", why)
	}
}

// TestTwoGrowersNeverShareASowCell: two growers sow a 12-cell field; at no
// tick do two of them hold Sow jobs on one cell, no Sow fails because its
// cell was taken, and the field ends fully sown.
func TestTwoGrowersNeverShareASowCell(t *testing.T) {
	g, ada, bo := twoGrowers(t, 5, 2, 10, 3)
	gone := 0
	for tick := 0; tick < 8000; tick++ {
		g.Step(1)
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.SetNeeds(bo, needs.Full, needs.Full)
		a, b := jobOf(g, ada), jobOf(g, bo)
		if a.Kind == job.Sow && b.Kind == job.Sow && a.Active() && b.Active() && a.TX == b.TX && a.TY == b.TY {
			t.Fatalf("tick %d: Ada and Bo both sowing %d,%d", tick, a.TX, a.TY)
		}
		for _, j := range []job.Record{a, b} {
			if j.Kind == job.Sow && j.State == job.Fail && j.Fail == job.TargetGone {
				gone++
			}
		}
		if sownCells(g, 5, 2, 10, 3) == 12 {
			break
		}
	}
	if n := sownCells(g, 5, 2, 10, 3); n != 12 {
		t.Fatalf("%d of 12 cells sown", n)
	}
	if gone != 0 {
		t.Fatalf("%d sow jobs failed with their cell taken", gone)
	}
}

// TestSowClaimSurvivesSaveLoad: a sow in progress keeps its claim through
// a save and load, so the other grower still can't take the cell.
func TestSowClaimSurvivesSaveLoad(t *testing.T) {
	g, ada, _ := twoGrowers(t, 5, 2, 8, 3)
	if !g.assignSow(ada) {
		t.Fatal("Ada found nothing to sow")
	}
	a := jobOf(g, ada)
	l := saveLoad(t, g)
	if owner, taken := l.reserve.TileOwner(a.TX, a.TY); !taken || owner != ada {
		t.Fatalf("after load the cell %d,%d: owner %v taken %v, want Ada's claim", a.TX, a.TY, owner, taken)
	}
}

// sownCells counts potato plants in the rectangle.
func sownCells(g *Game, x1, y1, x2, y2 int) int {
	n := 0
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			if pid := g.plantAt(x, y); pid != 0 {
				if p, ok := g.plants.Get(pid); ok && p.Species == plant.Potato {
					n++
				}
			}
		}
	}
	return n
}
