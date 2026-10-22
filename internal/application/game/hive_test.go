package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

func TestHiveSwarmEvent(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		g := New(Config{Seed: seed, Width: 40, Height: 30})
		_, _ = g.SpawnNamedPawn(20, 15, "Ada")
		// a rock outcrop to the east (nearest: its west face at x=30) and a
		// farther one in the north-west corner
		for y := 10; y < 20; y++ {
			for x := 30; x < 34; x++ {
				g.setTerrain(x, y, terrain.Rock)
			}
		}
		for y := 0; y < 3; y++ {
			for x := 0; x < 3; x++ {
				g.setTerrain(x, y, terrain.Rock)
			}
		}
		g.hiveSwarm()
		n, brutes := 0, 0
		for _, id := range g.animalIDs {
			a, _ := g.animals.Get(id)
			if !animal.DefOf(a.Species).Hive {
				t.Fatalf("a %s in the swarm", a.Species)
			}
			if a.Species == animal.HiveBrute {
				brutes++
			}
			if !a.Enraged(uint64(g.clock.Now()) + 100*clock.TicksPerDay) {
				t.Fatal("hive insects are hostile from spawn and never calm down")
			}
			p, _ := g.positions.Get(id)
			if p.X < 27 || p.X > 31 || p.Y < 8 || p.Y > 12 { // within 2 of the exit (29,10)
				t.Fatalf("seed %d: insect at %v, not by the rock face nearest the colony", seed, p)
			}
			n++
		}
		if n < 3 || n > 6 || brutes > n/3 {
			t.Fatalf("seed %d: %d insects, %d brutes", seed, n, brutes)
		}
		if !hasMessage(g, "A hive swarm bursts from the rock!") {
			t.Fatal("no message")
		}
	}
	// no rock: they come in at a map edge
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	_, _ = g.SpawnNamedPawn(15, 10, "Ada")
	g.hiveSwarm()
	if len(g.animalIDs) < 3 {
		t.Fatalf("edge swarm: %d", len(g.animalIDs))
	}
	for _, id := range g.animalIDs {
		p, _ := g.positions.Get(id)
		if p.X > 2 && p.Y > 2 && p.X < 27 && p.Y < 17 {
			t.Fatalf("edge insect at %v", p)
		}
	}
}

func TestThreatOdds(t *testing.T) {
	t.Parallel()
	count := func(winter bool, day int) map[threatKind]int {
		out := map[threatKind]int{}
		for roll := 0; roll < threatRolls; roll++ {
			out[pickThreat(roll, day, winter)]++
		}
		return out
	}
	if c := count(false, 7); c[threatRaid] != threatRolls {
		t.Fatalf("before day 8 only raids: %v", c)
	}
	if c := count(false, 10); c[threatManhunters] != 2 || c[threatHive] != 0 || c[threatRaid] != 6 {
		t.Fatalf("day 10: %v", c)
	}
	if c := count(false, 12); c[threatManhunters] != 2 || c[threatHive] != 1 || c[threatRaid] != 5 {
		t.Fatalf("day 12: %v", c)
	}
	if c := count(true, 12); c[threatPack] != 4 || c[threatHive] != 1 || c[threatRaid] != 3 {
		t.Fatalf("winter: %v", c)
	}
}

func TestHiveNeverTameable(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	p, _ := g.SpawnNamedPawn(2, 2, "Ada")
	id, _ := g.SpawnAnimal("hive_crawler", 10, 5)
	g.Apply(command.Command{Kind: command.DesignateTame, X: 0, Y: 0, X2: 19, Y2: 9})
	g.Step(1)
	if a, _ := g.animals.Get(id); a.TameMark {
		t.Fatal("a hive insect cannot be marked for taming")
	}
	if g.assignTame(p) {
		t.Fatal("no taming job for a hive insect")
	}
	g.hurt(id, 3, p)
	if a, _ := g.animals.Get(id); a.Mode == animal.Flee {
		t.Fatal("hive insects never flee")
	}
}

func TestLegendaryRevengeCallsHerd(t *testing.T) {
	g := New(Config{Seed: 5, Width: 60, Height: 20})
	p, _ := g.SpawnNamedPawn(10, 10, "Ada")
	stag, _ := g.SpawnAnimal("glimmerstag", 12, 10)
	mate, _ := g.SpawnAnimal("glimmerstag", 18, 12)
	far, _ := g.SpawnAnimal("glimmerstag", 58, 18)
	deer, _ := g.SpawnAnimal("deer", 13, 11)
	now := uint64(g.clock.Now())
	a, _ := g.animals.Get(stag)
	if a.Enraged(now) || animal.DefOf(animal.Glimmerstag).Predator {
		t.Fatal("a glimmerstag is peaceful")
	}
	g.hurt(stag, 5, p)
	for _, id := range []entity.EntityID{stag, mate} {
		if o, _ := g.animals.Get(id); !o.Enraged(now) || o.Target != p {
			t.Fatalf("legendary %d enraged at the attacker: %+v", id, o)
		}
	}
	if o, _ := g.animals.Get(far); o.Enraged(now) {
		t.Fatal("one across the map does not hear")
	}
	if o, _ := g.animals.Get(deer); o.Enraged(now) {
		t.Fatal("other species are not called")
	}
}

func TestLegendaryEventOncePerYear(t *testing.T) {
	g := New(Config{Seed: 7, Width: 30, Height: 20})
	_, _ = g.SpawnNamedPawn(15, 10, "Ada")
	g.wild = true
	glimmer := func() int {
		n := 0
		for _, s := range speciesOf(g) {
			if s == animal.Glimmerstag {
				n++
			}
		}
		return n
	}
	// never before day 20
	for d := 1; d < legendaryFromDay; d++ {
		g.legendaryVisit(uint64(d*clock.TicksPerDay+legendaryHour*clock.TicksPerHour), 0)
	}
	if glimmer() != 0 {
		t.Fatal("too early")
	}
	// a 2% roll that always succeeds: once, then not again that year
	day := legendaryFromDay
	for ; day < clock.DaysPerYear; day++ {
		g.legendaryVisit(uint64(day*clock.TicksPerDay+legendaryHour*clock.TicksPerHour), 0)
	}
	if glimmer() != 1 || !hasMessage(g, "A glimmerstag wanders in.") {
		t.Fatalf("one glimmerstag in the first year, got %d", glimmer())
	}
	g.legendaryVisit(uint64(clock.DaysPerYear*clock.TicksPerDay+legendaryHour*clock.TicksPerHour), legendaryChance)
	if glimmer() != 1 {
		t.Fatal("a failed roll brings nobody")
	}
	g.legendaryVisit(uint64((clock.DaysPerYear+1)*clock.TicksPerDay+legendaryHour*clock.TicksPerHour), 0)
	if glimmer() != 2 {
		t.Fatal("the next year another may come")
	}
	for _, id := range g.animalIDs {
		p, _ := g.positions.Get(id)
		if p.X > 1 && p.Y > 1 && p.X < 28 && p.Y < 18 {
			t.Fatalf("legendary not at an edge: %v", p)
		}
	}
	// not on a wrong hour, nor on a bare map
	h := New(Config{Seed: 7, Width: 30, Height: 20})
	h.legendaryVisit(uint64(30*clock.TicksPerDay+legendaryHour*clock.TicksPerHour), 0)
	h.wild = true
	h.legendaryVisit(uint64(30*clock.TicksPerDay+legendaryHour*clock.TicksPerHour+1), 0)
	if len(h.animalIDs) != 0 {
		t.Fatal("legendary off schedule")
	}
}
