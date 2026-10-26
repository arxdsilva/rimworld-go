package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/plant"
)

func TestUntreatedInfectionKills(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 4, "Ada")
	g.setPriority(id, work.Doctor, 0) // no one to tend it
	g.Infect(id, infection.Onset)
	for i := 0; i < 30*clock.TicksPerHour/500 && g.VitalOf(id).Status != health.Dead; i++ {
		g.Step(500)
		g.SetNeeds(id, 1<<20, 1<<20)
	}
	if v := g.VitalOf(id); v.Status != health.Dead || v.Cause != health.CauseInfection {
		t.Fatalf("an untreated infection runs its course: %+v (severity %d)", v, g.InfectionOf(id))
	}
}

func TestWoundsMayFesterWhenLeftUntended(t *testing.T) {
	infected := 0
	for seed := uint64(1); seed <= 20; seed++ {
		g := New(Config{Seed: seed, Width: 12, Height: 8})
		id, _ := g.SpawnNamedPawn(5, 4, "Ada")
		g.hurt(id, 20, 0)
		if !g.Snapshot()[0].Wounded {
			t.Fatal("a fresh wound shows as untended")
		}
		g.tendedUntil[id] = 0
		g.wounds[id] = wound{At: 1}
		g.clock.Advance(infection.DelayHours*clock.TicksPerHour + 1)
		g.phaseInfectionAt()
		if g.InfectionOf(id) > 0 {
			infected++
		}
		if g.wounds[id].At != 0 {
			t.Fatal("the open-wound window closes either way")
		}
	}
	if infected == 0 || infected == 20 {
		t.Fatalf("untended wounds fester some of the time: %d/20", infected)
	}
}

// phaseInfectionAt runs the hourly infection pass now.
func (g *Game) phaseInfectionAt() {
	now := uint64(g.clock.Now())
	if r := now % clock.TicksPerHour; r != 0 {
		g.clock.Advance(int(clock.TicksPerHour - r))
	}
	g.phaseMedical()
}

func TestDoctorTendsInfectionWithMedicine(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(14, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	placeBuilt(t, g, construction.Bed, construction.South, 15, 2)
	if _, err := g.SpawnStack(item.HerbalMedicine, 3, 6, 9); err != nil {
		t.Fatal(err)
	}
	g.Infect(patient, 40)
	treated := false
	for i := 0; i < 300 && !treated; i++ {
		g.Step(20)
		g.SetNeeds(doc, 1<<20, 1<<20)
		treated = g.tended(patient)
	}
	if !treated {
		t.Fatalf("a sick colonist rests and a doctor tends them (patient job %v)", g.Snapshot()[0].Job)
	}
	if g.wounds[patient].Care != infection.Medicine {
		t.Fatalf("care %v, want medicine", g.wounds[patient].Care)
	}
	if n := g.countItem(item.HerbalMedicine); n != 2 {
		t.Fatalf("one dose used, %d left", n)
	}
	for i := 0; i < 20 && g.InfectionOf(patient) > 0; i++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(patient, 1<<20, 1<<20)
		g.SetNeeds(doc, 1<<20, 1<<20)
	}
	if g.InfectionOf(patient) != 0 || g.VitalOf(patient).Status == health.Dead {
		t.Fatalf("treated infection recedes (severity %d)", g.InfectionOf(patient))
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
	if !g.tended(id) || g.wounds[id].Care != infection.SelfTended && g.wounds[id].Infection == 0 && g.wounds[id].At != 0 {
		t.Fatal("with no doctor around, a colonist tends its own wound")
	}
}

func TestHealrootYieldsMedicine(t *testing.T) {
	d := plant.DefOf(plant.Healroot)
	if !d.Crop || d.Yields != item.HerbalMedicine || d.Yield <= 0 {
		t.Fatalf("healroot def %+v", d)
	}
}
