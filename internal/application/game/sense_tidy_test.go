package game

import (
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
)

// tidyGame is a cleaner at (3, 6) in a roofed hut over (1, 2)..(9, 9).
func tidyGame(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g, id := cleanGame(t)
	hut(t, g, 1, 2, 9, 9)
	return g, id
}

// dirty lays a queueable dirt on each tile.
func dirty(g *Game, tiles ...[2]int) {
	for _, c := range tiles {
		g.addDirt(tileIdx(g, c[0], c[1]), 2*filth.CleanStart)
	}
}

func tidyBenchAt(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	if !g.BuildNow("research bench", "wood", "south", x, y) {
		t.Fatal("build bench")
	}
	for _, b := range g.bldgIDs {
		if v, ok := g.bldgs.Get(b); ok && v.X == x && v.Y == y {
			return b
		}
	}
	t.Fatal("no bench")
	return 0
}

func jobKind(g *Game, id entity.EntityID) job.Kind {
	j, _ := g.jobs.Get(id)
	return j.Kind
}

// sweepUntilEnd steps until the tidy is over.
func sweepUntilEnd(t *testing.T, g *Game, id entity.EntityID) {
	t.Helper()
	for range 3000 {
		g.Step(1)
		if jobKind(g, id) != job.CleanRoom {
			return
		}
	}
	t.Fatal("the tidy never ended")
}

func TestTidyCleansRoomThenResumes(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4}, [2]int{7, 6}, [2]int{3, 3})
	bench := tidyBenchAt(t, g, 5, 4)
	rec := job.Record{Kind: job.Research, Target: bench, TX: 5, TY: 6}
	if !g.startJob(id, rec) {
		t.Fatal("start")
	}
	j, _ := g.jobs.Get(id)
	if j.Kind != job.CleanRoom || len(j.Route) != 3 {
		t.Fatalf("job %v route %v, want a tidy of 3 tiles", j.Kind, j.Route)
	}
	if res := senseOf(g).Resume[id]; res.Kind != job.Research || res.Target != bench || res.Path != nil {
		t.Fatalf("resume %+v", res)
	}
	if pawn(t, g, id).Job == "" {
		t.Fatal("no job line while tidying")
	}
	sweepUntilEnd(t, g, id)
	if k := jobKind(g, id); k != job.Research {
		t.Fatalf("after the tidy: %v, want Research", k)
	}
	for _, c := range [][2]int{{7, 4}, {7, 6}, {3, 3}} {
		if dirtAt(g, c[0], c[1]) != 0 {
			t.Fatalf("tile %v left at %d", c, dirtAt(g, c[0], c[1]))
		}
	}
	if len(senseOf(g).Resume) != 0 {
		t.Fatal("resume kept")
	}
}

func TestTidyNeverInterceptsOrdersOrDraft(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	if g.startJob(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6, Forced: true}); jobKind(g, id) != job.Recreate {
		t.Fatalf("a forced job became %v", jobKind(g, id))
	}
	g.draft[id] = true
	if g.startJob(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6}); jobKind(g, id) != job.Recreate {
		t.Fatalf("a drafted pawn got %v", jobKind(g, id))
	}
	g.draft[id] = false
	for _, k := range []job.Kind{job.Move, job.Attack, job.FightFire, job.Tend, job.Clean, job.Sleep} {
		if _, ok := senseOf(g).Tidy.Before(id, job.Record{Kind: k, TX: 5, TY: 6}); ok {
			t.Fatalf("%v was intercepted", k)
		}
	}
}

func TestTidyOffKeepsTheJob(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.CleanBeforeWork, false)
	if _, ok := s.Tidy.Before(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6}); ok {
		t.Fatal("tidied with the rule off")
	}
	if _, ok := s.TidyAt[id]; ok {
		t.Fatal("no cooldown without a try")
	}
}

// A tidy never loops: the cooldown holds through the way back in, and dirt
// tracked in behind a finished tidy waits for it.
func TestTidyDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	rec := job.Record{Kind: job.Recreate, TX: 5, TY: 6}
	tidies := 0
	var last job.Kind
	for range flapSpan {
		if dirtAt(g, 7, 4) < filth.CleanStart {
			dirty(g, [2]int{7, 4})
		}
		if j, _ := g.jobs.Get(id); j.Kind != job.CleanRoom && j.Kind != job.Recreate || j.State == job.Complete {
			g.startJob(id, rec)
		}
		if k := jobKind(g, id); k == job.CleanRoom && last != job.CleanRoom {
			tidies++
		}
		last = jobKind(g, id)
		g.Step(1)
	}
	if tidies == 0 || tidies > 2 {
		t.Fatalf("%d tidies in %d ticks, the cooldown allows 1 or 2", tidies, flapSpan)
	}
}

const flapSpan = 2000

// A colonist whose only dirt is taken doesn't retry each tick.
func TestTidyUnreachableDoesNotRetry(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	g.reserve.TryTile(entity.EntityID(9999), 7, 4)
	s := senseOf(g)
	if _, ok := s.Tidy.Before(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6}); ok {
		t.Fatal("tidied a taken tile")
	}
	until := s.TidyAt[id]
	if until == 0 {
		t.Fatal("no cooldown after a failed try")
	}
	g.reserve.ReleaseOwner(entity.EntityID(9999))
	if _, ok := s.Tidy.Before(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6}); ok || s.TidyAt[id] != until {
		t.Fatalf("retried at once: ok %v, until %d then %d", ok, until, s.TidyAt[id])
	}
	g.clock.Advance(int(until - uint64(g.clock.Now())))
	if _, ok := s.Tidy.Before(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6}); !ok {
		t.Fatal("no retry after the cooldown")
	}
}

func TestTidyResumeSurvivesSave(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4}, [2]int{7, 6})
	if !g.startJob(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6, DestX: 5, DestY: 6, WorkLeft: 50}) {
		t.Fatal("start")
	}
	g.Step(3)
	if jobKind(g, id) != job.CleanRoom {
		t.Fatalf("job %v", jobKind(g, id))
	}
	h := saveLoad(t, g)
	if a, b := g.StateHash(), h.StateHash(); a != b {
		t.Fatalf("hash %x != %x mid-tidy", a, b)
	}
	if h2 := senseOf(h).Resume[id]; h2.Kind != job.Recreate || h2.WorkLeft != 50 {
		t.Fatalf("resume after load: %+v", h2)
	}
	g.Step(600)
	h.Step(600)
	if a, b := g.StateHash(), h.StateHash(); a != b {
		t.Fatalf("hash %x != %x after the tidy", a, b)
	}
}

// What a giver claims after startJob stays with the pawn through the tidy
// and comes back with the job; what startJob itself claimed (the target) is
// claimed again on the way back.
func TestTidySwapKeepsClaims(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4}, [2]int{3, 3})
	bench := tidyBenchAt(t, g, 5, 4)
	rec := job.Record{Kind: job.Research, Target: bench, TX: 5, TY: 6}
	if !g.startJob(id, rec) || !g.reserve.TryTile(id, rec.TX, rec.TY) { // research.go does this
		t.Fatal("start")
	}
	if jobKind(g, id) != job.CleanRoom {
		t.Fatalf("job %v", jobKind(g, id))
	}
	if _, held := g.reserve.EntityOwner(bench); held {
		t.Fatal("the tidy holds the bench it was given no claim on")
	}
	for range 3000 {
		if jobKind(g, id) != job.CleanRoom {
			break
		}
		if o, ok := g.reserve.TileOwner(rec.TX, rec.TY); !ok || o != id {
			t.Fatalf("spot released mid-tidy: %v %v", o, ok)
		}
		g.Step(1)
	}
	if o, ok := g.reserve.TileOwner(rec.TX, rec.TY); !ok || o != id {
		t.Fatalf("resumed job lost its spot: %v %v", o, ok)
	}
	if o, ok := g.reserve.EntityOwner(bench); !ok || o != id {
		t.Fatalf("resumed job lost its bench: %v %v", o, ok)
	}
	if _, ok := g.reserve.TileOwner(7, 4); ok {
		t.Fatal("swept tile still claimed")
	}
}

func TestResumeDroppedOnInterrupt(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	if !g.startJob(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6}) {
		t.Fatal("start")
	}
	g.interrupt(id, job.Interrupted)
	if len(senseOf(g).Resume) != 0 {
		t.Fatal("an interrupted job stays resumable")
	}
	g.Step(5)
	if jobKind(g, id) == job.Recreate {
		t.Fatal("the job came back")
	}
}

// A forced order that overwrites a tidy drops the job behind it too.
func TestResumeDroppedByOrder(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	if !g.startJob(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6}) {
		t.Fatal("start")
	}
	g.startJob(id, job.Record{Kind: job.Move, TX: 3, TY: 3, Forced: true})
	if len(senseOf(g).Resume) != 0 {
		t.Fatal("kept behind a forced order")
	}
}

// A resumed job whose target went fails like any job: no panic, no retry
// through Resume, and the pawn looks for work.
func TestResumeFailsQuietly(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	bench := tidyBenchAt(t, g, 5, 4)
	if !g.startJob(id, job.Record{Kind: job.Research, Target: bench, TX: 5, TY: 6}) {
		t.Fatal("start")
	}
	g.reserve.TryEntity(entity.EntityID(9999), bench)
	sweepUntilEnd(t, g, id)
	if len(senseOf(g).Resume) != 0 {
		t.Fatal("resume kept after failing")
	}
	if j, _ := g.jobs.Get(id); j.Kind == job.Research && j.State != job.Fail {
		t.Fatalf("resumed a job it can't claim: %+v", j)
	}
}

func TestIncapableNeverTidies(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	s := senseOf(g)
	rec := job.Record{Kind: job.Recreate, TX: 5, TY: 6}
	g.setPriority(id, work.Clean, 0)
	if _, ok := s.Tidy.Before(id, rec); ok {
		t.Fatal("cleaning priority off")
	}
	g.setPriority(id, work.Clean, work.DefaultPriority)
	g.raiders[id] = raid.Raider{}
	if _, ok := s.Tidy.Before(id, rec); ok {
		t.Fatal("not a colonist")
	}
	delete(g.raiders, id)
	if _, ok := s.Tidy.Before(id, rec); !ok {
		t.Fatal("a capable colonist should tidy")
	}
}

func TestUnlimitedIsBounded(t *testing.T) {
	t.Parallel()
	g, id := cleanGame(t)
	g.paintHome(0, 0, 14, 13, true)
	hut(t, g, 1, 1, 13, 13)
	var all [][2]int
	for y := 2; y < 13; y++ {
		for x := 2; x < 13; x++ {
			all = append(all, [2]int{x, y})
		}
	}
	dirty(g, all...)
	s := senseOf(g)
	r, ok := s.Tidy.Before(id, job.Record{Kind: job.Recreate, TX: 7, TY: 7})
	if !ok || len(r.Route) != gameplay.LimitDefault(gameplay.TidyStation) {
		t.Fatalf("route %d, want the default limit", len(r.Route))
	}
	s.Tidy.Forget(id)
	delete(s.TidyAt, id)
	s.Rules = s.Rules.WithLimit(gameplay.TidyStation, 0)
	if r, ok = s.Tidy.Before(id, job.Record{Kind: job.Recreate, TX: 7, TY: 7}); !ok || len(r.Route) != 64 {
		t.Fatalf("unlimited route %d, want the bound of 64", len(r.Route))
	}
}

func TestTidyHugeRoomUsesRadius(t *testing.T) {
	t.Parallel()
	g, id := cleanGame(t)
	g.paintHome(0, 0, 29, 13, true)
	hut(t, g, 1, 1, 22, 12)
	dirty(g, [2]int{4, 6}, [2]int{20, 6})
	r, ok := senseOf(g).Tidy.Before(id, job.Record{Kind: job.Recreate, TX: 5, TY: 6})
	if !ok || len(r.Route) != 1 {
		t.Fatalf("route %v, want only the near tile", r.Route)
	}
	if i := int(r.Route[0]); i != tileIdx(g, 4, 6) {
		t.Fatalf("tile %d", i)
	}
}

func TestCleanBeforeMealResumes(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.CleanBeforeMeal, true)
	n, _ := g.need.Get(id)
	n.Hunger = needs.RavenousAt + 100
	g.need.Set(id, n)
	meal := stack(t, g, item.Berries, 1, 5, 6)
	if !g.startJob(id, job.Record{Kind: job.Eat, Target: meal, TX: 5, TY: 6}) || jobKind(g, id) != job.CleanRoom {
		t.Fatalf("no tidy before a meal: %v", jobKind(g, id))
	}
	if g.noFood[id] <= uint64(g.clock.Now()) {
		t.Fatal("the hunger check isn't held off")
	}
	sweepUntilEnd(t, g, id)
	if j, _ := g.jobs.Get(id); j.Kind != job.Eat || j.Target != meal {
		t.Fatalf("back as %+v, want the meal", j)
	}
	if g.noFood[id] != 0 {
		t.Fatal("hold outlived the tidy")
	}
}

func TestCleanBeforeMealOff(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	if _, ok := senseOf(g).Tidy.Before(id, job.Record{Kind: job.Eat, TX: 5, TY: 6}); ok {
		t.Fatal("off by default: tidied before a meal")
	}
}

func TestStarvingSkipsClean(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.CleanBeforeMeal, true)
	n, _ := g.need.Get(id)
	n.Hunger = needs.RavenousAt
	g.need.Set(id, n)
	if _, ok := s.Tidy.Before(id, job.Record{Kind: job.Eat, TX: 5, TY: 6}); ok {
		t.Fatal("a ravenous colonist stopped to tidy")
	}
}

func TestCleanAfterTend(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	patient, _ := g.SpawnNamedPawn(4, 6, "Bea")
	dirty(g, [2]int{7, 4})
	s := senseOf(g)
	done := job.Record{Kind: job.Tend, State: job.Complete, Target: patient}
	if !s.Tidy.After(id, done) || jobKind(g, id) != job.CleanRoom {
		t.Fatalf("no tidy after a tend: %v", jobKind(g, id))
	}
	if len(s.Resume) != 0 {
		t.Fatal("a tidy after work resumes nothing")
	}
	g.jobs.Set(id, job.Record{})
	if s.Tidy.After(id, done) {
		t.Fatal("the cooldown holds after work too")
	}
	delete(s.TidyAt, id)
	if unspent := (job.Record{Kind: job.Tend, State: job.Complete, Target: patient, WorkLeft: 5}); s.Tidy.After(id, unspent) {
		t.Fatal("a tend that did nothing tidied")
	}
	s.Rules = s.Rules.With(gameplay.CleanAfterTend, false)
	if s.Tidy.After(id, done) {
		t.Fatal("rule off")
	}
}

func TestCleanAfterDelivery(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	patient, _ := g.SpawnNamedPawn(4, 6, "Bea")
	dirty(g, [2]int{7, 4})
	s := senseOf(g)
	if s.Tidy.After(id, job.Record{Kind: job.FeedPatient, State: job.Complete, Dest: patient, Carry: 7}) {
		t.Fatal("still holding the meal")
	}
	far, _ := g.SpawnNamedPawn(8, 8, "Cid")
	if s.Tidy.After(id, job.Record{Kind: job.DeliverFood, State: job.Complete, Dest: far}) {
		t.Fatal("not beside the patient: it never delivered")
	}
	if !s.Tidy.After(id, job.Record{Kind: job.DeliverFood, State: job.Complete, Dest: patient}) || jobKind(g, id) != job.CleanRoom {
		t.Fatalf("no tidy after a delivery: %v", jobKind(g, id))
	}
}

func billRecord(bench, load entity.EntityID) job.Record {
	return job.Record{Kind: job.Bill, Target: bench, Dest: bench, DestX: 5, DestY: 6, TX: 5, TY: 6, Carry: load, Route: []entity.EntityID{}}
}

func TestBillCleansBeforeWork(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	bench := tidyBenchAt(t, g, 5, 4)
	laid := stack(t, g, item.Wood, 2, 6, 6)
	load := stack(t, g, item.Wood, 2, 3, 6)
	st, _ := g.stacks.Get(load)
	st.Carrier = id
	g.stacks.Set(load, st)
	j := billRecord(bench, load)
	j.Route = []entity.EntityID{laid}
	s := senseOf(g)
	if !s.Tidy.TidyBench(id, bench, j) || jobKind(g, id) != job.CleanRoom {
		t.Fatalf("no tidy before the bill: %v", jobKind(g, id))
	}
	if o, ok := g.reserve.EntityOwner(bench); !ok || o != id {
		t.Fatal("the bench went")
	}
	if o, ok := g.reserve.EntityOwner(laid); !ok || o != id {
		t.Fatal("the laid-out load went")
	}
	if o, ok := g.reserve.TileOwner(5, 6); !ok || o != id {
		t.Fatal("the work spot went")
	}
	if cur, _ := g.jobs.Get(id); cur.Carry != 0 {
		t.Fatal("the tidy mustn't hold the load: the job-end seam would drop it")
	}
	sweepUntilEnd(t, g, id)
	back, _ := g.jobs.Get(id)
	if back.Kind != job.Bill || back.Carry != load || back.Dest != bench {
		t.Fatalf("bill back as %+v", back)
	}
	if st, _ := g.stacks.Get(load); st.Carrier != id {
		t.Fatal("the load was dropped")
	}
	if o, ok := g.reserve.EntityOwner(laid); !ok || o != id {
		t.Fatal("the laid-out load went after resuming")
	}
}

func TestResumeDropDropsTheLoad(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	bench := tidyBenchAt(t, g, 5, 4)
	load := stack(t, g, item.Wood, 2, 3, 6)
	st, _ := g.stacks.Get(load)
	st.Carrier = id
	g.stacks.Set(load, st)
	if !senseOf(g).Tidy.TidyBench(id, bench, billRecord(bench, load)) {
		t.Fatal("no tidy")
	}
	g.interrupt(id, job.Interrupted)
	if st, ok := g.stacks.Get(load); ok && st.Carrier != 0 {
		t.Fatal("an interrupted pawn keeps a load for a job gone")
	}
}

func TestBillCleanRespectsLimit(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4}, [2]int{7, 5}, [2]int{7, 6}, [2]int{7, 7})
	bench := tidyBenchAt(t, g, 5, 4)
	s := senseOf(g)
	s.Rules = s.Rules.WithLimit(gameplay.TidyBench, 2)
	if !s.Tidy.TidyBench(id, bench, billRecord(bench, 0)) {
		t.Fatal("no tidy")
	}
	if j, _ := g.jobs.Get(id); len(j.Route) != 2 {
		t.Fatalf("route %d, want the limit", len(j.Route))
	}
}

func TestBillCleanDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	bench := tidyBenchAt(t, g, 5, 4)
	s := senseOf(g)
	if !s.Tidy.TidyBench(id, bench, billRecord(bench, 0)) {
		t.Fatal("no tidy")
	}
	sweepUntilEnd(t, g, id)
	dirty(g, [2]int{7, 4})
	if s.Tidy.TidyBench(id, bench, billRecord(bench, 0)) {
		t.Fatal("tidied twice within the cooldown")
	}
}

func TestBenchCleanCommand(t *testing.T) {
	t.Parallel()
	g, id := tidyGame(t)
	dirty(g, [2]int{7, 4})
	bench := tidyBenchAt(t, g, 5, 4)
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.BenchCleanToggle, true)
	g.Enqueue(command.Command{Kind: command.SetBenchClean, Target: uint64(bench), Enabled: false})
	g.Enqueue(command.Command{Kind: command.SetBenchClean, Target: 987654, Enabled: false})
	g.Step(1)
	if !s.NoClean[bench] || s.NoClean[987654] {
		t.Fatalf("no clean %v", s.NoClean)
	}
	g.jobs.Set(id, job.Record{})
	g.reserve.ReleaseOwner(id) // the step gave it the tile as plain cleaning
	v, _ := g.bldgs.Get(bench)
	bv := BuildingView{}
	g.fillBuildingView(&bv, bench, v)
	if !bv.CleanOff {
		t.Fatal("read model doesn't show it")
	}
	if s.Tidy.TidyBench(id, bench, billRecord(bench, 0)) {
		t.Fatal("a bench switched off tidied")
	}
	if _, ok := s.Tidy.Before(id, job.Record{Kind: job.Research, Target: bench, TX: 5, TY: 6}); ok {
		t.Fatal("a bench switched off tidied before research")
	}
	s.Rules = s.Rules.With(gameplay.BenchCleanToggle, false)
	if !s.Tidy.TidyBench(id, bench, billRecord(bench, 0)) {
		t.Fatal("the per-bench switch only counts with its rule on")
	}
}

func TestBenchCleanPruned(t *testing.T) {
	t.Parallel()
	g, _ := tidyGame(t)
	bench := tidyBenchAt(t, g, 5, 4)
	s := senseOf(g)
	s.Tidy.SetBench(bench, false)
	g.removeBuilding(bench)
	s.Tidy.SetBench(entity.EntityID(987654), false)
	if len(s.NoClean) != 0 {
		t.Fatalf("no clean %v", s.NoClean)
	}
}

// plainHut is hut for tests and benchmarks alike.
func plainHut(g *Game) {
	for x := 1; x <= 9; x++ {
		g.BuildNow("wall", "wood", "south", x, 2)
		if x == 5 {
			g.BuildNow("door", "wood", "south", x, 9)
			continue
		}
		g.BuildNow("wall", "wood", "south", x, 9)
	}
	for y := 3; y < 9; y++ {
		g.BuildNow("wall", "wood", "south", 1, y)
		g.BuildNow("wall", "wood", "south", 9, y)
	}
	g.RoofRooms()
}

// detourGame is a colonist in a dirty hut on a wide map.
func detourGame(t testing.TB, width int) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 3, Width: width, Height: 14})
	jumpTo(g, 10, 12)
	clear(g.seedGrass().cells)
	id := cleaner(t, g, 3, 6, "Ada")
	g.paintHome(0, 0, 12, 13, true)
	plainHut(g)
	dirty(g, [2]int{7, 4})
	return g, id
}

func TestDetourSkipsFarFetch(t *testing.T) {
	t.Parallel()
	for _, full := range []bool{true, false} {
		g, id := detourGame(t, 90)
		s := senseOf(g)
		s.Rules = s.Rules.With(gameplay.FullPathCheck, full)
		bill := job.Record{Kind: job.Bill, DestX: 5, DestY: 6, TX: 85, TY: 8}
		if _, ok := s.Tidy.Before(id, bill); ok {
			t.Fatalf("full %v: the bench is back the way the fetch isn't, and it tidied", full)
		}
	}
}

func TestDetourOnTheWayTidies(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ x, y, fx, fy int }{
		"next door": {4, 6, 7, 7},
		"far side":  {85, 8, 7, 7}, // the bench lies on the way to the fetch
	} {
		g, id := detourGame(t, 90)
		g.positions.Set(id, Position{X: c.x, Y: c.y})
		if _, ok := senseOf(g).Tidy.Before(id, job.Record{Kind: job.Bill, DestX: 5, DestY: 6, TX: c.fx, TY: c.fy}); !ok {
			t.Fatalf("%s: no detour here, it should tidy", name)
		}
	}
}

func TestDetourDeferredSkips(t *testing.T) {
	t.Parallel()
	g, id := detourGame(t, 130)
	b := &g.regs.budget
	b.tick, b.used = uint64(g.clock.Now()), longSearchBudget
	if _, ok := senseOf(g).Tidy.Before(id, job.Record{Kind: job.Bill, DestX: 5, DestY: 6, TX: 125, TY: 8}); ok {
		t.Fatal("a tidy on a deferred search")
	}
}

// The gate on startJob's path: a clean room, with dirt elsewhere.
func tidyGateGame(t testing.TB) (*Game, entity.EntityID, job.Record) {
	g, id := cleanGame(t)
	plainHut(g)
	g.addDirt(tileIdx(g, 11, 12), 300)
	return g, id, job.Record{Kind: job.Recreate, TX: 5, TY: 6}
}

func TestStartJobTidyGateAllocatesNothing(t *testing.T) {
	g, id, rec := tidyGateGame(t)
	tidy := senseOf(g).Tidy
	if n := testing.AllocsPerRun(200, func() { tidy.Before(id, rec) }); n != 0 {
		t.Fatalf("%v allocations on the way past", n)
	}
}

func BenchmarkStartJobTidyGate(b *testing.B) {
	g, id, rec := tidyGateGame(b)
	tidy := senseOf(g).Tidy
	b.ReportAllocs()
	for b.Loop() {
		tidy.Before(id, rec)
	}
}

// Three path searches, the worst a tidy's gate can spend, behind the cooldown
// and the room's dirt.
func BenchmarkTidyDetour(b *testing.B) {
	g, id := detourGame(b, 90)
	s := senseOf(g)
	bill := job.Record{Kind: job.Bill, DestX: 5, DestY: 6, TX: 85, TY: 8}
	b.ReportAllocs()
	for b.Loop() {
		delete(s.TidyAt, id)
		s.Tidy.Before(id, bill)
	}
}
