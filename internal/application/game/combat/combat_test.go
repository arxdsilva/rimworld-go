// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"rimworld-go/internal/domain/combat/cover"
	"slices"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

func TestLineOfSight(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Sight
	if !s.Line(2, 4, 8, 4) || !s.Line(8, 4, 2, 4) || !s.Line(3, 3, 3, 3) {
		t.Fatal("open field has sight")
	}
	f.build(1, construction.Wall, 5, 4)
	if s.Line(2, 4, 8, 4) {
		t.Fatal("a wall blocks sight")
	}
	if !s.Line(5, 4, 8, 4) || !s.Line(2, 4, 5, 4) {
		t.Fatal("the endpoints never block")
	}
	f.bldgs[1] = construction.Building{Kind: construction.Door, X: 5, Y: 4}
	if s.Line(2, 4, 8, 4) {
		t.Fatal("a door blocks sight")
	}
	f.bldgs[1] = construction.Building{Kind: construction.Sandbags, X: 5, Y: 4}
	if !s.Line(2, 4, 8, 4) {
		t.Fatal("sandbags don't block sight")
	}
	f.rock[[2]int{4, 6}] = true
	if s.Line(2, 8, 6, 4) || !s.Line(2, 2, 6, 6) {
		t.Fatal("rock blocks only the line through it")
	}
}

func TestCanHit(t *testing.T) {
	t.Parallel()
	f := newFake(30, 10)
	s := f.combat().Sight
	fists, rifle := weapon.DefOf(weapon.Fists), weapon.DefOf(weapon.Rifle)
	if !s.CanHit(5, 5, fists, 6, 5) || s.CanHit(5, 5, fists, 7, 5) {
		t.Fatal("fists reach one tile")
	}
	if !s.CanHit(5, 5, rifle, 5+rifle.Range, 5) || s.CanHit(5, 5, rifle, 6+rifle.Range, 5) {
		t.Fatal("a rifle reaches its range")
	}
	f.build(1, construction.Wall, 8, 5)
	if s.CanHit(5, 5, rifle, 12, 5) {
		t.Fatal("a gun needs sight")
	}
	f.build(2, construction.Wall, 6, 6)
	if !s.CanHit(5, 5, rifle, 6, 6) {
		t.Fatal("a gun touching its target needs no sight")
	}
}

func TestCoverFacesTheShooter(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	s := f.combat().Sight
	f.build(1, construction.Sandbags, 9, 10)
	want := cover.Permille(cover.Sandbags) // DEF-3: permille, the cover domain's value
	if c := s.Cover(2, 10, 10, 10); c != want {
		t.Fatalf("cover from the west %d, want %d", c, want)
	}
	if c := s.Cover(18, 10, 10, 10); c != 0 {
		t.Fatalf("no cover from the open side, got %d", c)
	}
	if c := s.Cover(9, 10, 10, 10); c != 0 {
		t.Fatalf("no cover at touching distance, got %d", c)
	}
	if c := s.Cover(18, 2, 10, 10); c != 0 {
		t.Fatalf("diagonal shot from the north-east: nothing beside, got %d", c)
	}
	if c := s.Cover(2, 18, 10, 10); c != want*cover.AngleFactor(2, 18, 10, 10, 9, 10)/1000 || c != want*400/1000 {
		t.Fatalf("diagonal shot from the south-west: the sandbags beside count at 45 deg (40 %%), got %d", c)
	}
	if c := s.Cover(7, 10, 10, 10); c != want*cover.NearFactor/1000 {
		t.Fatalf("one tile from the cover it holds two thirds, got %d", c)
	}
	if c := s.Cover(8, 10, 10, 10); c != want*cover.FrontFactor/1000 {
		t.Fatalf("right in front of the cover it holds a third, got %d", c)
	}
}

func TestReachHysteresis(t *testing.T) {
	t.Parallel()
	if a, r := Reach(true, false); a != AnimalReach || r != RaiderReach {
		t.Fatal("armed reach")
	}
	a, _ := Reach(false, false)
	ca, cr := Reach(false, true)
	if a != FleeReach || ca != FleeRelease || cr != RaiderRelease || FleeRelease <= FleeReach || AnimalRelease <= AnimalReach {
		t.Fatal("a fleeing colonist must let go further out than it started running")
	}
}

func TestTrapsSpringAndRelay(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	c := f.combat()
	tr := c.Traps
	if tr.Has() {
		t.Fatal("no traps yet")
	}
	f.build(5, construction.SpikeTrap, 10, 5)
	f.build(6, construction.Wall, 3, 3)
	if !tr.Has() || tr.At(10, 5) != 5 || tr.At(3, 3) != 0 || tr.At(1, 1) != 0 {
		t.Fatal("trap lookup")
	}
	f.raider(9, 10, 5, weapon.Fists)
	f.rolls = []int{200} // the damage roll: x1.0
	tr.Underfoot(nil)
	tr.Underfoot([]entity.EntityID{9})
	spikes := []string{"9-20-0-stab", "9-20-0-stab", "9-20-0-stab", "9-20-0-stab", "9-20-0-stab"} // wiki: 5 stabs of 100/5 (steel)
	if tr.At(10, 5) != 0 || !slices.Equal(f.hurts, spikes) || len(f.frames) != 1 || f.frames[0] != construction.SpikeTrap {
		t.Fatalf("a raider always springs it: hurts %v frames %v", f.hurts, f.frames)
	}
	if len(f.msgs) != 1 || f.msgs[0] != "A spike trap caught p9." || f.logs[len(f.logs)-1] != "trap 9 100" {
		t.Fatalf("message %v logs %v", f.msgs, f.logs)
	}
	f.build(7, construction.SpikeTrap, 4, 4)
	f.beast(20, 4, 4, animal.State{Species: animal.TimberWolf})
	f.rolls = []int{trapWildPct} // a wild animal springs it only 20 % of the time
	tr.Spring(20, 4, 4)
	if tr.At(4, 4) == 0 || len(f.msgs) != 1 {
		t.Fatal("a missed roll leaves the trap set")
	}
	b := f.bldgs[7]
	b.Stuff = item.Wood
	f.bldgs[7] = b
	f.hurts = nil
	f.rolls = []int{trapWildPct - 1, 200}
	tr.Spring(20, 4, 4)
	if f.msgs[1] != "A spike trap caught a "+animal.TimberWolf.String()+"." || len(f.hurts) != 5 || f.hurts[0] != "20-8-0-stab" {
		t.Fatalf("a wooden trap stabs 5 x 8: %v %v", f.msgs, f.hurts)
	}
	if !tr.Victim(animal.State{Species: animal.TimberWolf}) || !tr.Victim(animal.State{Species: animal.Deer}) ||
		tr.Victim(animal.State{Species: animal.TimberWolf, Tame: true}) {
		t.Fatal("trap victims: every wild animal")
	}
	f.build(8, construction.SpikeTrap, 6, 6)
	f.beast(21, 6, 6, animal.State{Species: animal.Deer, Manhunter: f.now + 1})
	f.rolls = []int{99}
	tr.Spring(21, 6, 6)
	if tr.At(6, 6) != 0 {
		t.Fatal("a manhunter always springs it")
	}
}

// TestTrapsNimble: a Nimble victim (Light-footed) springs any trap only
// trapNimblePct of the time, rolled first and only for it. wiki: Trap
// Spring Chance (Nimble ×10 %)
func TestTrapsNimble(t *testing.T) {
	t.Parallel()
	nimble := attack.Melee{Nimble: true, Sight: 1000, Manip: 1000, Moving: 1000}
	// step: one raider on a fresh trap of kind k with the queued rolls;
	// returns the fake, the traps and the draws taken.
	step := func(k construction.Kind, isNimble bool, rolls ...int) (*fakeHost, *Traps, int) {
		f := newFake(20, 10)
		tr := f.combat().Traps
		f.build(5, k, 10, 5)
		f.raider(9, 10, 5, weapon.Fists)
		if isNimble {
			f.melee[9] = nimble
		}
		f.rolls, f.rolled = rolls, 0
		tr.Spring(9, 10, 5)
		return f, tr, f.rolled
	}
	// a step on no trap draws and asks nothing
	f := newFake(20, 10)
	f.raider(9, 3, 3, weapon.Fists)
	f.melee[9] = nimble
	f.combat().Traps.Spring(9, 3, 3)
	if f.rolled != 0 {
		t.Fatalf("no trap: %d draws", f.rolled)
	}
	// spike trap: a miss leaves it set
	f, tr, n := step(construction.SpikeTrap, true, trapNimblePct)
	if tr.At(10, 5) != 5 || len(f.msgs) != 0 || len(f.hurts) != 0 || len(f.frames) != 0 || len(f.logs) != 0 || n != 1 {
		t.Fatalf("nimble miss: msgs %v hurts %v frames %v draws %d", f.msgs, f.hurts, f.frames, n)
	}
	// non-Nimble baseline: the damage roll only
	f, tr, base := step(construction.SpikeTrap, false, 200)
	if base != baseSpikeDraws || tr.At(10, 5) != 0 || len(f.hurts) != 5 {
		t.Fatalf("non-nimble spike: %d draws (want %d), hurts %v", base, baseSpikeDraws, f.hurts)
	}
	// Nimble hit: one more draw than the baseline, the same spring
	f, tr, n = step(construction.SpikeTrap, true, trapNimblePct-1, 200)
	if tr.At(10, 5) != 0 || len(f.hurts) != 5 || len(f.msgs) != 1 || n != base+1 {
		t.Fatalf("nimble hit: hurts %v msgs %v draws %d, want %d", f.hurts, f.msgs, n, base+1)
	}
	// charge: a miss keeps it unlit, a hit lights it
	_, tr, n = step(construction.IEDHighExplosive, true, trapNimblePct)
	if tr.Burning(5) || n != 1 {
		t.Fatalf("nimble miss on a charge: burning %v draws %d", tr.Burning(5), n)
	}
	_, tr, n = step(construction.IEDHighExplosive, true, trapNimblePct-1)
	if !tr.Burning(5) || n != 1 {
		t.Fatalf("nimble hit on a charge: burning %v draws %d", tr.Burning(5), n)
	}
	// non-Nimble on a charge draws nothing
	_, tr, n = step(construction.IEDHighExplosive, false)
	if !tr.Burning(5) || n != 0 {
		t.Fatalf("non-nimble charge: burning %v draws %d", tr.Burning(5), n)
	}
	// a Nimble victim on a charge whose fuse already burns draws the roll
	// and then light dedupes: accepted, one fuse stays
	f, tr, _ = step(construction.IEDHighExplosive, true, trapNimblePct-1)
	f.rolls, f.rolled = []int{0}, 0
	tr.Spring(9, 10, 5)
	if f.rolled != 1 || len(tr.fuses) != 1 {
		t.Fatalf("lit charge: %d draws, %d fuses", f.rolled, len(tr.fuses))
	}
	// a Nimble wild beast: the Nimble roll comes first, then the wild one
	f = newFake(20, 10)
	tr = f.combat().Traps
	f.build(7, construction.SpikeTrap, 4, 4)
	f.beast(20, 4, 4, animal.State{Species: animal.TimberWolf})
	f.melee[20] = nimble
	f.rolls = []int{trapNimblePct - 1, trapWildPct}
	tr.Spring(20, 4, 4)
	if tr.At(4, 4) == 0 || len(f.msgs) != 0 || f.rolled != 2 || len(f.rolls) != 0 {
		t.Fatalf("nimble beast: wild miss must follow the nimble hit: draws %d left %v", f.rolled, f.rolls)
	}
	f.rolls = []int{trapNimblePct, 0}
	f.rolled = 0
	tr.Spring(20, 4, 4)
	if f.rolled != 1 || len(f.rolls) != 1 || tr.At(4, 4) == 0 {
		t.Fatalf("nimble beast miss stops before the wild roll: draws %d", f.rolled)
	}
}

// baseSpikeDraws is a non-Nimble raider's draws on a spike trap: the
// damage roll (measured on the tree before the Nimble roll existed).
const baseSpikeDraws = 1
