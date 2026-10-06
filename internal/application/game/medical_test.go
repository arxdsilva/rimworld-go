package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
)

func TestDoctorRescuesToBedAndTends(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(14, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 2, 2)
	g.hurt(patient, health.DownedInjury+10, 0)
	start := g.VitalOf(patient).Injury

	rescued := false
	for i := 0; i < 200 && !(rescued && g.tended(patient)); i++ {
		g.Step(20)
		if g.inBed(patient) {
			rescued = true
		}
		if v := g.VitalOf(patient); v.Status == health.Alive && !rescued {
			t.Fatalf("patient got up before rescue: %+v", v)
		}
	}
	if !rescued {
		j, _ := g.jobs.Get(doc)
		t.Fatalf("patient never reached the bed (doctor job %+v)", j)
	}
	if !g.tended(patient) {
		t.Fatal("doctor should tend the patient in bed")
	}
	if v := g.VitalOf(patient); v.Injury >= start {
		t.Fatalf("tending reduces injury: %d -> %d", start, v.Injury)
	}
	p := g.Snapshot()[0]
	if !p.InBed || !p.Lying || !p.Tended || p.CarriedBy != 0 {
		t.Fatalf("patient view: %+v", p)
	}
	for i := 0; i < 400 && g.VitalOf(patient).Status != health.Alive; i++ {
		g.Step(250)
		g.SetNeeds(patient, 1<<20, 1<<20)
		g.SetNeeds(doc, 1<<20, 1<<20)
	}
	if g.VitalOf(patient).Status != health.Alive {
		t.Fatalf("patient should recover in bed: %+v", g.VitalOf(patient))
	}
}

func TestRescueCarriesPatient(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(12, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(10, 6, "Doc")
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 2, 6)
	g.hurt(patient, health.DownedInjury+10, 0)
	for i := 0; i < 400; i++ {
		g.Step(1)
		if g.carriedBy(patient) == doc {
			pp, _ := g.positions.Get(patient)
			dp, _ := g.positions.Get(doc)
			if pp != dp {
				t.Fatal("a carried patient moves with the rescuer")
			}
			return
		}
	}
	t.Fatal("doctor never picked the patient up")
}

func TestButcherTableTurnsCorpsesIntoMeat(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	if _, err := g.SpawnNamedPawn(2, 5, "Cook"); err != nil {
		t.Fatal(err)
	}
	placeBuilt(t, g, construction.ButcherTable, construction.South, 6, 3)
	if w := g.World(); len(w.Buildings) != 1 || len(w.Buildings[0].Bills) != 2 {
		t.Fatalf("butcher table starts with butcher bills: %+v", w.Buildings)
	}
	if _, err := g.SpawnStack(item.CorpseDustbuck, 1, 3, 7); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100 && itemsOnGround(g, "meat") == 0; i++ {
		g.Step(30)
	}
	if got := itemsOnGround(g, "meat"); got != recipe.DefOf(recipe.ButcherDustbuck).OutQty {
		t.Fatalf("meat = %d", got)
	}
	if itemsOnGround(g, "dustbuck corpse") != 0 {
		t.Fatal("corpse consumed")
	}
}

func TestUnarmedColonistFleesArmedFightsBack(t *testing.T) {
	g := New(Config{Seed: 2, Width: 30, Height: 12})
	runner, _ := g.SpawnNamedPawn(10, 6, "Runner")
	wolf, _ := g.SpawnAnimal("ridgewolf", 13, 6)
	a, _ := g.animals.Get(wolf)
	a.Target, a.Mode = runner, animal.Attack
	g.animals.Set(wolf, a)
	g.Step(2)
	if j, _ := g.jobs.Get(runner); j.Kind != job.Move {
		t.Fatalf("unarmed colonist should run, job %+v", j)
	}

	g2 := New(Config{Seed: 2, Width: 30, Height: 12})
	fighter, _ := g2.SpawnNamedPawn(10, 6, "Fighter")
	g2.equip[fighter] = item.Club
	wolf2, _ := g2.SpawnAnimal("ridgewolf", 13, 6)
	a2, _ := g2.animals.Get(wolf2)
	a2.Target, a2.Mode = fighter, animal.Attack
	g2.animals.Set(wolf2, a2)
	g2.Step(2)
	if j, _ := g2.jobs.Get(fighter); j.Kind != job.Attack || j.Target != wolf2 {
		t.Fatalf("armed colonist should fight back, job %+v", j)
	}
}

func TestSleepingSpotIsInstantAndFree(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	sleeper, _ := g.SpawnNamedPawn(2, 2, "Sleeper")
	g.Apply(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.SleepingSpot), X: 5, Y: 4})
	g.Step(1)
	w := g.World()
	if len(w.Frames) != 0 || len(w.Buildings) != 1 || w.Buildings[0].Kind != "sleeping spot" {
		t.Fatalf("spot should exist at once with no frame: frames %+v buildings %+v", w.Frames, w.Buildings)
	}
	g.SetNeeds(sleeper, 1<<20, 1500) // tired, not yet collapsed
	for i := 0; i < 60 && !g.inBed(sleeper); i++ {
		g.Step(10)
	}
	if !g.inBed(sleeper) {
		t.Fatal("a tired colonist sleeps on the spot")
	}
	g.Apply(command.Command{Kind: command.DesignateDeconstruct, X: 5, Y: 4, X2: 5, Y2: 4})
	g.Step(1)
	if len(g.World().Buildings) != 0 {
		t.Fatal("deconstructing a spot erases it immediately")
	}
}

func TestTwoColonistsShareADoubleBed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 10})
	a, _ := g.SpawnNamedPawn(2, 7, "A")
	b, _ := g.SpawnNamedPawn(3, 7, "B")
	placeBuilt(t, g, construction.DoubleBed, construction.South, 5, 2)
	g.SetNeeds(a, 1<<20, 1500)
	g.SetNeeds(b, 1<<20, 1500)
	for i := 0; i < 100 && !(g.inBed(a) && g.inBed(b)); i++ {
		g.Step(10)
	}
	if !g.inBed(a) || !g.inBed(b) {
		t.Fatalf("both should sleep in the double bed: a=%v b=%v", g.inBed(a), g.inBed(b))
	}
	pa, _ := g.positions.Get(a)
	pb, _ := g.positions.Get(b)
	if pa == pb || pa.Y != 2 || pb.Y != 2 {
		t.Fatalf("side by side at the pillows: %+v %+v", pa, pb)
	}
	w := g.World()
	if w.Buildings[0].Claimant == 0 || w.Buildings[0].Claimant2 == 0 {
		t.Fatalf("both slots claimed: %+v", w.Buildings[0])
	}
}
