package game

import (
	"bytes"
	"os"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// MED-09: animals bleed, get tended, get infected, by their care level.

// cutAnimal gives animal id an open cut of dmg (no attacker).
func cutAnimal(g *Game, id entity.EntityID, dmg int) {
	g.hurtAs(id, dmg, 0, body.Cut, health.CauseInjury)
}

// animalBody is animal id's body (nil: none, or no such animal).
func animalBody(g *Game, id entity.EntityID) *body.Body {
	a, ok := g.animals.Get(id)
	if !ok {
		return nil
	}
	return a.Body
}

// toNextHour steps g to the first tick of the next hour (the medical
// phase runs there).
func toNextHour(g *Game) {
	now := uint64(g.clock.Now())
	g.Step(int(clock.TicksPerHour - now%clock.TicksPerHour))
}

// medicineLeft counts the medicine of kind k on the map.
func medicineLeft(g *Game, k item.Kind) int {
	n := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == k {
			n += st.Qty
		}
	}
	return n
}

// A cut opens a body on the animal: it bleeds by its size and joins the
// animal patients; the vet tends every wound, which stops the bleeding,
// mends Injury and uses a dose of medicine; the blood comes back.
func TestAnimalBleedsAndIsTended(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 40)
	g.positions.Set(dog, Position{X: 12, Y: 6})
	g.SpawnStack(item.HerbalMedicine, 3, 8, 3)
	if animalBody(g, dog) != nil {
		t.Fatal("an unhurt animal has no body")
	}
	g.setPriority(ada, work.Doctor, 0) // an hour's bleeding before the vet comes
	const cut = 12                     // MED-13: on seed 40 it lands on a part it does not take off
	cutAnimal(g, dog, cut)
	b := animalBody(g, dog)
	if b == nil || len(b.Wounds) != 1 || b.Wounds[0].Kind != body.Cut || b.Anatomy != body.Quadruped || b.Wounds[0].InfectAt == 0 {
		t.Fatalf("the cut is a wound on a part of the dog's body: %+v", b)
	}
	a, _ := g.animals.Get(dog)
	if a.Injury != cut || b.Damage() != cut {
		t.Fatalf("Injury %d and the wounds %d agree", a.Injury, b.Damage())
	}
	scale := animalHealthScale(a.Species)
	if got, want := animalBleedPerDay(b, a.Species), int32(cut)*body.BleedPerHP*1000/scale; got != want || got <= int32(cut)*body.BleedPerHP {
		t.Fatalf("a dog smaller than a man bleeds faster: %d/day, want %d", got, want)
	}
	if !slices.Contains(g.animalPatientIDs(), dog) {
		t.Fatal("the wounded dog is an animal patient")
	}
	toNextHour(g)
	if b = animalBody(g, dog); b == nil || b.BloodLoss == 0 {
		t.Fatalf("an hour of bleeding: %+v", b)
	}
	g.setPriority(ada, work.Doctor, 1)
	if !runCare(g, 300, func() bool { return g.tended(dog) }, ada) {
		t.Fatalf("the vet tended the cut: %+v", animalBody(g, dog))
	}
	a, _ = g.animals.Get(dog)
	if b = a.Body; b == nil || b.BleedPerDay() != 0 || len(b.Wounds) == 0 || !b.Wounds[0].Tended || b.BloodLoss == 0 {
		t.Fatalf("the tend stopped the bleeding of the wound left: %+v", b)
	}
	if b.Wounds[0].TendQ <= 0 || medicineLeft(g, item.HerbalMedicine) != 2 {
		t.Fatalf("tend quality %d, herbal left %d", b.Wounds[0].TendQ, medicineLeft(g, item.HerbalMedicine))
	}
	if av := animalViewOf(t, g, dog); av.BleedPerDay != 0 || av.NeedsTend || av.BleedDeathHours != -1 {
		t.Fatalf("view after the tend: %+v", av)
	}
	// it heals on: the body goes once nothing is left, and the index lets go
	runCare(g, 3000, func() bool { return animalBody(g, dog) == nil }, ada)
	toNextHour(g)
	if animalBody(g, dog) != nil || slices.Contains(g.animalPatientIDs(), dog) {
		t.Fatalf("a healed dog drops its body and the index: %+v %v", animalBody(g, dog), g.animalPatientIDs())
	}
}

// animalViewOf is animal id's view.
func animalViewOf(t *testing.T, g *Game, id entity.EntityID) AnimalView {
	t.Helper()
	for _, av := range g.animalViews() {
		if av.ID == id {
			return av
		}
	}
	t.Fatalf("no view of animal %d", id)
	return AnimalView{}
}

// A wild animal (no vet, no natural heal) with a deep cut bleeds out in
// the hours its bleed rate says; the view counts the hours down.
func TestAnimalBleedsOutUntended(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 41, Width: 20, Height: 12})
	dog, err := g.SpawnAnimal("labrador", 10, 6)
	if err != nil {
		t.Fatal(err)
	}
	maxHP := animal.DefOf(animal.Labrador).MaxHP
	cutAnimal(g, dog, maxHP*3/4)
	toNextHour(g)
	b := animalBody(g, dog)
	if b == nil || b.BloodLoss == 0 {
		t.Fatalf("an hour of bleeding: %+v", b)
	}
	av := animalViewOf(t, g, dog)
	if av.BleedDeathHours <= 0 || av.BleedPerDay == 0 || av.BloodLossPct != int(b.BloodLoss)/10 {
		t.Fatalf("view: %+v", av)
	}
	hours := av.BleedDeathHours
	for h := 0; h <= hours+2 && g.animals.Has(dog); h++ {
		toNextHour(g)
	}
	if g.animals.Has(dog) {
		t.Fatalf("still alive %d hours on: %+v", hours+2, animalBody(g, dog))
	}
	corpse := false
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind.IsCorpse() {
			corpse = true
		}
	}
	if !corpse || slices.Contains(g.animalPatientIDs(), dog) {
		t.Fatalf("it left a corpse (%v) and the index (%v)", corpse, g.animalPatientIDs())
	}
}

// Untended bites fester on some animals (the medical stream, MED-02
// odds), and an infection left to run kills.
func TestAnimalWoundCanInfect(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 42, Width: 40, Height: 20})
	var cows []entity.EntityID
	for i := 0; i < 20; i++ {
		id, err := g.SpawnAnimal("cow", 2+(i%10)*3, 4+(i/10)*8)
		if err != nil {
			t.Fatal(err)
		}
		g.hurtAs(id, 10, 0, body.Bite, health.CauseInjury)
		cows = append(cows, id)
	}
	for h := 0; h < infection.DelayMin+infection.DelaySpan+1; h++ {
		toNextHour(g)
	}
	infected := 0
	for _, id := range cows {
		if b := animalBody(g, id); b != nil {
			if _, ok := b.Cond(body.CondInfection); ok {
				infected++
			}
		}
	}
	if infected == 0 || infected == len(cows) {
		t.Fatalf("%d of %d bitten cows infected; want some (300‰ each)", infected, len(cows))
	}
	// a severe infection left untended kills within the hour
	id := cows[0]
	a, _ := g.animals.Get(id)
	if a.Body == nil {
		a.Body = newAnimalBody(a.Species)
	}
	a.Body.Conds = nil
	a.Body.AddCond(body.Condition{Kind: body.CondInfection, Part: a.Body.Root(), Sev: infection.Full - 5})
	g.animals.Set(id, a)
	g.markAnimalPatient(id)
	if av := animalViewOf(t, g, id); av.InfectionPct != 99 || !av.InfectionLosing {
		t.Fatalf("view of the infection: %+v", av)
	}
	toNextHour(g)
	if g.animals.Has(id) {
		t.Fatal("an infection at full severity kills the animal")
	}
}

// No care keeps the vet away from the animal (per animal, and by the
// Animals default); "Care, no medicine" tends it bare; the per-animal
// level beats the group default.
func TestAnimalCarePolicySkipsNoCare(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 43)
	g.positions.Set(dog, Position{X: 12, Y: 6})
	g.SpawnStack(item.HerbalMedicine, 3, 8, 3)
	if g.careOf(dog) != medcare.Default(medcare.Animals) {
		t.Fatalf("an animal starts at the Animals default: %v", g.careOf(dog))
	}
	setCare(g, dog, medcare.NoCare)
	cutAnimal(g, dog, 20)
	tendJob := func() bool { j, _ := g.jobs.Get(ada); return j.Kind == job.TendAnimal }
	if runCare(g, 200, tendJob, ada) {
		t.Fatal("No care: the vet stays away")
	}
	if b := animalBody(g, dog); b == nil || b.BleedPerDay() == 0 {
		t.Fatalf("No care: the wound stays untended: %+v", b)
	}
	// back to the group default, which the player set to No care
	g.care.per = nil
	setDefaultCare(g, medcare.Animals, medcare.NoCare)
	if runCare(g, 100, tendJob, ada) {
		t.Fatal("Animals default No care: the vet stays away")
	}
	// the animal's own level wins: tended, without medicine
	setCare(g, dog, medcare.NoMeds)
	if !runCare(g, 300, func() bool { b := animalBody(g, dog); return b != nil && len(b.Wounds) > 0 && b.BleedPerDay() == 0 }, ada) {
		t.Fatal("Care, no medicine: tended")
	}
	if medicineLeft(g, item.HerbalMedicine) != 3 {
		t.Fatalf("no medicine used: %d left", medicineLeft(g, item.HerbalMedicine))
	}
	if av := animalViewOf(t, g, dog); av.CareLevel != uint8(medcare.NoMeds) {
		t.Fatalf("view care level %d", av.CareLevel)
	}
}

func setDefaultCare(g *Game, grp medcare.Group, p medcare.Policy) {
	g.cmdSetDefaultCare(int(grp), uint8(p))
}

// A wounded, infected, half-bled animal loads back exactly, keeps being
// stepped through the rebuilt index, and the save hashes the same.
func TestAnimalHealthSurvivesSave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 44, Width: 20, Height: 12})
	cow, _ := g.SpawnAnimal("cow", 10, 6)
	g.hurtAs(cow, 20, 0, body.Bite, health.CauseInjury)
	a, _ := g.animals.Get(cow)
	a.Body.AddCond(body.Condition{Kind: body.CondInfection, Part: a.Body.Root(), Sev: 200, Immunity: 100})
	g.animals.Set(cow, a)
	toNextHour(g)
	toNextHour(g)
	before := *animalBody(g, cow)
	l := saveLoad(t, g)
	got := animalBody(l, cow)
	if got == nil || !reflect.DeepEqual(*got, before) {
		t.Fatalf("loaded body %+v, want %+v", got, before)
	}
	if l.StateHash() != g.StateHash() {
		t.Fatal("same state, same hash after a load")
	}
	toNextHour(g)
	toNextHour(l)
	if !reflect.DeepEqual(*animalBody(l, cow), *animalBody(g, cow)) || l.StateHash() != g.StateHash() {
		t.Fatalf("the loaded game steps the animal as the original: %+v vs %+v", animalBody(l, cow), animalBody(g, cow))
	}
	if !slices.Contains(l.animalPatientIDs(), cow) {
		t.Fatal("the index is rebuilt after a load")
	}
}

// Saves from before MED-09 load with no animal bodies; a legacy injury
// (no body) is still tended the old way, and a save with no wounded
// animal writes no body.
func TestOldSaveAnimalsLoad(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(ng1V2Fixture)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.animalIDs) == 0 {
		t.Fatal("fixture has animals")
	}
	for _, id := range g.animalIDs {
		if animalBody(g, id) != nil {
			t.Fatalf("animal %d loaded with a body", id)
		}
	}
	toNextHour(g)
	if len(g.animalPatientIDs()) != 0 {
		t.Fatalf("no animal patients from an older save: %v", g.animalPatientIDs())
	}
	// a legacy injury: no body is made, the vet heals it as before
	g2, ada, dog := bondYard(t, 45)
	g2.positions.Set(dog, Position{X: 12, Y: 6})
	a, _ := g2.animals.Get(dog)
	a.Injury = 20
	g2.animals.Set(dog, a)
	if !runCare(g2, 300, func() bool { return g2.tended(dog) }, ada) {
		t.Fatal("the legacy injured dog is tended")
	}
	if a, _ := g2.animals.Get(dog); a.Body != nil || a.Injury > 20-TendHeal/2 {
		t.Fatalf("legacy tend: body %+v, injury %d", a.Body, a.Injury)
	}
	var buf bytes.Buffer
	if err := g2.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("BloodStage")) {
		t.Fatal("no animal body in the save")
	}
}

// The vet tends a wounded, infected animal once per window: the tend
// covers the wounds and the infection for its 12 hours, so the doctor
// does not come back before then, and comes back once after.
func TestAnimalTendDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 46)
	g.positions.Set(dog, Position{X: 12, Y: 6})
	g.setPriority(ada, work.Doctor, 1)
	scheduleAll(g, schedule.Work) // on call around the clock
	cutAnimal(g, dog, 14)
	a, _ := g.animals.Get(dog)
	a.Body.AddCond(body.Condition{Kind: body.CondInfection, Part: a.Body.Root(), Sev: infection.Onset})
	g.animals.Set(dog, a)
	starts := 0
	last := job.None
	for i := 0; i < 20*clock.TicksPerHour/10; i++ {
		g.Step(10)
		if i%5 == 0 {
			g.SetNeeds(ada, 1<<20, 1<<20)
		}
		j, _ := g.jobs.Get(ada)
		if j.Kind == job.TendAnimal && last != job.TendAnimal {
			starts++
		}
		last = j.Kind
	}
	if !g.animals.Has(dog) {
		t.Fatal("fixture: the dog lives")
	}
	if starts != 2 {
		t.Fatalf("tends in 20 h: %d, want 2 (now, and the infection's next window)", starts)
	}
}
