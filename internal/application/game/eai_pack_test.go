package game

import (
	"testing"

	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
)

// EAI-005..007: packs sized by points from the map's biome, the grieving
// pet, and a botched taming.

// eaiPack is the enraged (or, starving, Attack-moded) animals packed in
// by send on an open map of biome b: their one species and number.
func eaiPack(t *testing.T, b biome.ID, send func(g *Game)) (animal.Species, int) {
	t.Helper()
	g := New(Config{Seed: 9, Width: 60, Height: 40})
	g.biome = b
	before := map[entity.EntityID]bool{}
	for _, id := range g.animalIDs {
		before[id] = true
	}
	send(g)
	var s animal.Species
	n := 0
	for _, id := range g.animalIDs {
		if before[id] {
			continue
		}
		a := animalOf(g, id)
		if n > 0 && a.Species != s {
			t.Fatalf("a pack is one species: %s and %s", s, a.Species)
		}
		s = a.Species
		n++
	}
	return s, n
}

// TestManhunterPackByBiome: a manhunter pack and a starving pack are of
// a species the biome has, as many as the points buy.
func TestManhunterPackByBiome(t *testing.T) {
	t.Parallel()
	for _, b := range []biome.ID{biome.TemperateForest, biome.Tundra, biome.AridShrubland, biome.TropicalRainforest} {
		for _, pts := range []int{150, 600, 1500} {
			s, n := eaiPack(t, b, func(g *Game) { rageOf(g).ManhunterPack(pts) })
			if n == 0 {
				t.Fatalf("%v: no manhunter pack for %d points", b, pts)
			}
			if !rage.ManhunterSpecies(s) || animal.Weight(s, b) <= 0 {
				t.Fatalf("%v: a manhunter pack of %s, not a grazer or territorial animal of the biome", b, s)
			}
			if want := animal.PackSize(pts, s); n != want {
				t.Fatalf("%v, %d points: %d %s, want %d", b, pts, n, s, want)
			}
			s, n = eaiPack(t, b, func(g *Game) { g.raid.Threats.WolfPack(pts) })
			if n == 0 {
				continue // a biome without predators sends none
			}
			if !animal.DefOf(s).Predator || animal.Weight(s, b) <= 0 {
				t.Fatalf("%v: a starving pack of %s, not a predator of the biome", b, s)
			}
			if want := max(animal.PackSize(pts, s), 2); n != want {
				t.Fatalf("%v, %d points: %d starving %s, want %d", b, pts, n, s, want)
			}
		}
	}
}

// eaiGriefYard: Ada (armed with a club when armed) at (20, 10), Bo at
// (26, 10), and Rook, Ada's tame dog, gone manhunter with grief until
// until, attacking Bo from (23, 10).
func eaiGriefYard(t *testing.T, until uint64, armed bool) (g *Game, ada, bo, rook entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 31, Width: 48, Height: 24})
	bareGround(g)
	ada, _ = g.SpawnNamedPawn(20, 10, "Ada")
	bo, _ = g.SpawnNamedPawn(26, 10, "Bo")
	fullNeeds(g, ada, bo)
	if armed {
		g.Arm(ada, item.Club)
	}
	rook, _ = g.SpawnAnimal("labrador", 23, 10)
	g.TameAnimal(rook, "Rook")
	setAnimal(g, rook, func(a *animal.State) { a.Master, a.Manhunter = uint64(ada), until })
	holdAttacker(g, rook, bo, 23, 10)
	return g, ada, bo, rook
}

// TestGrievingPetIsEngaged: a tame animal gone manhunter is the colony's
// foe: hostile, a guard's foe, fought by an armed colonist, and shown as
// "Manhunter (grief)".
func TestGrievingPetIsEngaged(t *testing.T) {
	t.Parallel()
	now := uint64(0)
	g, ada, bo, rook := eaiGriefYard(t, now+animal.ManhunterTicks, true)
	now = uint64(g.clock.Now())
	if !g.hostile(animalOf(g, rook)) || !g.guardFoe(0, rook, now) {
		t.Fatal("a grieving manhunter is hostile and a guard's foe")
	}
	if v := eaiView(g, rook); !v.Grief || !v.Manhunter {
		t.Fatalf("the inspector shows the grief: %+v", v)
	}
	fought := false
	for i := 0; i < 30 && !fought; i++ { // the colonist looks for foes every few ticks
		holdAttacker(g, rook, bo, 23, 10)
		g.Step(1)
		j, _ := g.jobs.Get(ada)
		fought = j.Kind == job.Attack && j.Target == rook
	}
	if !fought {
		j, _ := g.jobs.Get(ada)
		t.Fatalf("an armed colonist fights the grieving manhunter: job %+v", j)
	}
	guard := eaiGuardDog(g, ada)
	a := animalOf(g, guard)
	if !g.masterDuty(guard, &a, dutyTick(guard, uint64(g.clock.Now()))) || a.Target != rook {
		t.Fatalf("a guard dog takes on the grieving manhunter: target %d", a.Target)
	}
}

// TestGrievingPetCalmsNoFlap: the grief runs out on its tick
// (ManhunterTicks after it began); the pet is tame and calm again, and
// nobody takes it up again.
func TestGrievingPetCalmsNoFlap(t *testing.T) {
	t.Parallel()
	g, _, _, rook := eaiGriefYard(t, animal.ManhunterTicks, false)
	r := rageOf(g)
	a := animalOf(g, rook)
	if !r.ThinkManhunter(rook, &a, animal.ManhunterTicks-1) {
		t.Fatal("still grieving the tick before the rage ends")
	}
	a = animalOf(g, rook)
	if r.ThinkManhunter(rook, &a, animal.ManhunterTicks) || a.Manhunter != 0 || a.Mode != animal.Wander || a.Target != 0 {
		t.Fatalf("calm once ManhunterTicks have passed: %+v", a)
	}

	// an armed colonist too: its attack ends with the grief (combat Strikes.Do)
	for _, armed := range []bool{false, true} {
		g, ada, _, rook := eaiGriefYard(t, uint64(g.clock.Now())+20, armed)
		guard := eaiGuardDog(g, ada)
		engaged := func() bool {
			j, _ := g.jobs.Get(ada)
			now := uint64(g.clock.Now())
			ga := animalOf(g, guard)
			return j.Kind == job.Attack && j.Target == rook || ga.Mode == animal.Attack && ga.Target == rook ||
				g.guardFoe(0, rook, now) || g.hostile(animalOf(g, rook))
		}
		for i := 0; i < 20; i++ {
			g.Step(1)
			fullNeeds(g, ada)
		}
		changes, last := 0, engaged()
		for i := 0; i < 600; i++ {
			g.Step(1)
			fullNeeds(g, ada)
			a := animalOf(g, rook)
			if a.Manhunter != 0 || !a.Tame || eaiView(g, rook).Grief {
				t.Fatalf("armed %v, tick %d: the grief came back: %+v", armed, i, a)
			}
			on := engaged()
			if on && !last {
				t.Fatalf("armed %v, tick %d: a calm pet was taken up again", armed, i)
			}
			if on != last {
				changes++
			}
			last = on
		}
		if last || changes > 1 {
			t.Fatalf("armed %v: the colony lets the calm pet be (engaged %v, %d changes)", armed, last, changes)
		}
	}
}

// TestFailedTamingRage: a whole game. A botched taming of an emu (it
// always takes revenge) sends it manhunter at the tamer; it reaches and
// kicks her; a day later it is calm and leaves her be.
func TestFailedTamingRage(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 7, Width: 40, Height: 20})
	bareGround(g)
	ada := draftedPawn(t, g, 10, 10) // she holds her ground
	fullNeeds(g, ada)
	emu, _ := g.SpawnAnimal("emu", 16, 10)
	herdHost{g}.TameFailed(ada, emu)
	if a := animalOf(g, emu); !a.Enraged(uint64(g.clock.Now())) || a.Target != ada || a.Mode != animal.Attack {
		t.Fatalf("a botched taming enrages the emu at its tamer: %+v", a)
	}
	hit := false
	for i := 0; i < 2000 && !hit; i++ {
		g.Step(1)
		fullNeeds(g, ada)
		_, hit = g.lastHit[ada]
	}
	if !hit {
		t.Fatalf("the enraged emu never reached its tamer: %+v", animalOf(g, emu))
	}
	g.setDraft(ada, false)
	g.clock.Advance(animal.ManhunterTicks)
	g.Step(1)
	if a := animalOf(g, emu); a.Manhunter != 0 || a.Mode == animal.Attack {
		t.Fatalf("a day later the emu is calm: %+v", a)
	}
	struck := g.lastHit[ada].Tick
	for i := 0; i < 600; i++ {
		g.Step(1)
		fullNeeds(g, ada)
		if a := animalOf(g, emu); a.Enraged(uint64(g.clock.Now())) || a.Target == ada {
			t.Fatalf("tick %d: the calm emu went for Ada again: %+v", i, a)
		}
	}
	if g.lastHit[ada].Tick != struck {
		t.Fatal("the calm emu kicked Ada again")
	}
}
