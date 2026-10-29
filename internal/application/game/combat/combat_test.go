package combat

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
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
	want := construction.CoverPct(construction.Sandbags)
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
	if c := s.Cover(2, 18, 10, 10); c != want {
		t.Fatalf("diagonal shot from the south-west: the sandbags beside count, got %d", c)
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
	f.rolls = []int{trapChance} // a miss leaves it set
	tr.Underfoot([]entity.EntityID{9})
	if tr.At(10, 5) == 0 || len(f.hurts) != 0 {
		t.Fatal("a missed roll leaves the trap set")
	}
	tr.Underfoot(nil)
	tr.Underfoot([]entity.EntityID{9})
	if tr.At(10, 5) != 0 || len(f.hurts) != 1 || f.hurts[0] != "9-32-0" || len(f.frames) != 1 || f.frames[0] != construction.SpikeTrap {
		t.Fatalf("sprung: hurts %v frames %v", f.hurts, f.frames)
	}
	if len(f.msgs) != 1 || f.msgs[0] != "A spike trap caught p9." {
		t.Fatalf("message %v", f.msgs)
	}
	f.build(7, construction.SpikeTrap, 4, 4)
	f.beast(20, 4, 4, animal.State{Species: animal.TimberWolf})
	tr.Spring(20, 4, 4)
	if f.msgs[1] != "A spike trap caught a "+animal.TimberWolf.String()+"." {
		t.Fatalf("animal message %v", f.msgs)
	}
	if !tr.Victim(animal.State{Species: animal.TimberWolf}) || tr.Victim(animal.State{Species: animal.Deer}) ||
		!tr.Victim(animal.State{Species: animal.Deer, Manhunter: f.now + 1}) || tr.Victim(animal.State{Species: animal.TimberWolf, Tame: true}) {
		t.Fatal("trap victims: wild predators and manhunters")
	}
}
