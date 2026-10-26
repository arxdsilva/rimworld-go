package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/plant"
)

// TestUntreatedInfectionKills: with no one to tend it, an infection
// climbs 84 % a day and kills in about 28 hours (MED-02).
func TestUntreatedInfectionKills(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 4, "Ada")
	g.setPriority(id, work.Doctor, 0) // no one to tend it
	g.Infect(id, 1)
	hours := 0
	for ; hours < 40 && g.VitalOf(id).Status != health.Dead; hours++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(id, 1<<20, 1<<20)
	}
	if v := g.VitalOf(id); v.Status != health.Dead || v.Cause != health.CauseInfection {
		t.Fatalf("an untreated infection runs its course: %+v (severity %d)", v, g.InfectionOf(id))
	}
	if hours < 26 || hours > 31 {
		t.Fatalf("died after %d h, want about 28", hours)
	}
}

// TestWoundsMayFesterWhenLeftUntended: an untended bite gets its roll
// 6–18 hours after it opened and festers some of the time (30 %
// outdoors); the roll is spent either way.
func TestWoundsMayFesterWhenLeftUntended(t *testing.T) {
	infected := 0
	for seed := uint64(1); seed <= 40; seed++ {
		g := New(Config{Seed: seed, Width: 12, Height: 8})
		id, _ := g.SpawnNamedPawn(5, 4, "Ada")
		b := addWound(t, g, id, body.ArmL, 10, body.Bite) // heals in 30 h: still open at the roll
		if !g.Snapshot()[0].Wounded {
			t.Fatal("a fresh wound shows as untended")
		}
		at := b.Wounds[0].InfectAt
		if h := (at - b.Wounds[0].At) / clock.TicksPerHour; h < infection.DelayMin || h > infection.DelayMin+infection.DelaySpan {
			t.Fatalf("roll due in %d h", h)
		}
		for uint64(g.clock.Now()) < at {
			medHour(g)
		}
		if g.InfectionOf(id) > 0 {
			infected++
		}
		if g.bodies[id].Wounds[0].InfectAt != 0 {
			t.Fatal("the roll is spent either way")
		}
	}
	if infected < 4 || infected > 22 {
		t.Fatalf("untended bites fester about 30%% of the time: %d/40", infected)
	}
}

// TestBruisesNeverFester: a bruise gets no roll at all.
func TestBruisesNeverFester(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 4, "Ada")
	if b := addWound(t, g, id, body.Torso, 5, body.Bruise); b.Wounds[0].InfectAt != 0 {
		t.Fatal("a bruise is never rolled")
	}
}

// TestCleanRoomHalvesTheChance: a roofed, floored, clean room halves the
// infection chance; outdoors it is whole.
func TestCleanRoomHalvesTheChance(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(9, 5, "Ada")
	if f := g.roomInfectionFactor(id); f != infection.RoomOther {
		t.Fatalf("outdoors: %d", f)
	}
	hut(t, g, 6, 2, 12, 8)
	for y := 3; y < 8; y++ {
		for x := 7; x < 12; x++ {
			if !g.BuildNow("floor", "wood", "south", x, y) {
				t.Fatalf("floor at %d,%d", x, y)
			}
		}
	}
	g.warmIndexes(true)
	if f := g.roomInfectionFactor(id); f != infection.RoomClean {
		r := g.roomAt(9, 5)
		t.Fatalf("a clean floored hut: %d (room %d roofed %d floored %d/%d)", f, r, g.roomRoofed(r), g.rooms[r].contents.Floored, g.rooms[r].contents.Cells)
	}
}

// phaseInfectionAt runs the hourly medical pass now.
func (g *Game) phaseInfectionAt() {
	now := uint64(g.clock.Now())
	if r := now % clock.TicksPerHour; r != 0 {
		g.clock.Advance(int(clock.TicksPerHour - r))
	}
	g.phaseMedical()
}

// TestDoctorTendsInfectionWithMedicine: a sick colonist rests, a doctor
// tends the infection with industrial medicine at a rolled quality, tends
// it again each 12-hour window, and the infection is beaten.
func TestDoctorTendsInfectionWithMedicine(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(14, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	scheduleAll(g, schedule.Work) // the doctor is on call around the clock
	placeBuilt(t, g, construction.Bed, construction.South, 15, 2)
	if _, err := g.SpawnStack(item.IndustrialMedicine, 6, 6, 9); err != nil {
		t.Fatal(err)
	}
	g.Infect(patient, 5)
	treated := false
	for i := 0; i < 600 && !treated; i++ {
		g.Step(20)
		g.SetNeeds(doc, 1<<20, 1<<20)
		treated = g.tended(patient)
	}
	if !treated {
		t.Fatalf("a sick colonist rests and a doctor tends them (patient job %v)", g.Snapshot()[0].Job)
	}
	c, _ := g.worstInfection(patient)
	if c.TendQ <= 0 || c.TendUntil <= uint64(g.clock.Now()) {
		t.Fatalf("the infection carries the tend: %+v", c)
	}
	if n := g.countItem(item.IndustrialMedicine); n != 5 {
		t.Fatalf("one dose used, %d left", n)
	}
	for i := 0; i < 4*24 && g.InfectionOf(patient) > 0; i++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(patient, 1<<20, 1<<20)
		g.SetNeeds(doc, 1<<20, 1<<20)
	}
	if g.InfectionOf(patient) != 0 || g.VitalOf(patient).Status == health.Dead {
		t.Fatalf("a well-tended infection is beaten (severity %d)", g.InfectionOf(patient))
	}
	if used := 6 - g.countItem(item.IndustrialMedicine); used < 2 || used > 6 {
		t.Fatalf("re-tended each window: %d doses", used)
	}
}

// TestOneTendPerWindow (no-flap): an infected, tended patient in bed with
// a doctor and plenty of medicine is tended once per 12-hour window, not
// again and again.
func TestOneTendPerWindow(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(14, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	scheduleAll(g, schedule.Work)
	placeBuilt(t, g, construction.Bed, construction.South, 15, 2)
	if _, err := g.SpawnStack(item.HerbalMedicine, 20, 6, 9); err != nil {
		t.Fatal(err)
	}
	g.Infect(patient, 5)
	tends := 0
	last, _ := g.jobs.Get(doc)
	for i := 0; i < 30*clock.TicksPerHour/10; i++ {
		g.Step(10)
		g.SetNeeds(doc, 1<<20, 1<<20)
		g.SetNeeds(patient, 1<<20, 1<<20)
		j, _ := g.jobs.Get(doc)
		if j.Kind == job.Tend && last.Kind != job.Tend {
			tends++
		}
		last = j
	}
	if tends < 2 || tends > 3 {
		t.Fatalf("%d tends in 30 hours, want one per 12-hour window (2-3)", tends)
	}
}

// TestPrisonerWoundsCanInfect: a prisoner's wounds get infection rolls
// and an infection races on its body like a colonist's (MED-01 left
// raiders and prisoners out).
func TestPrisonerWoundsCanInfect(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	b := addWound(t, g, rid, body.ArmL, 6, body.Bite)
	w := len(b.Wounds) - 1
	if b.Wounds[w].InfectAt == 0 {
		t.Fatal("a prisoner's bite is rolled")
	}
	b.Wounds[w].InfectAt = uint64(g.clock.Now()) + 1
	medHour(g)
	if g.bodies[rid].Wounds[w].InfectAt != 0 {
		t.Fatal("the prisoner's roll is spent in the hourly pass")
	}
	g.Infect(rid, 50)
	medHour(g)
	if c, ok := g.worstInfection(rid); !ok || c.Immunity <= 0 || c.Sev == 500 {
		t.Fatalf("a prisoner's infection races: %+v", c)
	}
	if !g.indexedPatient(rid) {
		t.Fatal("the infected prisoner is a patient")
	}
}

func (g *Game) countItem(k item.Kind) int {
	n := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == k {
			n += st.Qty
		}
	}
	return n
}

func TestLoneColonistTendsItself(t *testing.T) {
	g := New(Config{Seed: 3, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 4, "Ada")
	g.hurt(id, 25, 0)
	for i := 0; i < 100 && !g.tended(id); i++ {
		g.Step(20)
		g.SetNeeds(id, 1<<20, 1<<20)
	}
	if !g.tended(id) || g.needsTending(id) {
		t.Fatal("with no doctor around, a colonist tends its own wound")
	}
	for _, w := range g.bodies[id].Wounds {
		if !w.Permanent && (!w.Tended || w.TendAt == 0) {
			t.Fatalf("every wound tended: %+v", w)
		}
	}
}

func TestHealrootYieldsMedicine(t *testing.T) {
	d := plant.DefOf(plant.Healroot)
	if !d.Crop || d.Yields != item.HerbalMedicine || d.Yield <= 0 {
		t.Fatalf("healroot def %+v", d)
	}
}
