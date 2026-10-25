package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// shovelGame is a frosty snowGame (nothing melts or falls) in work hours
// with one colonist who only cleans, at (3, 6).
func shovelGame(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := snowGame(t, -50)
	scheduleAll(g, schedule.Work)
	return g, cleaner(t, g, 3, 6, "Ada")
}

func TestClearSnowJobEmptiesAreaCells(t *testing.T) {
	t.Parallel()
	g, id := shovelGame(t)
	g.Enqueue(command.Command{Kind: command.DesignateSnowClear, X: 8, Y: 5, X2: 10, Y2: 6, Enabled: true})
	g.Step(1)
	for x := 8; x <= 10; x++ {
		g.setSnowCell(snowIdx(g, x, 5), 200)
	}
	g.setSnowCell(snowIdx(g, 8, 6), snowClearStart-1) // too shallow to bother
	g.setSnowCell(snowIdx(g, 14, 5), 200)             // outside the area
	saw := false
	for range 900 {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.ClearSnow && j.State == job.Do {
			saw = true
			if pv := pawn(t, g, id); pv.Action != ActionClean || pv.ActionProgress < 0 {
				t.Fatalf("shovelling shows action %q, progress %v", pv.Action, pv.ActionProgress)
			}
		}
	}
	if !saw {
		t.Fatal("never cleared snow")
	}
	for x := 8; x <= 10; x++ {
		if d := g.snowAt(x, 5); d != 0 {
			t.Fatalf("area cell %d,5 left at %d", x, d)
		}
	}
	if d := g.snowAt(8, 6); d != snowClearStart-1 {
		t.Fatalf("shallow area cell changed to %d", d)
	}
	if d := g.snowAt(14, 5); d != 200 {
		t.Fatalf("outside cell changed to %d", d)
	}
}

// TestClearSnowNoFlap: a cleared cell is taken up again only once
// snowClearStart has built up on it, not at the first flake.
func TestClearSnowNoFlap(t *testing.T) {
	t.Parallel()
	g, id := shovelGame(t)
	g.paintSnowClear(8, 5, 8, 5, true)
	g.setSnowCell(snowIdx(g, 8, 5), snowClearStart-1)
	if g.assignClearSnow(id) {
		t.Fatal("snow under the start depth is left alone")
	}
	g.setSnowCell(snowIdx(g, 8, 5), snowClearStart)
	if !g.assignClearSnow(id) {
		t.Fatal("snow at the start depth is cleared")
	}
	if j, _ := g.jobs.Get(id); j.Kind != job.ClearSnow || j.DestX != 8 || j.DestY != 5 || j.WorkLeft != snowClearWork(snowClearStart) {
		t.Fatalf("job %+v", j)
	}
}

func TestClearSnowSkipsReservedCells(t *testing.T) {
	t.Parallel()
	g, id := shovelGame(t)
	other := cleaner(t, g, 20, 6, "Bea")
	g.paintSnowClear(8, 5, 8, 5, true)
	g.setSnowCell(snowIdx(g, 8, 5), 120)
	g.reserve.TryTile(other, 8, 5)
	if g.assignClearSnow(id) {
		t.Fatal("a cell someone else holds is not taken")
	}
}

func TestClearSnowRespectsAllowedArea(t *testing.T) {
	t.Parallel()
	g, id := shovelGame(t)
	g.paintHome(0, 0, 5, 10, true)
	g.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(id), Index: command.AreaHome})
	g.Step(1)
	if g.inArea(id, 8, 5) {
		t.Skip("allowed areas do not restrict to Home yet (AREA-CORE)")
	}
	g.paintSnowClear(8, 5, 8, 5, true)
	g.setSnowCell(snowIdx(g, 8, 5), 120)
	if g.assignClearSnow(id) {
		t.Fatal("a cell outside the allowed area is not taken")
	}
	g.paintSnowClear(4, 5, 4, 5, true)
	g.setSnowCell(snowIdx(g, 4, 5), 120)
	if !g.assignClearSnow(id) {
		t.Fatal("a cell inside the allowed area is taken")
	}
}

func TestClearSnowJobEndsWhenAreaRemoved(t *testing.T) {
	t.Parallel()
	g, id := shovelGame(t)
	g.paintSnowClear(4, 6, 4, 6, true)
	g.setSnowCell(snowIdx(g, 4, 6), 250)
	if !g.assignClearSnow(id) {
		t.Fatal("no job")
	}
	g.paintSnowClear(4, 6, 4, 6, false)
	g.Step(60)
	if j, _ := g.jobs.Get(id); j.Kind == job.ClearSnow && j.State == job.Do {
		t.Fatal("the job outlives its area")
	}
	if d := g.snowAt(4, 6); d != 250 {
		t.Fatalf("depth %d, want untouched", d)
	}
}

func TestDesignateSnowClearPaintsAndClears(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -50)
	g.Enqueue(command.Command{Kind: command.DesignateSnowClear, X: 5, Y: 4, X2: 2, Y2: 2, Enabled: true})
	g.Step(1)
	v0 := g.snow.clearVer
	if !g.inSnowClear(snowIdx(g, 2, 2)) || !g.inSnowClear(snowIdx(g, 5, 4)) || g.inSnowClear(snowIdx(g, 6, 4)) {
		t.Fatal("rectangle painted")
	}
	g.Enqueue(command.Command{Kind: command.DesignateSnowClear, X: 3, Y: 3, X2: 3, Y2: 4})
	g.Step(1)
	if g.inSnowClear(snowIdx(g, 3, 3)) || !g.inSnowClear(snowIdx(g, 2, 3)) || g.snow.clearVer == v0 {
		t.Fatal("rectangle cleared")
	}
	v1 := g.snow.clearVer
	g.paintSnowClear(3, 3, 3, 4, false)
	g.paintSnowClear(-5, -5, -1, -1, true)
	if g.snow.clearVer != v1 {
		t.Fatal("a no-op paint keeps the version")
	}
}

func TestSnowClearWorkScalesWithDepth(t *testing.T) {
	t.Parallel()
	if a, b := snowClearWork(snowClearStart), snowClearWork(255); a < snowClearMinWork || b != snowClearMaxWork || a >= b {
		t.Fatalf("work %d..%d", a, b)
	}
}
