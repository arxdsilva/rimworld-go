package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// setMedical sends SetBedMedical for bed and applies it.
func setMedical(g *Game, bed entity.EntityID, on bool) {
	g.Enqueue(command.Command{Kind: command.SetBedMedical, Target: uint64(bed), Enabled: on})
	g.ApplyPending()
}

// assignOwner sends AssignBedOwner (slot 0) and applies it.
func assignOwner(g *Game, bed, owner entity.EntityID) {
	g.Enqueue(command.Command{Kind: command.AssignBedOwner, Target: uint64(bed), Pawn: uint64(owner)})
	g.ApplyPending()
}

func bedOf(g *Game, id entity.EntityID) construction.Building {
	b, _ := g.bldgs.Get(id)
	return b
}

// Medical is a per-bed setting: the bed beside it in the same room is
// left as it was, and only sleeping places take it.
func TestSetBedMedicalChangesOnlyThatBed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	hut(t, g, 2, 1, 12, 7)
	a := buildNow(t, g, construction.Bed, item.Wood, construction.South, 4, 2)
	b := buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
	bench := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 9, 4)
	if g.roomAt(4, 2) != g.roomAt(6, 2) {
		t.Fatal("setup: one room")
	}
	setMedical(g, a, true)
	setMedical(g, bench, true)
	if !bedOf(g, a).Medical || bedOf(g, b).Medical || bedOf(g, bench).Medical {
		t.Fatalf("medical: a %v, b %v, bench %v", bedOf(g, a).Medical, bedOf(g, b).Medical, bedOf(g, bench).Medical)
	}
	if !buildingView(g, a).Medical || buildingView(g, b).Medical {
		t.Fatal("the view shows the one medical bed")
	}
	setMedical(g, a, false)
	if bedOf(g, a).Medical || buildingView(g, a).Medical {
		t.Fatal("turned off again")
	}
}

// Turning a bed medical frees it of its owner (a sleeper in it gets up);
// a patient lying in it keeps it for its stay.
func TestMedicalBedReleasesOwners(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	sleepAnytime(g)
	ada, _ := g.SpawnNamedPawn(4, 6, "Ada")
	bo, _ := g.SpawnNamedPawn(12, 6, "Bo")
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 4, 2)
	sick := buildNow(t, g, construction.Bed, item.Wood, construction.South, 12, 2)
	assignOwner(g, bed, ada)
	assignOwner(g, sick, bo)
	g.SetNeeds(ada, needs.Full, needs.CriticalRest-500)
	if !stepUntil(g, 600, func() bool { return restingIn(g, ada, bed) }) {
		t.Fatal("setup: Ada sleeps in her bed")
	}
	g.Infect(bo, 40)
	setMedical(g, bed, true)
	setMedical(g, sick, true)
	if bedOf(g, bed).Claimant != 0 {
		t.Fatal("a healthy owner loses a bed made medical")
	}
	if j, _ := g.jobs.Get(ada); j.Target == bed && j.Active() {
		t.Fatal("and gets up out of it")
	}
	if bedOf(g, sick).Claimant != uint64(bo) || g.care.Rest.HeldBed(bo) != sick {
		t.Fatal("a patient keeps its slot as a stay")
	}
}

// Healthy colonists never sleep in, own, or are ordered into a medical
// bed; a hurt one may be.
func TestHealthyColonistsNeverUseMedicalBed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	sleepAnytime(g)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	med := buildNow(t, g, construction.Bed, item.Wood, construction.South, 8, 2)
	setMedical(g, med, true)
	g.SetNeeds(id, needs.Full, needs.CriticalRest-500)
	if !stepUntil(g, 600, func() bool {
		j, _ := g.jobs.Get(id)
		return j.Kind == job.Sleep && j.State == job.Do
	}) {
		t.Fatal("setup: tired, it sleeps")
	}
	if j, _ := g.jobs.Get(id); j.Target == med || bedOf(g, med).Claimant != 0 {
		t.Fatal("a healthy colonist sleeps on the floor rather than in a medical bed")
	}
	assignOwner(g, med, id)
	if bedOf(g, med).Claimant != 0 {
		t.Fatal("a medical bed has no owner")
	}
	b := bedOf(g, med)
	if _, why := g.forcedJob(id, command.OrderSleep, b.X, b.Y, construction.LayerObject); why != ReasonMedicalBed {
		t.Fatalf("ordered into it: %q, want %q", why, ReasonMedicalBed)
	}
	refused := false
	for _, o := range g.OrdersAt(id, b.X, b.Y) {
		refused = refused || o.Disabled && o.Reason == ReasonMedicalBed
	}
	if !refused {
		t.Fatal("the order menu shows why")
	}
	setInjury(g, id, 45, true)
	if _, why := g.forcedJob(id, command.OrderSleep, b.X, b.Y, construction.LayerObject); why != "" {
		t.Fatalf("a hurt colonist may be ordered into it: %q", why)
	}
}

// A healthy colonist woken in a medical bed (well again) gets up, and a
// plain bed is chosen over it.
func TestHealthySleeperLeavesMedicalBed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	sleepAnytime(g)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	med := buildNow(t, g, construction.Bed, item.Wood, construction.South, 4, 2)
	plain := buildNow(t, g, construction.Bed, item.Wood, construction.South, 12, 2)
	setMedical(g, med, true)
	g.SetNeeds(id, needs.Full, needs.CriticalRest-500)
	if !stepUntil(g, 600, func() bool { return restingIn(g, id, plain) }) {
		t.Fatal("tired, it walks past the medical bed to the plain one")
	}
	// put it in the medical bed by hand (an old save): it gets up
	b := bedOf(g, med).Claim(uint64(id))
	g.bldgs.Set(med, b)
	sx, sy := g.sleepCellFor(id, b)
	g.positions.Set(id, Position{X: sx, Y: sy})
	g.jobs.Set(id, job.Record{Kind: job.Sleep, State: job.Do, Target: med, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin})
	g.Step(2)
	if restingIn(g, id, med) {
		t.Fatal("healthy, it gets up out of a medical bed")
	}
}

// freeBedFor (rescue, surgery) ranks medical beds first, then the
// hospital kind, then distance.
func TestFreeBedForRanksMedicalFirst(t *testing.T) {
	g := New(Config{Seed: 2, Width: 24, Height: 12})
	patient, _ := g.SpawnNamedPawn(2, 5, "Patient")
	near := buildNow(t, g, construction.Bed, item.Wood, construction.South, 3, 5)
	hosp := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 10, 5)
	far := buildNow(t, g, construction.Bed, item.Wood, construction.South, 20, 5)
	from := Position{X: 2, Y: 5}
	if got := g.care.SafeBedFor(patient, from.X, from.Y, care.RescueBed); got != hosp {
		t.Fatalf("a new hospital bed is medical and wins: got %v want %v", got, hosp)
	}
	setMedical(g, hosp, false)
	setMedical(g, far, true)
	if got := g.care.SafeBedFor(patient, from.X, from.Y, care.RescueBed); got != far {
		t.Fatalf("a far medical bed beats a hospital bed that isn't: got %v want %v", got, far)
	}
	setMedical(g, far, false)
	if got := g.care.SafeBedFor(patient, from.X, from.Y, care.RescueBed); got != hosp {
		t.Fatalf("no medical bed: the hospital kind, then distance: got %v want %v (near %v)", got, hosp, near)
	}
}

func TestNewHospitalBedStartsMedical(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	h := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 4, 2)
	p := buildNow(t, g, construction.Bed, item.Wood, construction.South, 8, 2)
	if !bedOf(g, h).Medical || bedOf(g, p).Medical {
		t.Fatal("a hospital bed starts medical, a plain bed doesn't")
	}
}

// A rescued patient taken to a medical bed holds it for its stay and
// keeps its own bed.
func TestRescueToMedicalBedKeepsOwnBed(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	pat, _ := g.SpawnNamedPawn(5, 6, "Patient")
	g.SpawnNamedPawn(3, 9, "Doc")
	own := buildNow(t, g, construction.Bed, item.Wood, construction.South, 8, 2)
	hosp := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 15, 7)
	assignOwner(g, own, pat)
	setInjury(g, pat, 60, true)
	g.down(pat, health.CauseInjury)
	if !restFedUntil(g, 3000, func() bool { return g.bedUnder(pat) == hosp }) {
		t.Fatal("the doctor carries the patient to the medical bed")
	}
	if bedOf(g, own).Claimant != uint64(pat) || g.care.Rest.HeldBed(pat) != hosp {
		t.Fatalf("own bed kept (claimant %d), the medical bed a stay (%v)", bedOf(g, own).Claimant, g.care.Rest.HeldBed(pat))
	}
}

// A patient rests in a medical bed outside its allowed area; a healthy
// sleep there would not be exempt.
func TestMedicalBedIsAreaExempt(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	b := buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
	rec := job.Record{Kind: job.Sleep, Target: b}
	if g.areaExemptJob(id, rec) {
		t.Fatal("a plain bed is not exempt")
	}
	setMedical(g, b, true)
	if !g.areaExemptJob(id, rec) {
		t.Fatal("a medical bed is")
	}
}

func TestMedicalFlagSurvivesSave(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	b := buildNow(t, g, construction.Bed, item.Wood, construction.South, 4, 2)
	h := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 8, 2)
	setMedical(g, b, true)
	setMedical(g, h, false) // the player's choice outlives a save
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bedOf(l, b).Medical || bedOf(l, h).Medical {
		t.Fatalf("loaded: plain medical %v, hospital medical %v", bedOf(l, b).Medical, bedOf(l, h).Medical)
	}
}

// A v3 save (before medical beds) gets every hospital bed medical once:
// an owner elsewhere gives it up, a colonist lying in it keeps it for the
// stay, and plain beds are left alone.
func TestMigrateHospitalBedsFromV3(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 8})
	ada, _ := g.SpawnNamedPawn(2, 6, "Ada")
	bo, _ := g.SpawnNamedPawn(3, 6, "Bo")
	owned := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 4, 2)
	lying := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 8, 2)
	plain := buildNow(t, g, construction.Bed, item.Wood, construction.South, 12, 2)
	for _, id := range []entity.EntityID{owned, lying} {
		b := bedOf(g, id)
		b.Medical = false // as saved before PLAY-020
		g.bldgs.Set(id, b)
	}
	g.bldgs.Set(owned, bedOf(g, owned).Claim(uint64(ada)))
	b := bedOf(g, lying).Claim(uint64(bo))
	g.bldgs.Set(lying, b)
	sx, sy := g.sleepCellFor(bo, b)
	g.positions.Set(bo, Position{X: sx, Y: sy})
	g.jobs.Set(bo, job.Record{Kind: job.Sleep, State: job.Do, Target: lying, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin})

	l := loadAsVersion(t, g, 3)
	if !bedOf(l, owned).Medical || !bedOf(l, lying).Medical || bedOf(l, plain).Medical {
		t.Fatal("hospital beds become medical, plain beds don't")
	}
	if bedOf(l, owned).Claimant != 0 {
		t.Fatal("an owner sleeping elsewhere gives the bed up")
	}
	if bedOf(l, lying).Claimant != uint64(bo) || l.care.Rest.HeldBed(bo) != lying {
		t.Fatal("a colonist lying in it keeps it for the stay")
	}
	if c := loadAsVersion(t, g, SaveVersion); bedOf(c, owned).Medical {
		t.Fatal("a current save keeps the player's setting")
	}
}
