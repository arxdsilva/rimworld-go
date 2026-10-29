package game

import (
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/prison"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Feeding patients (SIM-064): a downed pawn grows hungry and a doctor
// feeds it.

// feedGame: colonist Ann downed on (4, 5) with food hunger, meals on
// (16, 5); with doctor, colonist Doc on (12, 5) doing only Doctor work.
func feedGame(t *testing.T, hunger int, doctor bool) (g *Game, ann, doc entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 64, Width: 20, Height: 10})
	ann, _ = g.SpawnNamedPawn(4, 5, "Ann")
	if doctor {
		doc, _ = g.SpawnNamedPawn(12, 5, "Doc")
		onlyWork(g, doc, work.Doctor)
	}
	scheduleAll(g, schedule.Work)
	if _, err := g.SpawnStack(item.Meal, 5, 16, 5); err != nil {
		t.Fatal(err)
	}
	g.down(ann, health.CauseInjury) // no wound: nothing to tend, it never gets up by itself
	g.SetNeeds(ann, hunger, needs.Full)
	return g, ann, doc
}

// TestDownedColonistStarvesWithNobodyToFeedIt: alone, a downed colonist
// grows hungry (it used to stay frozen), starves and dies.
func TestDownedColonistStarvesWithNobodyToFeedIt(t *testing.T) {
	g, ann, _ := feedGame(t, 600, false)
	g.Step(needs.HungerPeriod * 300)
	if n, _ := g.need.Get(ann); n.Hunger > 300 {
		t.Fatalf("a downed colonist grows hungry: %d", n.Hunger)
	}
	if n, _ := g.need.Get(ann); n.Rest != needs.Full {
		t.Fatalf("rest stays as it is while downed: %d", n.Rest)
	}
	g.Step(needs.HungerPeriod*300 + 10)
	if v, _ := g.vital.Get(ann); v.Status != health.Downed || v.Cause != health.CauseNoFood {
		t.Fatalf("out of food: starving, %+v", v)
	}
	g.Step(health.StarvationTicks + 10)
	if v, _ := g.vital.Get(ann); v.Status != health.Dead {
		t.Fatalf("nobody fed it: dead of starvation, %+v", v)
	}
}

// TestDoctorFeedsDownedColonist: a doctor fetches a meal and feeds the
// downed colonist before it starves; "Fed by Doc" is logged.
func TestDoctorFeedsDownedColonist(t *testing.T) {
	g, ann, doc := feedGame(t, care.FeedHungry+60, true)
	sawFeed := false
	for i := 0; i < 3000; i++ {
		g.Step(1)
		fed(g, doc)
		if j, _ := g.jobs.Get(doc); j.Kind == job.FeedPatient {
			sawFeed = true
		}
	}
	n, _ := g.need.Get(ann)
	if !sawFeed || n.Hunger < needs.Full*8/10 {
		t.Fatalf("fed by the doctor: job seen %v, hunger %d", sawFeed, n.Hunger)
	}
	if !hasLog(g, ann, "Fed by Doc") || !hasLog(g, doc, "Fed Ann") {
		t.Fatalf("logs: %v / %v", logTexts(g, ann), logTexts(g, doc))
	}
	if v, _ := g.vital.Get(ann); v.Status != health.Downed {
		t.Fatalf("still downed, fed where it lies: %+v", v)
	}
}

// TestStarvingDownedColonistFedGetsUp: a colonist down from starvation
// that is fed gets up again.
func TestStarvingDownedColonistFedGetsUp(t *testing.T) {
	g, ann, doc := feedGame(t, 0, true)
	g.Step(1)
	if v, _ := g.vital.Get(ann); v.Cause != health.CauseNoFood {
		t.Fatalf("starving: %+v", v)
	}
	for i := 0; i < health.StarvationTicks-10; i++ {
		g.Step(1)
		fed(g, doc)
		if v, _ := g.vital.Get(ann); v.Status == health.Alive {
			return
		}
	}
	v, _ := g.vital.Get(ann)
	t.Fatalf("fed in time, it should stand up: %+v", v)
}

// TestFeedPatientOrder: "Feed Ann" from the menu feeds a downed colonist
// above Hungry; with no food it is disabled.
func TestFeedPatientOrder(t *testing.T) {
	g, ann, doc := feedGame(t, needs.Full/2, true)
	opt := feedOption(t, g, doc, ann)
	if opt.Disabled {
		t.Fatalf("Feed offered: %+v", opt)
	}
	g.Enqueue(opt.Cmd)
	g.Step(1)
	if j, _ := g.jobs.Get(doc); j.Kind != job.FeedPatient || !j.Forced {
		t.Fatalf("a forced feeding: %+v", j)
	}
	for i := 0; i < 2000 && !hasLog(g, ann, "Fed by"); i++ {
		g.Step(1)
		fed(g, doc)
	}
	if n, _ := g.need.Get(ann); n.Hunger < needs.Full*8/10 {
		t.Fatalf("ordered feeding fills it: %d", n.Hunger)
	}
	g2, ann2, doc2 := feedGame(t, needs.Full/2, true)
	for _, id := range g2.itemIDs {
		g2.deleteItem(id)
	}
	if opt := feedOption(t, g2, doc2, ann2); !opt.Disabled || opt.Reason != care.ReasonNoFood {
		t.Fatalf("no food: %+v", opt)
	}
}

// feedOption is doc's "Feed" entry on patient.
func feedOption(t *testing.T, g *Game, doc, patient entity.EntityID) OrderOption {
	t.Helper()
	p := g.positionOf(patient)
	for _, o := range g.OrdersAt(doc, p.X, p.Y) {
		if o.Cmd.Kind == command.Prioritize && o.Cmd.Order == command.OrderFeedPatient && o.Cmd.Target == uint64(patient) {
			return o
		}
	}
	t.Fatalf("no Feed entry for %d", patient)
	return OrderOption{}
}

// TestWardenFeedsDownedPrisoner: a downed prisoner grows hungry too and
// the warden's food run (the shared driver) feeds it.
func TestWardenFeedsDownedPrisoner(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	g.setPriority(bo, work.Warden, 1)
	if _, err := g.SpawnStack(item.Meal, 5, 10, 10); err != nil {
		t.Fatal(err)
	}
	g.prison.State(rid).NextVisit = 1 << 62 // food only
	if v, _ := g.vital.Get(rid); v.Status != health.Downed {
		t.Fatalf("the prisoner lies downed: %+v", v)
	}
	g.SetNeeds(rid, prison.FoodLow+120, needs.Full)
	for i := 0; i < 3000 && !hasLog(g, rid, "Fed by Bo"); i++ {
		g.Step(1)
		fed(g, ada, bo)
	}
	if n, _ := g.need.Get(rid); n.Hunger < prison.FoodFed || !hasLog(g, rid, "Fed by Bo") {
		t.Fatalf("the downed prisoner grew hungry and was fed: hunger %d, log %v", n.Hunger, logTexts(g, rid))
	}
}

// TestFeedPatientDoesNotFlap is the no-flap test (AGENTS.md, after
// sleep_test.go): fed at Hungry, the downed colonist is not fed again
// for as long as the meal lasts, and with no food the doctor tries once
// per care.FeedRetry instead of every tick.
func TestFeedPatientDoesNotFlap(t *testing.T) {
	g, ann, doc := feedGame(t, care.FeedHungry+60, true)
	runs, last := 0, job.None
	for i := 0; i < 20000; i++ {
		g.Step(1)
		fed(g, doc)
		j, _ := g.jobs.Get(doc)
		if j.Kind == job.FeedPatient && last != job.FeedPatient {
			runs++
		}
		last = j.Kind
	}
	if runs != 1 {
		t.Fatalf("%d feeding runs in 20000 ticks, want 1", runs)
	}
	if n, _ := g.need.Get(ann); n.Hunger <= care.FeedHungry {
		t.Fatalf("the meal holds: %d", n.Hunger)
	}
	g2, ann2, doc2 := feedGame(t, care.FeedHungry, true)
	for _, id := range g2.itemIDs {
		g2.deleteItem(id)
	}
	g2.Step(2)
	first := g2.care.Feed.RetryAt(ann2)
	if first == 0 {
		t.Fatal("no food: the patient waits for a retry")
	}
	g2.Step(care.FeedRetry - 10)
	fed(g2, doc2)
	if g2.care.Feed.RetryAt(ann2) != first {
		t.Fatal("no new look before the retry is up")
	}
	for i := 0; i < 300 && g2.care.Feed.RetryAt(ann2) == first; i++ {
		g2.Step(1) // an idle doctor looks once its wander ends
	}
	if r := g2.care.Feed.RetryAt(ann2); r <= first {
		t.Fatalf("after the retry the doctor looks again (retry %d, was %d)", r, first)
	}
}
