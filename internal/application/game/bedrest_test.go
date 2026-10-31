package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// restingIn reports pawn id lying in bed bid on a sleep job.
func restingIn(g *Game, id, bid entity.EntityID) bool {
	j, _ := g.jobs.Get(id)
	return j.Kind == job.Sleep && j.State == job.Do && j.Target == bid
}

// setInjury gives colonist id injury hp with no body wounds behind it
// (all of it can still heal); open also opens an untended wound.
func setInjury(g *Game, id entity.EntityID, hp int, open bool) {
	v, _ := g.vital.Get(id)
	v.Injury = hp
	b := g.ensureBody(id, &v) // MED-02: the injury becomes wounds on the body
	if !open {
		b.TendAll(500, uint64(g.clock.Now())) // closed: already tended
	}
	g.bodyChanged(id, b, &v)
	g.woundOpened(id)
}

// restFed keeps every colonist fed and rested while stepping n ticks.
func restFed(g *Game, n int) {
	for i := 0; i < n; i++ {
		for _, id := range g.pawnIDs {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
		g.Step(1)
	}
}

// restFedUntil is stepUntil with every colonist kept fed and rested.
func restFedUntil(g *Game, n int, cond func() bool) bool {
	for i := 0; i < n; i++ {
		if cond() {
			return true
		}
		restFed(g, 1)
	}
	return cond()
}

// sickPatient is Pat, sick in bed at (6, 2) with a doctor walled in out of
// reach (so no one tends it and it does not tend itself), and food at
// (10, 4).
func sickPatient(t *testing.T) (g *Game, pat, bed entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 1, Width: 20, Height: 10})
	pat, _ = g.SpawnNamedPawn(4, 4, "Pat")
	g.SpawnNamedPawn(16, 7, "Doc")
	walledIn(t, g, 16, 7, 16, 7)
	bed = buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
	if _, err := g.SpawnStack(item.Food, 10, 10, 4); err != nil {
		t.Fatal(err)
	}
	g.Infect(pat, 40)
	g.SetNeeds(pat, needs.Full, needs.Full)
	if !stepUntil(g, 600, func() bool { return restingIn(g, pat, bed) }) {
		j, _ := g.jobs.Get(pat)
		t.Fatalf("setup: a sick colonist lies down in bed (job %v %v)", j.Kind, j.State)
	}
	return g, pat, bed
}

// A patient turning hungry in bed used to be put straight back to bed
// every tick: rest.Sleep.done woke it to eat, then patientRest (before the eat
// check) laid it down again, so it never ate.
func TestHungryPatientEatsThenReturnsToBed(t *testing.T) {
	g, pat, bed := sickPatient(t)
	g.SetNeeds(pat, needs.RavenousAt+1, needs.Full) // turns ravenous asleep (rest.Sleep.Do; FOOD-1: sleepers wake ravenous)
	ate := stepUntil(g, 1500, func() bool {
		n, _ := g.need.Get(pat)
		return n.Hunger > needs.RavenousAt+1000
	})
	if !ate {
		j, _ := g.jobs.Get(pat)
		t.Fatalf("a hungry patient gets up to eat (job %v %v)", j.Kind, j.State)
	}
	if !stepUntil(g, 1500, func() bool { return restingIn(g, pat, bed) }) {
		j, _ := g.jobs.Get(pat)
		t.Fatalf("fed, the patient goes back to bed (job %v %v)", j.Kind, j.State)
	}
}

// A colonist in pain goes to bed with a doctor about and stays there
// until it is well (healing injury down to care.RestWellInjury, at least
// care.RestMin), then gets up.
func TestInjuredColonistRestsUntilHealed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	pat, _ := g.SpawnNamedPawn(4, 4, "Pat")
	g.SpawnNamedPawn(12, 6, "Doc")
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
	setInjury(g, pat, 45, true)
	if !restFedUntil(g, 600, func() bool { return restingIn(g, pat, bed) }) {
		t.Fatal("a colonist in pain lies down in bed")
	}
	start, _ := g.care.Rest.Since(pat)
	for i := 0; i < 3*clock.TicksPerDay && g.care.Rest.On(pat); i++ { // MED-02: wounds heal about 20 HP a day in bed, tended
		v, _ := g.vital.Get(pat)
		if healingInjury(g.bodies[pat], v) > care.RestWellInjury && !restingIn(g, pat, bed) {
			j, _ := g.jobs.Get(pat)
			t.Fatalf("tick %d: still hurt (%d) but out of bed (job %v %v)", i, healingInjury(g.bodies[pat], v), j.Kind, j.State)
		}
		restFed(g, 1)
	}
	if g.care.Rest.On(pat) {
		t.Fatal("the rest never ended")
	}
	v, _ := g.vital.Get(pat)
	if h := healingInjury(g.bodies[pat], v); h > care.RestWellInjury || g.needsTending(pat) {
		t.Fatalf("got up unwell: healing injury %d, needs tending %v", h, g.needsTending(pat))
	}
	if now := uint64(g.clock.Now()); now-start < care.RestMin {
		t.Fatalf("rested %d ticks, want at least %d", now-start, care.RestMin)
	}
	if !restFedUntil(g, 300, func() bool { return !restingIn(g, pat, bed) }) {
		t.Fatal("well again in Work hours, it gets up")
	}
}

// With no bed free a patient carries on and only looks again after
// care.RestRetry.
func TestNoBedPutsRestRetryInEffect(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	setInjury(g, id, 45, true)
	restFed(g, 2)
	now := uint64(g.clock.Now())
	if g.care.Rest.On(id) || g.care.Rest.RetryAt(id) <= now || g.care.Rest.RetryAt(id) > now+care.RestRetry {
		t.Fatalf("no bed: resting %v, retry at %d (now %d)", g.care.Rest.On(id), g.care.Rest.RetryAt(id), now)
	}
	if g.care.Rest.PatientRest(id) {
		t.Fatal("no new rest before the retry")
	}
	if c := kindChanges(g, id, 300); c > 3 {
		t.Fatalf("job kind changed %d times in 300 ticks", c)
	}
	if j, _ := g.jobs.Get(id); j.Kind == job.Sleep {
		t.Fatal("it keeps going instead of lying on the ground")
	}
}

// With no bed, a patient another colonist will tend lies down where it is
// for the doctor (as before beds were required), then carries on.
func TestNoBedPatientWaitsOnFloorForDoctor(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	pat, _ := g.SpawnNamedPawn(2, 2, "Pat")
	g.SpawnNamedPawn(10, 5, "Doc")
	setInjury(g, pat, 45, true)
	onFloor := func() bool {
		j, _ := g.jobs.Get(pat)
		return j.Kind == job.Sleep && j.State == job.Do && j.Target == 0
	}
	if !restFedUntil(g, 300, onFloor) {
		t.Fatal("it lies down on the floor for the doctor")
	}
	if !restFedUntil(g, 3000, func() bool { return g.tended(pat) }) {
		t.Fatal("the doctor tends it where it lies")
	}
	if !restFedUntil(g, 300, func() bool { return !onFloor() }) || g.care.Rest.On(pat) {
		t.Fatal("tended, with no bed to rest in, it carries on")
	}
	if c := kindChanges(g, pat, 300); c > 3 {
		t.Fatalf("job kind changed %d times in 300 ticks", c)
	}
}

// "Rest when hurt" off: the patient gets up and works; back on, it rests.
func TestRestWhenHurtToggle(t *testing.T) {
	g, pat, bed := sickPatient(t)
	g.Enqueue(command.Command{Kind: command.SetPawnBedRest, Target: uint64(pat), Enabled: false})
	if !restFedUntil(g, 300, func() bool { return !restingIn(g, pat, bed) }) {
		t.Fatal("turned off, the patient gets up")
	}
	if g.care.Rest.On(pat) || pawnView(g, pat).RestWhenHurt {
		t.Fatal("turned off: no rest, and the view shows it")
	}
	for i := 0; i < 600; i++ {
		restFed(g, 1)
		if j, _ := g.jobs.Get(pat); j.Kind == job.Sleep {
			t.Fatalf("tick %d: back in bed with rest off", i)
		}
	}
	g.Enqueue(command.Command{Kind: command.SetPawnBedRest, Target: uint64(pat), Enabled: true})
	if !restFedUntil(g, 600, func() bool { return restingIn(g, pat, bed) }) {
		t.Fatal("turned on again, the patient rests")
	}
	if !pawnView(g, pat).RestWhenHurt {
		t.Fatal("view: rest when hurt on")
	}
}

func TestDraftingOverridesBedRest(t *testing.T) {
	g, pat, bed := sickPatient(t)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(pat), Enabled: true})
	for i := 0; i < 300; i++ {
		restFed(g, 1)
		if j, _ := g.jobs.Get(pat); j.Kind == job.Sleep || !g.drafted(pat) {
			t.Fatalf("tick %d: drafted, it stays up (job %v, drafted %v)", i, j.Kind, g.drafted(pat))
		}
	}
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(pat), Enabled: false})
	if !restFedUntil(g, 600, func() bool { return restingIn(g, pat, bed) }) {
		t.Fatal("undrafted, the patient lies down again")
	}
}

func TestBedRestResumesAfterForcedOrder(t *testing.T) {
	g, pat, bed := sickPatient(t)
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(pat), X: 12, Y: 6})
	arrived := restFedUntil(g, 600, func() bool {
		p, _ := g.positions.Get(pat)
		return p.X == 12 && p.Y == 6
	})
	if !arrived {
		j, _ := g.jobs.Get(pat)
		t.Fatalf("the order comes first (job %v %v)", j.Kind, j.State)
	}
	if !restFedUntil(g, 600, func() bool { return restingIn(g, pat, bed) }) {
		t.Fatal("after the order, rest resumes")
	}
}

// A patient takes a free medical bed over its own, holds it only for the
// stay, and keeps its own bed throughout.
func TestPatientPrefersMedicalBedThenGivesItBack(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 10})
	pat, _ := g.SpawnNamedPawn(4, 4, "Pat")
	g.SpawnNamedPawn(10, 7, "Doc")
	own := buildNow(t, g, construction.Bed, item.Wood, construction.South, 5, 2)
	med := buildNow(t, g, construction.Bed, item.Wood, construction.South, 18, 2)
	g.Enqueue(command.Command{Kind: command.SetBedMedical, Target: uint64(med), Enabled: true})
	g.Enqueue(command.Command{Kind: command.AssignBedOwner, Target: uint64(own), Pawn: uint64(pat)})
	restFed(g, 1)
	setInjury(g, pat, 45, true)
	if !restFedUntil(g, 1500, func() bool { return restingIn(g, pat, med) }) {
		j, _ := g.jobs.Get(pat)
		t.Fatalf("the patient rests in the medical bed (job %v %v target %v)", j.Kind, j.State, j.Target)
	}
	if b, _ := g.bldgs.Get(own); b.Claimant != uint64(pat) || g.care.Rest.HeldBed(pat) != med {
		t.Fatalf("its own bed stays its own (claimant %d), the medical one is a stay (%v)", b.Claimant, g.care.Rest.HeldBed(pat))
	}
	if !restFedUntil(g, 3*clock.TicksPerDay, func() bool { return !g.care.Rest.On(pat) && !restingIn(g, pat, med) }) { // MED-02: wiki healing rates
		t.Fatal("well again, the patient gets up")
	}
	restFed(g, 2)
	if b, _ := g.bldgs.Get(med); b.Claimant != 0 {
		t.Fatalf("the medical bed is given back (claimant %d)", b.Claimant)
	}
	if b, _ := g.bldgs.Get(own); b.Claimant != uint64(pat) {
		t.Fatal("its own bed is still its own")
	}
	if g.care.Rest.HeldBed(pat) != 0 {
		t.Fatal("no stay left on record")
	}
}

// With no one else to doctor it, a patient tends its own wound in bed.
func TestLonePatientTendsItselfInBed(t *testing.T) {
	g := New(Config{Seed: 3, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 4, "Ada")
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 8, 2)
	setInjury(g, id, 20, true)
	if !restFedUntil(g, 600, func() bool { return restingIn(g, id, bed) }) {
		t.Fatal("an untended wound sends it to bed")
	}
	if !restFedUntil(g, 2000, func() bool { return g.tended(id) }) {
		t.Fatal("alone, it tends itself in bed")
	}
	if !g.care.Rest.On(id) {
		t.Fatal("tended during the rest")
	}
	if !restFedUntil(g, 600, func() bool { return restingIn(g, id, bed) }) {
		t.Fatal("tended, it lies back down")
	}
}

// Scars and lost parts can't heal: they never keep a colonist in bed.
func TestPermanentDamageDoesNotRest(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Peg")
	placeBuilt(t, g, construction.Bed, construction.South, 12, 2)
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	b.Missing |= 1<<body.LegL | 1<<body.FootL
	g.bodyChanged(id, b, &v)
	if v := g.VitalOf(id); v.Injury < painInjury || healingInjury(g.bodies[id], v) != 0 {
		t.Fatalf("setup: injury %d (want >= %d), healing %d", v.Injury, painInjury, healingInjury(g.bodies[id], g.VitalOf(id)))
	}
	if g.care.Rest.NeedsBedRest(id) {
		t.Fatal("a lost leg alone is no reason to rest")
	}
	restFed(g, 300)
	if g.care.Rest.On(id) {
		t.Fatal("an amputee is not kept in bed")
	}
}

// No-flap: around the start and stop lines a patient changes job kind a
// handful of times at most.
func TestBedRestDoesNotFlap(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(g *Game, id entity.EntityID)
	}{
		{"pain line, tended", func(g *Game, id entity.EntityID) { setInjury(g, id, painInjury, false) }},
		{"just over the well line", func(g *Game, id entity.EntityID) {
			setInjury(g, id, painInjury, false)
			restFed(g, 300)
			setInjury(g, id, care.RestWellInjury+1, false)
		}},
		{"at the well line after care.RestMin", func(g *Game, id entity.EntityID) {
			setInjury(g, id, painInjury, false)
			restFed(g, 300)
			setInjury(g, id, care.RestWellInjury, false)
			g.care.Rest.Start(id, uint64(g.clock.Now())-care.RestMin)
		}},
		{"turning hungry", func(g *Game, id entity.EntityID) {
			setInjury(g, id, painInjury, false)
			restFed(g, 300)
			g.SetNeeds(id, needs.CriticalHunger+1, needs.Full)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := New(Config{Seed: 1, Width: 20, Height: 10})
			id, _ := g.SpawnNamedPawn(4, 4, "Pat")
			g.SpawnNamedPawn(12, 6, "Doc")
			buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
			if _, err := g.SpawnStack(item.Food, 10, 10, 4); err != nil {
				t.Fatal(err)
			}
			c.set(g, id)
			if n := kindChanges(g, id, 300); n > 3 {
				t.Fatalf("job kind changed %d times in 300 ticks", n)
			}
		})
	}
}

func TestBedRestStateSurvivesSave(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.care.Rest.Import(&care.RestSave{Off: []entity.EntityID{id}, Since: []care.Entry[uint64]{{ID: id, V: 11}},
		Retry: []care.Entry[uint64]{{ID: id, V: 22}}, Bed: []care.Entry[entity.EntityID]{{ID: id, V: 33}}})
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if since, _ := h.care.Rest.Since(id); !h.care.Rest.Off(id) || since != 11 || h.care.Rest.RetryAt(id) != 22 || h.care.Rest.HeldBed(id) != 33 {
		t.Fatalf("lost: %+v", h.care.Rest.Export())
	}
	if New(Config{Seed: 1, Width: 10, Height: 8}).care.Rest.Export() != nil {
		t.Fatal("no rest: nothing saved (old hashes hold)")
	}
}

// A resting patient saved and loaded goes on exactly as the original.
func TestRestingPatientRoundTrip(t *testing.T) {
	g, pat, bed := sickPatient(t)
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	g.Step(500)
	h.Step(500)
	if g.StateHash() != h.StateHash() || !restingIn(h, pat, bed) {
		t.Fatal("a loaded patient rests on as the original does")
	}
}

func TestBedRestView(t *testing.T) {
	g, pat, _ := sickPatient(t)
	g.Step(1)
	pv := pawnView(g, pat)
	if !pv.BedRest || pv.Job != care.RestingLabel || !pv.RestWhenHurt {
		t.Fatalf("resting: BedRest %v, job %q, rest when hurt %v", pv.BedRest, pv.Job, pv.RestWhenHurt)
	}
	for _, p := range g.Snapshot() {
		if p.ID != pat && (p.BedRest || !p.RestWhenHurt) {
			t.Fatalf("%s: not resting, setting on by default", p.Name)
		}
	}
}

// otherDoctor skips doctors on bed rest: two resting doctors don't wait on
// each other forever.
func TestRestingDoctorIsNoOtherDoctor(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	a, _ := g.SpawnNamedPawn(2, 2, "A")
	b, _ := g.SpawnNamedPawn(4, 2, "B")
	if g.prioOf(b)[work.Doctor] == 0 || !g.care.Doctor.OtherDoctor(a) {
		t.Fatal("setup: B doctors")
	}
	g.care.Rest.Start(b, 1)
	if g.care.Doctor.OtherDoctor(a) {
		t.Fatal("a resting doctor tends no one")
	}
}
