package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Predator charges (PLAY-010 2.3 C).

// chargeScene is a sated, rested wild animal of species s that stays put
// unless it charges, with a drafted colonist 2 tiles away.
func chargeScene(t *testing.T, g *Game, s string) (beast, ada entity.EntityID) {
	t.Helper()
	bareGround(g)
	beast, err := g.SpawnAnimal(s, 15, 10)
	if err != nil {
		t.Fatal(err)
	}
	ada = draftedPawn(t, g, 17, 10)
	g.SetNeeds(ada, needs.Full, needs.Full)
	return beast, ada
}

// keepCalm holds an animal sated and rested and stops it wandering off.
func keepCalm(g *Game, id entity.EntityID) {
	setAnimal(g, id, func(a *animal.State) {
		a.Food, a.Rest, a.Hunger, a.NextThink = animal.Full, animal.Full, 0, 1<<40
	})
}

// healPawn keeps a test colonist standing.
func healPawn(g *Game, id entity.EntityID) {
	if v, ok := g.vital.Get(id); ok {
		v.Injury = 0
		g.vital.Set(id, v)
	}
}

// runUntilCharge steps up to ticks ticks and returns the tick a charge
// began (-1: none) and how many rolls were made before it.
func runUntilCharge(g *Game, beast, ada entity.EntityID, ticks int) (at, rolls int) {
	last := animalOf(g, beast).NextCharge
	for i := 0; i < ticks; i++ {
		keepCalm(g, beast)
		healPawn(g, ada)
		g.Step(1)
		a := animalOf(g, beast)
		if a.NextCharge != last {
			rolls++
			last = a.NextCharge
		}
		if a.ChargeSince != 0 {
			return i, rolls
		}
	}
	return -1, rolls
}

func TestPredatorChargesNearbyColonist(t *testing.T) {
	g := New(Config{Seed: 11, Width: 32, Height: 20})
	cougar, ada := chargeScene(t, g, "cougar")
	at, rolls := runUntilCharge(g, cougar, ada, 20*animal.ChargeCooldown)
	if at < 0 || rolls > 12 {
		t.Fatalf("a cougar beside a colonist charges at 30%% a roll: tick %d after %d rolls", at, rolls)
	}
	a := animalOf(g, cougar)
	if a.Mode != animal.Attack || a.Target != ada || a.HuntSince != 0 {
		t.Fatalf("charging: %+v", a)
	}
	if !hasLog(g, ada, "Charged by a cougar") || !hasLog(g, cougar, "Charged Colonist") {
		t.Fatalf("logs: %q / %q", logTexts(g, ada), logTexts(g, cougar))
	}
	g.viewGen++
	for _, av := range g.World().Animals {
		if av.ID == cougar && !strings.HasPrefix(av.State, "charging ") {
			t.Fatalf("the inspector reads %q", av.State)
		}
	}
	// the same seed charges on the same tick (determinism)
	h := New(Config{Seed: 11, Width: 32, Height: 20})
	c2, a2 := chargeScene(t, h, "cougar")
	if at2, _ := runUntilCharge(h, c2, a2, 20*animal.ChargeCooldown); at2 != at {
		t.Fatalf("not deterministic: %d vs %d", at, at2)
	}
}

func TestPackPredatorCharges(t *testing.T) {
	g := New(Config{Seed: 12, Width: 32, Height: 20})
	wolf, ada := chargeScene(t, g, "timber_wolf")
	if at, _ := runUntilCharge(g, wolf, ada, 20*animal.ChargeCooldown); at < 0 {
		t.Fatal("a wolf charges too")
	}
}

func TestNoChargeFromGrazersSleepersOrOnGentle(t *testing.T) {
	ticks := 15 * animal.ChargeCooldown
	for _, s := range []string{"deer", "bison", "cow"} {
		g := New(Config{Seed: 11, Width: 32, Height: 20})
		beast, ada := chargeScene(t, g, s)
		if at, _ := runUntilCharge(g, beast, ada, ticks); at >= 0 {
			t.Fatalf("a %s charged", s)
		}
	}
	// asleep
	g := New(Config{Seed: 11, Width: 32, Height: 20})
	cougar, _ := chargeScene(t, g, "cougar")
	for i := 0; i < ticks; i++ {
		setAnimal(g, cougar, func(a *animal.State) {
			a.Food, a.Hunger, a.Rest, a.Mode, a.Path, a.PathI = animal.Full, 0, 0, animal.Rest, nil, 0
		})
		g.Step(1)
		if a := animalOf(g, cougar); a.ChargeSince != 0 || a.NextCharge != 0 {
			t.Fatalf("tick %d: a sleeping cougar rolled: %+v", i, a)
		}
	}
	// tame
	g = New(Config{Seed: 11, Width: 32, Height: 20})
	cougar, ada := chargeScene(t, g, "cougar")
	g.TameAnimal(cougar, "Ash")
	if at, _ := runUntilCharge(g, cougar, ada, ticks); at >= 0 {
		t.Fatal("a tame cougar charged")
	}
	// Wanderer and Gentle: predators leave colonists alone
	for _, p := range []difficulty.Preset{difficulty.Wanderer, difficulty.Gentle} {
		g := New(Config{Seed: 11, Width: 32, Height: 20})
		g.ng.setup = &Setup{Preset: p, Difficulty: difficulty.Of(p), Storyteller: "steady"}
		cougar, ada := chargeScene(t, g, "cougar")
		if at, _ := runUntilCharge(g, cougar, ada, ticks); at >= 0 {
			t.Fatalf("preset %d: a cougar charged", p)
		}
	}
}

// After every roll the next waits ChargeCooldown ticks, and a charge that
// ends starts the cooldown again.
func TestChargeCooldownHolds(t *testing.T) {
	g := New(Config{Seed: 13, Width: 32, Height: 20})
	cougar, ada := chargeScene(t, g, "cougar")
	var rolls, charges int
	for i := 0; i < 12*animal.ChargeCooldown; i++ {
		keepCalm(g, cougar)
		healPawn(g, ada)
		before := animalOf(g, cougar)
		if before.ChargeSince != 0 { // end each charge at once: the colonist steps away
			g.positions.Set(ada, Position{X: 30, Y: 18})
			g.invalidateCreatures()
		} else if p := pos(g, ada); p.X != 17 {
			g.positions.Set(ada, Position{X: 17, Y: 10})
			g.invalidateCreatures()
		}
		g.Step(1)
		now := uint64(g.clock.Now())
		a := animalOf(g, cougar)
		if a.NextCharge == before.NextCharge {
			continue
		}
		if before.ChargeSince != 0 { // the charge ended: the cooldown starts again
			if a.ChargeSince != 0 || a.NextCharge+1 < now+animal.ChargeCooldown {
				t.Fatalf("tick %d: a charge ended without its cooldown: %+v", i, a)
			}
			continue
		}
		rolls++
		if now < before.NextCharge {
			t.Fatalf("tick %d: rolled %d ticks into its cooldown", i, before.NextCharge-now)
		}
		if a.ChargeSince != 0 {
			charges++
		}
	}
	if rolls < 5 || charges == 0 {
		t.Fatalf("rolls %d, charges %d", rolls, charges)
	}
}

func TestChargeBreaksOffBeyondLeash(t *testing.T) {
	g := New(Config{Seed: 11, Width: 40, Height: 20})
	cougar, ada := chargeScene(t, g, "cougar")
	if at, _ := runUntilCharge(g, cougar, ada, 20*animal.ChargeCooldown); at < 0 {
		t.Fatal("no charge")
	}
	p := pos(g, cougar)
	g.positions.Set(ada, Position{X: min(p.X+animal.ChargeLeash+2, 39), Y: p.Y})
	if p.X+animal.ChargeLeash+2 > 39 {
		g.positions.Set(ada, Position{X: p.X - animal.ChargeLeash - 2, Y: p.Y})
	}
	g.invalidateCreatures()
	g.Step(1)
	now := uint64(g.clock.Now())
	a := animalOf(g, cougar)
	if a.ChargeSince != 0 || a.Target == ada || a.Mode == animal.Attack || a.NextCharge < now+animal.ChargeCooldown-2 {
		t.Fatalf("a charge ends beyond %d tiles and the cooldown starts: %+v", animal.ChargeLeash, a)
	}
	// and after ChargeTicks however close
	h := New(Config{Seed: 11, Width: 40, Height: 20})
	c2, a2 := chargeScene(t, h, "cougar")
	if at, _ := runUntilCharge(h, c2, a2, 20*animal.ChargeCooldown); at < 0 {
		t.Fatal("no charge")
	}
	setAnimal(h, c2, func(a *animal.State) { a.ChargeSince = uint64(h.clock.Now()) - animal.ChargeTicks - 1 })
	h.Step(1)
	if a := animalOf(h, c2); a.ChargeSince != 0 || a.Target == a2 {
		t.Fatalf("a charge ends after %d ticks: %+v", animal.ChargeTicks, a)
	}
}

func TestChargeStateSurvivesSave(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	id, _ := g.SpawnAnimal("cougar", 5, 5)
	setAnimal(g, id, func(a *animal.State) { a.ChargeSince, a.NextCharge = 1234, 5678 })
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if a := animalOf(h, id); a.ChargeSince != 1234 || a.NextCharge != 5678 {
		t.Fatalf("lost: %+v", a)
	}
}
