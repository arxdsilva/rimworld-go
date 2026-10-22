package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/stain"
)

// cleanGame is a dirt game in work hours with one cleaner (only cleaning
// switched on) standing at (3, 6) and Home over x 0..12.
func cleanGame(t testing.TB) (*Game, entity.EntityID) {
	t.Helper()
	g := dirtGame(t)
	scheduleAll(g, schedule.Work)
	id := cleaner(t, g, 3, 6, "Ada")
	g.paintHome(0, 0, 12, 13, true)
	return g, id
}

// cleaner spawns a colonist who only cleans.
func cleaner(t testing.TB, g *Game, x, y int, name string) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, name)
	if err != nil {
		t.Fatal(err)
	}
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(id, wt, 0)
	}
	g.setPriority(id, work.Clean, work.DefaultPriority)
	return id
}

func tileIdx(g *Game, x, y int) int { return y*g.grid.Width() + x }

func queued(g *Game, x, y int) bool {
	_, ok := slices.BinarySearch(g.filth.cleanQ, int32(tileIdx(g, x, y)))
	return ok
}

func TestCleanerCleansHomeOnly(t *testing.T) {
	t.Parallel()
	g, id := cleanGame(t)
	g.addDirt(tileIdx(g, 8, 6), 300)
	g.addDirt(tileIdx(g, 20, 6), 300)
	if !queued(g, 8, 6) || queued(g, 20, 6) {
		t.Fatal("only the Home tile queues")
	}
	sawClean := false
	for range 400 {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.Clean && j.State == job.Do {
			sawClean = true
			if pawn(t, g, id).Action != ActionClean {
				t.Fatalf("cleaning shows action %q", pawn(t, g, id).Action)
			}
		}
	}
	if !sawClean {
		t.Fatal("never cleaned")
	}
	if dirtAt(g, 8, 6) != 0 || queued(g, 8, 6) {
		t.Fatalf("Home tile left at %d", dirtAt(g, 8, 6))
	}
	if dirtAt(g, 20, 6) != 300 {
		t.Fatalf("outside Home cleaned to %d", dirtAt(g, 20, 6))
	}
}

func TestCleansBloodStains(t *testing.T) {
	t.Parallel()
	g, id := cleanGame(t)
	g.stamp(8, 6, stain.Blood, 300)
	g.addDirt(tileIdx(g, 8, 6), 100)
	if !queued(g, 8, 6) {
		t.Fatal("blood plus dirt not queued")
	}
	dirtGoneFirst := false
	for range 400 {
		g.Step(1)
		if s, ok := stainAt(g, 8, 6); dirtAt(g, 8, 6) == 0 && ok && s.Amount == 300 {
			dirtGoneFirst = true
		}
	}
	if !dirtGoneFirst {
		t.Fatal("the dirt is not swept before the blood")
	}
	if _, ok := stainAt(g, 8, 6); ok || queued(g, 8, 6) {
		t.Fatal("blood not cleaned")
	}
	if j, _ := g.jobs.Get(id); j.Kind == job.Clean && j.State == job.Do {
		t.Fatal("still cleaning a clean tile")
	}
}

func TestCleanNoFlap(t *testing.T) {
	t.Parallel()
	g, id := cleanGame(t)
	// a tile just under the start threshold is never queued
	g.addDirt(tileIdx(g, 8, 6), filth.CleanStart-1)
	if queued(g, 8, 6) {
		t.Fatal("119 queued")
	}
	if n := cleanJobs(g, id, 300); n != 0 || dirtAt(g, 8, 6) != filth.CleanStart-1 {
		t.Fatalf("a tile below the threshold drew %d cleaning ticks, left %d", n, dirtAt(g, 8, 6))
	}
	// cleaned to 0, then dirtied to 60: not re-queued
	g.addDirt(tileIdx(g, 8, 6), 200)
	g.Step(300)
	if dirtAt(g, 8, 6) != 0 {
		t.Fatalf("queued tile left at %d", dirtAt(g, 8, 6))
	}
	g.addDirt(tileIdx(g, 8, 6), 60)
	if queued(g, 8, 6) {
		t.Fatal("a clean tile re-queued at 60")
	}
	if n := cleanJobs(g, id, 300); n != 0 || dirtAt(g, 8, 6) != 60 {
		t.Fatalf("a tile at 60 drew %d cleaning ticks, left %d", n, dirtAt(g, 8, 6))
	}
	// interrupted at 40: stays queued and is finished later
	g.addDirt(tileIdx(g, 4, 9), 300)
	for range 400 {
		g.Step(1)
		if dirtAt(g, 4, 9) <= 40 {
			break
		}
	}
	if d := dirtAt(g, 4, 9); d == 0 || d > 40 {
		t.Fatalf("cleaning stopped at %d", d)
	}
	g.interrupt(id, job.Interrupted)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	g.Step(50)
	if !queued(g, 4, 9) || dirtAt(g, 4, 9) > 40 || dirtAt(g, 4, 9) == 0 {
		t.Fatalf("interrupted tile queued=%v at %d", queued(g, 4, 9), dirtAt(g, 4, 9))
	}
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: false})
	g.Step(300)
	if dirtAt(g, 4, 9) != 0 || queued(g, 4, 9) {
		t.Fatalf("interrupted tile not resumed: %d", dirtAt(g, 4, 9))
	}
}

// cleanJobs steps n ticks and counts the ticks id spent on a Clean job.
func cleanJobs(g *Game, id entity.EntityID, n int) int {
	c := 0
	for range n {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.Clean {
			c++
		}
	}
	return c
}

func TestTwoCleanersDistinctTiles(t *testing.T) {
	t.Parallel()
	g, a := cleanGame(t)
	b := cleaner(t, g, 3, 7, "Bo")
	g.addDirt(tileIdx(g, 9, 6), 400)
	g.addDirt(tileIdx(g, 9, 7), 400)
	g.Step(1)
	ja, _ := g.jobs.Get(a)
	jb, _ := g.jobs.Get(b)
	if ja.Kind != job.Clean || jb.Kind != job.Clean {
		t.Fatalf("jobs %v, %v", ja.Kind, jb.Kind)
	}
	if ja.DestX == jb.DestX && ja.DestY == jb.DestY {
		t.Fatalf("both clean %d,%d", ja.DestX, ja.DestY)
	}
	if ja.Count != (400+filth.CleanPerTick-1)/filth.CleanPerTick+10 || g.workTotal(a, ja) != ja.Count {
		t.Fatalf("clean work %d, total %d", ja.Count, g.workTotal(a, ja))
	}
	if g.workCell(a, Position{}) != (Position{X: ja.DestX, Y: ja.DestY}) {
		t.Fatal("the work cell is not the dirty tile")
	}
	g.Step(400)
	if dirtAt(g, 9, 6) != 0 || dirtAt(g, 9, 7) != 0 {
		t.Fatal("tiles left dirty")
	}
}

func TestCleanTargetGone(t *testing.T) {
	t.Parallel()
	g, id := cleanGame(t)
	j := job.Record{Kind: job.Clean, State: job.Do, DestX: 8, DestY: 6, WorkLeft: 20, Count: 20}
	g.jobs.Set(id, j)
	g.doClean(id, j)
	if r, _ := g.jobs.Get(id); r.State != job.Fail || r.Fail != job.TargetGone {
		t.Fatalf("a clean tile gave %+v", r)
	}
	g.addDirt(tileIdx(g, 8, 6), 300)
	g.paintHome(8, 6, 8, 6, false)
	if queued(g, 8, 6) {
		t.Fatal("cleared Home still queued")
	}
	g.doClean(id, j)
	if r, _ := g.jobs.Get(id); r.State != job.Fail {
		t.Fatalf("outside Home gave %+v", r)
	}
	// cleaning the last of a tile someone else swept completes
	g.paintHome(8, 6, 8, 6, true)
	g.addDirt(tileIdx(g, 8, 6), -300)
	j.WorkLeft = 5
	g.doClean(id, j)
	if r, _ := g.jobs.Get(id); r.State != job.Complete {
		t.Fatalf("started tile now clean gave %+v", r)
	}
}

func TestWorkCleanToggle(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id, _ := g.SpawnNamedPawn(3, 3, "Ada")
	if !pawn(t, g, id).WorkClean {
		t.Fatal("a new colonist does not clean")
	}
	g.Enqueue(command.Command{Kind: command.SetWorkEnabled, Pawn: uint64(id), Work: command.WorkClean, Enabled: false})
	g.Step(1)
	if pawn(t, g, id).WorkClean {
		t.Fatal("WorkClean toggle ignored")
	}
	// an old save's colonist without cleaning gets it once
	g.wireCleaning()
	if !pawn(t, g, id).WorkClean || g.prioOf(id)[work.Clean] != work.DefaultPriority {
		t.Fatal("wireCleaning left cleaning off")
	}
}

// BenchmarkAssignClean picks a tile among 4000 queued (gate: <= 0.2 ms).
func BenchmarkAssignClean(b *testing.B) {
	g := New(Config{Seed: 1, Width: 100, Height: 100})
	scheduleAll(g, schedule.Work)
	id := cleaner(b, g, 50, 50, "Ada")
	g.paintHome(0, 0, 99, 99, true)
	for k := range 4000 {
		x, y := (k*37)%100, (k*61+k/100)%100
		g.addDirt(y*100+x, 200)
	}
	if len(g.filth.cleanQ) < 3000 {
		b.Fatalf("only %d queued", len(g.filth.cleanQ))
	}
	for b.Loop() {
		if !g.assignClean(id) {
			b.Fatal("no clean job")
		}
		g.reserve.ReleaseOwner(id)
		g.jobs.Set(id, job.Record{})
	}
}
