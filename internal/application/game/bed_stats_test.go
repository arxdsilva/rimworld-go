package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Beds and rest effectiveness (BED-1): rest/stats.go, rest/animals.go.

// sleepLength is how many ticks a colonist that goes to bed tired in a
// k (0: no bed, the ground) sleeps before it gets up.
func sleepLength(t *testing.T, k construction.Kind) int {
	t.Helper()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	sleepAnytime(g)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	var bed entity.EntityID
	if k != 0 {
		bed = buildNow(t, g, k, construction.DefaultStuff(k), construction.South, 2, 2)
	}
	tireOut(g, id)
	asleep := 0
	for range 4 * clock.TicksPerHour * 8 {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Sleep && j.State == job.Do {
			if j.Target != bed {
				t.Fatalf("%v: sleeps in %d, want %d", k, j.Target, bed)
			}
			asleep++
			continue
		}
		if asleep > 0 {
			return asleep
		}
	}
	t.Fatalf("%v: never got up (asleep %d)", k, asleep)
	return 0
}

// TestRestEffectivenessSetsSleepLength: rest comes back by the bed's rest
// effectiveness: a bed (1.0) beats a bedroll (0.95), which beats the
// ground and a sleeping spot (0.8), in proportion.
func TestRestEffectivenessSetsSleepLength(t *testing.T) {
	t.Parallel()
	bed, roll := sleepLength(t, construction.Bed), sleepLength(t, construction.Bedroll)
	spot, ground := sleepLength(t, construction.SleepingSpot), sleepLength(t, 0)
	if !(bed < roll && roll < spot) {
		t.Fatalf("sleep length: bed %d, bedroll %d, spot %d", bed, roll, spot)
	}
	near := func(got, want int) bool { return got >= want*98/100 && got <= want*102/100 }
	if !near(spot, bed*1000/800) || !near(ground, spot) || !near(roll, bed*1000/950) {
		t.Fatalf("not in proportion: bed %d, bedroll %d, spot %d, ground %d", bed, roll, spot, ground)
	}
}

// TestEndTableAndDresserRaiseComfort: an end table at the bed's head and
// a dresser in the room each add 0.05 comfort; the bed's rest
// effectiveness is unchanged.
func TestEndTableAndDresserRaiseComfort(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 4, 4)
	if v := g.rest.Stats.Of(bed); v.Comfort != 750 || v.RestEffect != 1000 || v.Beauty != 1000 {
		t.Fatalf("plain bed %+v", v)
	}
	buildNow(t, g, construction.EndTable, item.Wood, construction.South, 5, 4)
	if v := g.rest.Stats.Of(bed); v.Comfort != 800 || v.RestEffect != 1000 || v.EndTable == 0 {
		t.Fatalf("with an end table %+v", v)
	}
	buildNow(t, g, construction.Dresser, item.Wood, construction.South, 8, 6)
	if v := g.rest.Stats.Of(bed); v.Comfort != 850 || v.Dresser == 0 || v.Links() != rest.LinkEndTable|rest.LinkDresser {
		t.Fatalf("with a dresser too %+v", v)
	}
	if FacilityEndTable != rest.LinkEndTable || FacilityDresser != rest.LinkDresser {
		t.Fatal("read-model facility bits differ from rest's")
	}
	g.setForbidden(bed, true)
	for _, bv := range g.buildingViews() {
		if bv.ID == bed && (bv.Comfort != 850 || bv.RestEffect != 1000 || bv.Beauty != 1000 ||
			bv.Facilities != FacilityEndTable|FacilityDresser || !bv.Forbidden) {
			t.Fatalf("bed view %+v", bv)
		}
		if bv.Kind == "dresser" && (bv.Beauty != 5000 || bv.Comfort != 0 || bv.Facilities != 0) {
			t.Fatalf("dresser view %+v", bv)
		}
	}
}

// petOnBed tames a husky at (x, y) and builds an animal bed at (bx, by).
func petOnBed(t *testing.T, g *Game, x, y, bx, by int) (dog, bed entity.EntityID) {
	t.Helper()
	dog, err := g.SpawnAnimal("husky", x, y)
	if err != nil {
		t.Fatal(err)
	}
	setAnimal(g, dog, func(a *animal.State) { a.Tame = true })
	return dog, buildNow(t, g, construction.AnimalBed, item.Cloth, construction.South, bx, by)
}

// TestPetSleepsInAnimalBed: a tame pet beds down in its animal bed, rests
// faster there (1.0 against the ground's 0.8), and keeps the same bed
// night after night even with a nearer free spot (no flap).
func TestPetSleepsInAnimalBed(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	jumpTo(g, 1, 21)
	dog, bed := petOnBed(t, g, 10, 10, 16, 10)
	setAnimal(g, dog, func(a *animal.State) { a.Rest = animal.RestStart - 500 })
	var slept bool
	for range 2 * clock.TicksPerHour {
		g.Step(1)
		if a, pos := animalOf(g, dog), posOf(g, dog); sleeping(a) && pos.X == 16 && pos.Y == 10 {
			slept = true
			break
		}
	}
	if b, _ := g.bldgs.Get(bed); !slept || b.Claimant != uint64(dog) {
		t.Fatalf("pet not asleep in its bed: %+v at %+v, bed %+v", animalOf(g, dog), posOf(g, dog), b)
	}
	setAnimal(g, dog, func(a *animal.State) { a.Rest = 1000 })
	g.Step(clock.TicksPerHour)
	if got, want := animalOf(g, dog).Rest-1000, animal.SleepPerHour*1000/construction.GroundRestEffectiveness; got < want-10 {
		t.Fatalf("an hour in an animal bed restores %d, want %d", got, want)
	}
	placeBuilt(t, g, construction.AnimalSleepingSpot, construction.South, 11, 11) // nearer its day spot
	for night := 2; night <= 4; night++ {
		jumpTo(g, night, 10)
		setAnimal(g, dog, func(a *animal.State) { a.Mode, a.Rest, a.Path, a.PathI = animal.Wander, animal.Full, nil, 0 })
		g.positions.Set(dog, Position{X: 10, Y: 10})
		g.Step(animal.DecideEvery + 1)
		jumpTo(g, night, 21)
		setAnimal(g, dog, func(a *animal.State) { a.Rest = animal.RestStart - 500 })
		g.Step(animal.DecideEvery + 1)
		if a := animalOf(g, dog); a.Mode != animal.Rest || len(a.Path) == 0 || a.GoalX != 16 || a.GoalY != 10 {
			t.Fatalf("night %d: not off to its own bed: %+v", night, a)
		}
	}
}

// TestForbiddenBedUnused: a forbidden bed is left alone (the colonist
// sleeps on the ground) until allowed again; the mark survives a save.
func TestForbiddenBedUnused(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	sleepAnytime(g)
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
	g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(bed), Enabled: true})
	g.Step(1)
	tireOut(g, id)
	if j, _ := g.jobs.Get(id); j.Kind != job.Sleep || j.Target != 0 {
		t.Fatalf("slept in a forbidden bed: %+v", j)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := h.bldgs.Get(bed); !b.Forbidden {
		t.Fatal("forbidden mark lost in the save")
	}
	h.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(bed), Enabled: false})
	h.Step(1)
	tireOut(h, id)
	if j, _ := h.jobs.Get(id); j.Kind != job.Sleep || j.Target != bed {
		t.Fatalf("allowed bed unused: %+v", j)
	}
}

// TestFabricBedsDefaultToCloth: a bedroll designated with no material is
// sewn from cloth; leather is accepted, wood refused.
func TestFabricBedsDefaultToCloth(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	for i, c := range []struct {
		stuff item.Kind
		want  item.Kind
	}{{0, item.Cloth}, {item.Leather, item.Leather}, {item.Wood, item.Cloth}} {
		x := 2 + 3*i
		g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Bedroll), Stuff: uint8(c.stuff), X: x, Y: 2})
		g.Step(1)
		fr, ok := g.frames.Get(g.layerFrameAt(x, 2, construction.LayerObject))
		if !ok || fr.Stuff != c.want || fr.Need != construction.WoodCost(construction.Bedroll) {
			t.Fatalf("stuff %v: frame %+v", c.stuff, fr)
		}
	}
}
