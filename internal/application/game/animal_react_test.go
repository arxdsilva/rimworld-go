package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Reactions to being hurt (PLAY-010 2.3 A, B).

// sizeExtremes is, per behaviour, its smallest and its largest species
// (one species when all share a size).
func sizeExtremes() map[animal.Behaviour][]animal.Species {
	lo, hi := map[animal.Behaviour]animal.Species{}, map[animal.Behaviour]animal.Species{}
	for _, s := range animal.All() {
		r := animal.RosterOf(s)
		if l, ok := lo[r.Behaviour]; !ok || r.Size < animal.RosterOf(l).Size {
			lo[r.Behaviour] = s
		}
		if h, ok := hi[r.Behaviour]; !ok || r.Size > animal.RosterOf(h).Size {
			hi[r.Behaviour] = s
		}
	}
	out := map[animal.Behaviour][]animal.Species{}
	for b, s := range lo {
		out[b] = []animal.Species{s}
		if hi[b] != s {
			out[b] = append(out[b], hi[b])
		}
	}
	return out
}

// pinAnimal keeps a test's attacker animal fed, rested and standing.
func pinAnimal(g *Game, id entity.EntityID) {
	setAnimal(g, id, func(a *animal.State) {
		a.Food, a.Rest, a.Hunger, a.NextThink, a.NextDecide = animal.Full, animal.Full, 0, 1<<40, 1<<40
		a.Mode, a.Target, a.Path, a.PathI = animal.Wander, 0, nil, 0
	})
}

func TestHurtAnimalAlwaysReacts(t *testing.T) {
	type attacker int
	const (
		byColonist attacker = iota
		byPredator
		byRaider
	)
	names := [...]string{"colonist", "wild predator", "raider"}
	for b := animal.Grazer; b <= animal.Legendary; b++ {
		for _, s := range sizeExtremes()[b] {
			for _, tame := range []bool{false, true} {
				if tame && b == animal.Hive {
					continue // hive insects are never tamed
				}
				for by := byColonist; by <= byRaider; by++ {
					for _, asleep := range []bool{false, true} {
						g := New(Config{Seed: 4, Width: 40, Height: 24})
						bareGround(g)
						id, err := g.spawnAnimal(s, 15, 12)
						if err != nil {
							t.Fatal(err)
						}
						if tame {
							g.TameAnimal(id, "Pip")
						}
						var foe entity.EntityID
						switch by {
						case byColonist:
							foe = draftedPawn(t, g, 18, 12)
							g.SetNeeds(foe, needs.Full, needs.Full)
						case byPredator:
							foe, _ = g.SpawnAnimal("cougar", 18, 12)
							pinAnimal(g, foe)
						case byRaider:
							g.Raid(1)
							foe = g.raiderIDs[0]
							g.positions.Set(foe, Position{X: 18, Y: 12})
							g.invalidateCreatures()
						}
						if asleep {
							setAnimal(g, id, func(a *animal.State) { a.Mode, a.Path, a.PathI = animal.Rest, nil, 0 })
						}
						g.hurt(id, 1, foe)
						g.Step(1)
						a := animalOf(g, id)
						if a.Mode != animal.Flee && a.Mode != animal.Attack {
							t.Errorf("%s (tame %v, asleep %v) hurt by a %s: ignored it (%s)",
								s, tame, asleep, names[by], a.Mode)
						}
						want := animal.Flee
						switch {
						case tame && by == byColonist, !animal.Fighter(s) && b != animal.Hive:
						default:
							want = animal.Attack
						}
						if a.Mode != want && !a.Enraged(uint64(g.clock.Now())) {
							t.Errorf("%s (tame %v, asleep %v) hurt by a %s: %s, want %s",
								s, tame, asleep, names[by], a.Mode, want)
						}
					}
				}
			}
		}
	}
}

func TestFighterFightsBackAgainstItsOwnKind(t *testing.T) {
	g := New(Config{Seed: 2, Width: 30, Height: 16})
	bareGround(g)
	boar, _ := g.SpawnAnimal("wild_boar", 10, 8)
	rival, _ := g.SpawnAnimal("wild_boar", 12, 8)
	pinAnimal(g, rival)
	g.hurt(boar, 1, rival)
	for i := 0; i < 5; i++ {
		g.Step(1)
		pinAnimal(g, rival)
	}
	if a := animalOf(g, boar); a.Mode != animal.Attack || a.Target != rival {
		t.Fatalf("a boar gored by another boar fights back: %+v", a)
	}
}

func TestFighterKeepsItsTarget(t *testing.T) {
	g := New(Config{Seed: 2, Width: 30, Height: 16})
	bareGround(g)
	boar, _ := g.SpawnAnimal("wild_boar", 10, 8)
	ada := draftedPawn(t, g, 12, 8)
	bo := draftedPawn(t, g, 8, 8)
	g.hurt(boar, 1, ada)
	g.hurt(boar, 1, bo)
	if a := animalOf(g, boar); a.Target != ada || a.Mode != animal.Attack {
		t.Fatalf("a fighter keeps its first foe (no hopping): %+v", a)
	}
}

func TestTameAnimalHurtByColonistFlees(t *testing.T) {
	for _, s := range []string{"bison", "ridgewolf", "cow"} {
		g := New(Config{Seed: 3, Width: 30, Height: 16})
		bareGround(g)
		id, _ := g.SpawnAnimal(s, 10, 8)
		g.TameAnimal(id, "Moss")
		ada := draftedPawn(t, g, 12, 8)
		g.hurt(id, 1, ada)
		for i := 0; i < 120; i++ {
			g.Step(1)
			if a := animalOf(g, id); a.Mode != animal.Flee {
				t.Fatalf("%s, tick %d: a tame animal shot by its own colonist runs, never fights: %+v", s, i, a)
			}
		}
		if p := pos(g, id); weapon.Distance(p.X, p.Y, 12, 8) <= 2 {
			t.Fatalf("%s: still beside the colonist at %v", s, p)
		}
	}
}

// A territorial animal hit again and again over three flee windows fights
// on while healthy, breaks off once badly hurt, and never goes back: at
// most two switches (to Attack, then to Flee). The attacker is a wild
// cougar held in place (a colonist's hits may turn it manhunter, which
// fights to the death by design).
func TestTerritorialReactionDoesNotFlap(t *testing.T) {
	g := New(Config{Seed: 5, Width: 40, Height: 20})
	bareGround(g)
	boar, _ := g.SpawnAnimal("wild_boar", 20, 10)
	cougar, _ := g.SpawnAnimal("cougar", 22, 10)
	hold := func() {
		pinAnimal(g, cougar)
		setAnimal(g, cougar, func(a *animal.State) { a.Injury, a.Attacker, a.Cooldown = 0, 0, 1<<20 }) // never strikes
	}
	prev, switches, broke := animal.Wander, 0, false
	for i := 0; i < 3*animal.HurtFleeTicks; i++ {
		hold()
		if i%30 == 0 && !broke || i%300 == 0 { // fewer once it ran, so it lives
			g.hurt(boar, 1, cougar)
		}
		g.Step(1)
		a, ok := g.animals.Get(boar)
		if !ok {
			t.Fatalf("tick %d: the boar died", i)
		}
		if a.Mode == prev {
			continue
		}
		switches++
		if prev == animal.Flee {
			t.Fatalf("tick %d: back from fleeing to %s: %+v", i, a.Mode, a)
		}
		if a.Mode == animal.Flee {
			broke = true
			if !badlyHurt(a) {
				t.Fatalf("tick %d: a healthy boar ran: %+v", i, a)
			}
		}
		prev = a.Mode
	}
	if switches > 2 || !broke {
		t.Fatalf("switches %d, broke off %v", switches, broke)
	}
}

// A tame sheep in a fenced pen, hit through the fence every 60 ticks by
// a colonist it cannot reach, moves after every hit and never leaves the
// pen.
func TestPennedSheepFleesEveryHit(t *testing.T) {
	g := New(Config{Seed: 7, Width: 30, Height: 20})
	bareGround(g)
	fenceRing(t, g, 10, 5, 16, 11) // inside: 11..15 x 6..10
	sheep, _ := g.SpawnAnimal("sheep", 12, 8)
	g.TameAnimal(sheep, "Burr")
	ada := draftedPawn(t, g, 9, 8)
	g.SetNeeds(ada, needs.Full, needs.Full)
	g.Step(1)
	for hit := 0; hit < 20; hit++ {
		at := pos(g, sheep)
		g.hurt(sheep, 1, ada)
		if a := animalOf(g, sheep); a.Mode != animal.Flee || a.PathI >= len(a.Path) {
			t.Fatalf("hit %d at %v: no flee goal picked: %+v", hit, at, a)
		}
		g.Step(60)
		p := pos(g, sheep)
		if p == at {
			t.Fatalf("hit %d: stood still at %v", hit, p)
		}
		if p.X <= 10 || p.X >= 16 || p.Y <= 5 || p.Y >= 11 {
			t.Fatalf("hit %d: left the pen: %v", hit, p)
		}
	}
}

// The flee spot search: a goal past the fence is never picked; the far
// side of the pen is.
func TestFleeSpotStaysReachable(t *testing.T) {
	g := New(Config{Seed: 7, Width: 30, Height: 20})
	bareGround(g)
	fenceRing(t, g, 10, 5, 16, 11)
	sheep, _ := g.SpawnAnimal("sheep", 11, 8)
	g.TameAnimal(sheep, "Burr")
	a := animalOf(g, sheep)
	p, ok := g.fleeSpot(&a, Position{X: 11, Y: 8}, Position{X: 9, Y: 8}, 8, true)
	if !ok || p.X != 15 {
		t.Fatalf("the far side of the pen (x 15), got %v %v", p, ok)
	}
	// open ground: today's straight-away goal
	h := New(Config{Seed: 7, Width: 40, Height: 20})
	bareGround(h)
	deer, _ := h.SpawnAnimal("deer", 20, 10)
	d := animalOf(h, deer)
	if p, ok := h.fleeSpot(&d, Position{X: 20, Y: 10}, Position{X: 18, Y: 10}, 8, true); !ok || p != (Position{X: 28, Y: 10}) {
		t.Fatalf("straight away 8 tiles, got %v %v", p, ok)
	}
	// boxed in on three sides by a fence and the attacker beside it: any
	// open neighbour not next to the attacker
	k := New(Config{Seed: 7, Width: 20, Height: 12})
	bareGround(k)
	fenceRing(t, k, 4, 4, 6, 6) // a one-cell pen at 5,5
	hen, _ := k.SpawnAnimal("chicken", 5, 5)
	k.TameAnimal(hen, "Wren")
	c := animalOf(k, hen)
	if _, ok := k.fleeSpot(&c, Position{X: 5, Y: 5}, Position{X: 3, Y: 5}, 8, true); ok {
		t.Fatal("a one-cell pen has nowhere to run")
	}
}

// A fighter that can't get at its attacker (a fence between, for a wild
// one) runs instead of standing at the fence.
func TestFighterFleesFoeItCannotReach(t *testing.T) {
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	bareGround(g)
	fenceRing(t, g, 10, 5, 18, 13)
	boar, _ := g.SpawnAnimal("wild_boar", 12, 9)
	cougar, _ := g.SpawnAnimal("cougar", 8, 9) // outside the fence
	pinAnimal(g, cougar)
	g.hurt(boar, 1, cougar)
	if a := animalOf(g, boar); a.Mode != animal.Flee || a.Target != 0 {
		t.Fatalf("a boar fenced off from its attacker runs: %+v", a)
	}
	g.Step(60)
	if p := pos(g, boar); p.X <= 12 {
		t.Fatalf("it ran away from the fence: %v", p)
	}
	// two tiles apart with the fence between: still out of reach
	h := New(Config{Seed: 3, Width: 30, Height: 20})
	bareGround(h)
	fenceRing(t, h, 10, 5, 18, 13)
	b2, _ := h.SpawnAnimal("wild_boar", 11, 9)
	c2, _ := h.SpawnAnimal("cougar", 9, 9)
	pinAnimal(h, c2)
	h.hurt(b2, 1, c2)
	if a := animalOf(h, b2); a.Mode != animal.Flee {
		t.Fatalf("two tiles apart across a fence is out of reach: %+v", a)
	}
}

// reachesFoe on a foe standing where the animal can't (a fence post): a
// walkable cell beside it must be reachable.
func TestReachesFoeOnAFence(t *testing.T) {
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	bareGround(g)
	fenceRing(t, g, 2, 2, 7, 7)
	fenceRing(t, g, 12, 2, 17, 7)
	boar := Position{X: 4, Y: 4}
	if !g.reachesFoe(false, boar, Position{X: 7, Y: 4}) {
		t.Fatal("a foe on its own pen's fence is reached from inside")
	}
	if g.reachesFoe(false, boar, Position{X: 12, Y: 4}) {
		t.Fatal("a foe on another closed pen's fence is out of reach")
	}
	if !g.reachesFoe(false, boar, Position{X: 5, Y: 5}) {
		t.Fatal("a foe beside it is reached")
	}
}

// TestPennedAnimalFleesInsidePen: a tame animal hit while in its pen runs
// to the pen's far side, never out through the gate (gates are open to
// tame animals, but a frightened penned animal stays home; PLAY-010).
func TestPennedAnimalFleesInsidePen(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	fenceRing(t, g, 12, 6, 20, 14, [2]int{13, 6})
	zoneCmd(g, command.DesignatePen, 13, 7, 19, 13, 0)
	sheep, _ := g.SpawnAnimal("sheep", 14, 8)
	g.TameAnimal(sheep, "Wool")
	wolf, _ := g.SpawnAnimal("timber_wolf", 13, 4)
	for hit := 0; hit < 10; hit++ {
		g.hurt(sheep, 0, wolf)
		for i := 0; i < 60; i++ {
			g.Step(1)
			p, _ := g.positions.Get(sheep)
			if !inside(p, 12, 6, 20, 14) {
				t.Fatalf("hit %d: the penned sheep fled out of its pen to %v", hit, p)
			}
			a := animalOf(g, sheep)
			if a.PathI < len(a.Path) && !inside(Position{X: a.GoalX, Y: a.GoalY}, 12, 6, 20, 14) {
				t.Fatalf("hit %d: flee goal %d,%d is outside the pen", hit, a.GoalX, a.GoalY)
			}
		}
		setAnimal(g, sheep, func(a *animal.State) { a.Injury = 0 })
	}
}
