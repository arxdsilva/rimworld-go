package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017 §5: an animal corpse or egg the colony had no hand in is
// Unclaimed (haulers leave it until it is marked); kills by colonists,
// tame animals or the colony's traps, and every tame animal's death,
// leave the colony's own corpse.

// corpseAt is the carcass stack of species s lying at (x, y).
func corpseAt(g *Game, s animal.Species, x, y int) (item.Stack, bool) {
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.CorpseAnimal && st.Animal == uint8(s) && st.X == x && st.Y == y && st.Carrier == 0 {
			return st, true
		}
	}
	return item.Stack{}, false
}

// killWith kills animal id with one blow from by (0: no one).
func killWith(g *Game, id, by entity.EntityID) Position {
	p := pos(g, id)
	a := animalOf(g, id)
	g.hurt(id, animal.DefOf(a.Species).MaxHP+5, by)
	return p
}

func TestCorpseClaimFollowsTheKiller(t *testing.T) {
	g := New(Config{Seed: 1, Width: 40, Height: 20})
	bareGround(g)
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	rex, _ := g.SpawnAnimal("timber_wolf", 2, 18)
	g.TameAnimal(rex, "Rex")
	wild, _ := g.SpawnAnimal("cougar", 38, 18)
	raider := spawnRaiderAt(t, g, 38, 2, 1)
	cases := []struct {
		name      string
		by        entity.EntityID
		x         int
		unclaimed bool
	}{
		{"a colonist's kill", ada, 5, false},
		{"a tame animal's kill", rex, 10, false},
		{"a wild predator's kill", wild, 15, true},
		{"a raider's kill", raider, 20, true},
		{"no one (fire, a burst)", 0, 25, true},
	}
	for _, c := range cases {
		deer, _ := g.SpawnAnimal("deer", c.x, 10)
		killWith(g, deer, c.by)
		st, ok := corpseAt(g, animal.Deer, c.x, 10)
		if !ok {
			t.Fatalf("%s: no corpse", c.name)
		}
		if st.Unclaimed != c.unclaimed || st.HaulMarked {
			t.Errorf("%s: unclaimed %v, want %v", c.name, st.Unclaimed, c.unclaimed)
		}
	}
	// a tame animal's death is always the colony's, whoever killed it
	pet, _ := g.SpawnAnimal("husky", 30, 14)
	g.TameAnimal(pet, "Bo")
	killWith(g, pet, wild)
	if st, ok := corpseAt(g, animal.Husky, 30, 14); !ok || st.Unclaimed {
		t.Fatalf("a tame animal's corpse is the colony's: %+v (%v)", st, ok)
	}
}

func TestTrapKillIsTheColonys(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	placeBuilt(t, g, construction.SpikeTrap, construction.South, 12, 10)
	deer, err := g.SpawnAnimal("deer", 12, 10)
	if err != nil || g.combat.Traps.At(12, 10) == 0 {
		t.Fatalf("a deer on a built trap: %v", err)
	}
	setAnimal(g, deer, func(a *animal.State) { a.Injury = animal.DefOf(animal.Deer).MaxHP - 1 })
	for i := 0; i < 200 && animalAlive(g, deer); i++ {
		g.combat.Traps.Spring(deer, 12, 10) // a missed roll leaves the trap set
	}
	if animalAlive(g, deer) {
		t.Fatal("the trap never caught the deer")
	}
	// the re-laid trap's refund lies on the cell, so the carcass spills
	// beside it
	n := 0
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == item.CorpseAnimal {
			n++
			if st.Unclaimed {
				t.Fatalf("a trap's kill is the colony's: %+v", st)
			}
		}
	}
	if n != 1 {
		t.Fatalf("one carcass, got %d", n)
	}
}

func TestStarvedAndOldCorpsesClaim(t *testing.T) {
	// a wild animal that starves is unclaimed; a tame one is the colony's
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	wild, _ := g.SpawnAnimal("deer", 5, 10)
	tame, _ := g.SpawnAnimal("deer", 20, 10)
	g.TameAnimal(tame, "Clover")
	for _, id := range []entity.EntityID{wild, tame} {
		setAnimal(g, id, func(a *animal.State) {
			a.Food, a.Injury, a.NextThink = 0, animal.DefOf(animal.Deer).MaxHP-1, 1<<40
		})
	}
	wp, tp := pos(g, wild), pos(g, tame)
	for i := 0; i < 3*animal.StarveEvery && (animalAlive(g, wild) || animalAlive(g, tame)); i++ {
		if animalAlive(g, wild) {
			wp = pos(g, wild)
		}
		if animalAlive(g, tame) {
			tp = pos(g, tame)
		}
		g.Step(1)
	}
	if animalAlive(g, wild) || animalAlive(g, tame) {
		t.Fatal("both starve")
	}
	if st, ok := corpseAt(g, animal.Deer, wp.X, wp.Y); !ok || !st.Unclaimed {
		t.Fatalf("a wild animal's starved corpse is unclaimed: %+v (%v)", st, ok)
	}
	if st, ok := corpseAt(g, animal.Deer, tp.X, tp.Y); !ok || st.Unclaimed {
		t.Fatalf("a tame animal's starved corpse is the colony's: %+v (%v)", st, ok)
	}
	// old age: wild unclaimed, tame the colony's
	h := New(Config{Seed: 1, Width: 30, Height: 20})
	now := uint64(h.clock.Now())
	life := animal.LifeOf(animal.Goat)
	ow := spawnSexed(t, h, "goat", 5, 10, true, false, "")
	ot := spawnSexed(t, h, "goat", 20, 10, true, true, "Gran")
	for _, id := range []entity.EntityID{ow, ot} {
		setAnimal(h, id, func(a *animal.State) { a.Born = animal.BornAt(now, uint64(life.LifeDays)*clock.TicksPerDay) })
	}
	for day := uint64(1); day <= 200 && (animalAlive(h, ow) || animalAlive(h, ot)); day++ {
		advance(h, day*clock.TicksPerDay+dawnHour*clock.TicksPerHour)
		for _, id := range []entity.EntityID{ow, ot} {
			if animalAlive(h, id) {
				keepFed(h, id)
				setAnimal(h, id, func(a *animal.State) { a.Path, a.PathI, a.NextThink = nil, 0, 1<<40 })
			}
		}
		h.Step(1)
	}
	if animalAlive(h, ow) || animalAlive(h, ot) {
		t.Fatal("both die of old age")
	}
	if st, ok := corpseAt(h, animal.Goat, 5, 10); !ok || !st.Unclaimed {
		t.Fatalf("a wild animal's old-age corpse is unclaimed: %+v (%v)", st, ok)
	}
	if st, ok := corpseAt(h, animal.Goat, 20, 10); !ok || st.Unclaimed {
		t.Fatalf("a tame animal's old-age corpse is the colony's: %+v (%v)", st, ok)
	}
}

// Claimed and unclaimed carcasses of one species on one tile never merge,
// and a killer feeds on the stack its own kill went into.
func TestCorpseClaimsKeepApart(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	colony := g.dropCorpseAs(animal.Deer, 10, 10, false)
	found := g.dropCorpseAs(animal.Deer, 10, 10, true)
	if colony == 0 || found == 0 || colony == found {
		t.Fatalf("separate stacks: %d %d", colony, found)
	}
	a, _ := g.stacks.Get(colony)
	b, _ := g.stacks.Get(found)
	if a.Unclaimed || !b.Unclaimed || a.Qty != 1 || b.Qty != 1 {
		t.Fatalf("colony %+v found %+v", a, b)
	}
	// STO-2: a corpse is one per stack (wiki), so a second unclaimed
	// carcass lies on its own, still unclaimed
	again := g.dropCorpseAs(animal.Deer, b.X, b.Y, true)
	if st, _ := g.stacks.Get(again); again == found || !st.Unclaimed || st.Qty != 1 {
		t.Fatalf("second carcass %d %+v", again, st)
	}
	if st, _ := g.stacks.Get(found); st.Qty != 1 {
		t.Fatalf("unclaimed stack %+v", st)
	}
}

func TestWildEggsAreUnclaimed(t *testing.T) {
	for _, tame := range []bool{false, true} {
		g := New(Config{Seed: 1, Width: 30, Height: 20})
		warmDay(t, g)
		hen := spawnSexed(t, g, "turkey", 10, 10, true, tame, "Hen")
		setAnimal(g, hen, func(a *animal.State) { a.Mated = animal.MatedEggs })
		day := int(uint64(g.clock.Now())/clock.TicksPerDay) + 1
		for !animal.LaysToday(animal.Turkey, uint64(hen), day) {
			day++
		}
		advance(g, uint64(day)*clock.TicksPerDay+animal.LayTick(animal.Turkey, uint64(hen)))
		keepFed(g, hen)
		g.Step(1)
		_, st, ok := eggStack(g, item.FertileEgg)
		if !ok {
			t.Fatalf("tame %v: no egg", tame)
		}
		if st.Unclaimed == tame {
			t.Fatalf("tame %v: egg unclaimed %v", tame, st.Unclaimed)
		}
	}
}
