package game

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
)

func TestHunterTamesMarkedAnimal(t *testing.T) {
	// SIM-024: seed 6 (was 4). Taming is a 30% roll every TameRetryTicks;
	// paced animal thinking reorders the shared rng draws, and seed 4 now
	// fails all three tries that fit in the test's 15000 ticks.
	g := New(Config{Seed: 6, Width: 24, Height: 14})
	ada, _ := g.SpawnNamedPawn(4, 7, "Ada")
	buck, _ := g.SpawnAnimal("dustbuck", 16, 7)
	// FOOD-3: a deer (wildness 75 %) needs Animals 7 and costs food per try
	setSkill(g, ada, skill.Animals, 20)
	spawn(t, g, item.Potatoes, 75, 4, 9)
	g.Enqueue(command.Command{Kind: command.DesignateTame, X: 16, Y: 7, X2: 16, Y2: 7})
	for i := 0; i < 600; i++ {
		g.Step(50)
		g.SetNeeds(ada, 1<<20, 1<<20)
		if a, _ := g.animals.Get(buck); a.Tame {
			break
		}
	}
	a, ok := g.animals.Get(buck)
	if !ok || !a.Tame || a.Name == "" {
		t.Fatalf("the dustbuck should end up tame and named: %+v", a)
	}
	var view AnimalView
	for _, v := range g.World().Animals {
		if v.ID == buck {
			view = v
		}
	}
	if !view.Tame || view.Name != a.Name {
		t.Fatalf("view %+v", view)
	}
	g.Enqueue(command.Command{Kind: command.ReleaseAnimal, Pawn: uint64(buck)})
	g.Step(1)
	if a, _ := g.animals.Get(buck); a.Tame {
		t.Fatal("released animals go wild")
	}
}

func TestManhunterHuntsColonistsAndPausesAsThreat(t *testing.T) {
	g := New(Config{Seed: 5, Width: 30, Height: 14})
	ada, _ := g.SpawnNamedPawn(3, 7, "Ada")
	wolf, _ := g.SpawnAnimal("ridgewolf", 26, 7)
	a, _ := g.animals.Get(wolf)
	g.enrage(wolf, &a, 0, uint64(g.clock.Now()))
	g.animals.Set(wolf, a)
	msgs := g.World().Messages
	if len(msgs) == 0 || !msgs[len(msgs)-1].Danger || !strings.Contains(msgs[len(msgs)-1].Text, "manhunter") {
		t.Fatalf("a manhunter is announced as a threat: %+v", msgs)
	}
	hurt := false
	for i := 0; i < 80 && !hurt; i++ {
		g.Step(25)
		hurt = g.VitalOf(ada).Injury > 0
	}
	if !hurt {
		t.Fatal("a manhunter crosses the map to attack a colonist")
	}
}

func TestProvokedAnimalsSometimesGoManhunter(t *testing.T) {
	enraged := 0
	for seed := uint64(1); seed <= 30; seed++ {
		g := New(Config{Seed: seed, Width: 20, Height: 10})
		ada, _ := g.SpawnNamedPawn(3, 5, "Ada")
		buck, _ := g.SpawnAnimal("caribou", 15, 5) // FOOD-3: a caribou turns 10 % of the time (wiki); deer and elk never
		g.hurt(buck, 5, ada)
		if a, _ := g.animals.Get(buck); a.Enraged(uint64(g.clock.Now())) {
			enraged++
		}
	}
	if enraged == 0 || enraged > 12 {
		t.Fatalf("hurt dustbucks rarely turn: %d/30", enraged)
	}
}

func TestTameWolfGuardsAgainstRaiders(t *testing.T) {
	g := New(Config{Seed: 6, Width: 30, Height: 14})
	_, _ = g.SpawnNamedPawn(5, 7, "Ada")
	wolf, _ := g.SpawnAnimal("ridgewolf", 8, 7)
	g.TameAnimal(wolf, "Ash")
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.positions.Set(rid, Position{X: 14, Y: 7})
	for i := 0; i < 40; i++ {
		g.Step(5)
		if a, _ := g.animals.Get(wolf); a.Target == rid {
			if g.World().Animals[0].Hostile {
				t.Fatal("a guard wolf is not a threat to the colony")
			}
			return
		}
	}
	t.Fatal("a tame wolf goes for a raider near the colony")
}

// PLAY-015: livestock go into their pen on a handler's rope, through the
// pen's door (alone they can't pass it).
func TestTameAnimalsKeepToPensAndUseDoors(t *testing.T) {
	g := New(Config{Seed: 7, Width: 30, Height: 14})
	hut(t, g, 16, 2, 24, 10) // a walled pen with a door
	buildMarker(t, g, 20, 6)
	buck, _ := g.SpawnAnimal("dustbuck", 4, 6)
	g.TameAnimal(buck, "Clover")
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	handlerOnly(g, ada)
	inside := false
	for i := 0; i < 200 && !inside; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(50)
		p, _ := g.positions.Get(buck)
		inside = p.X >= 17 && p.X <= 23 && p.Y >= 3 && p.Y <= 9
	}
	if !inside {
		p, _ := g.positions.Get(buck)
		t.Fatalf("a handler leads the tame animal through the door into its pen (at %v)", p)
	}
	if len(g.World().Pens) != 1 {
		t.Fatal("pen in the world view")
	}
	// legacy painted pens: still a zone kind to paint and cancel
	g.Enqueue(command.Command{Kind: command.DesignatePen, X: 17, Y: 3, X2: 23, Y2: 9})
	g.Step(1)
	if len(zonesOf(g.World(), ZonePen)) != 1 {
		t.Fatal("pen zone in the world view")
	}
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 17, Y: 3, X2: 23, Y2: 9})
	g.Step(1)
	if len(g.zones.Zones()) != 0 {
		t.Fatal("cancel removes the pen")
	}
}
