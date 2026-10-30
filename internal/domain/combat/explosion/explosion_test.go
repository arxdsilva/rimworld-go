package explosion

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
)

var frag = Def{Kind: Bomb, Radius: 1900, Damage: 50, Pen: 100}

func TestDamageFallsWithDistance(t *testing.T) {
	if got := frag.At(0); got != 50 {
		t.Fatalf("centre = %d, want 50", got)
	}
	if got := frag.At(1900); got != 25 {
		t.Fatalf("rim = %d, want half (25)", got)
	}
	if frag.At(1901) != 0 || frag.At(-1) != 0 {
		t.Fatal("nothing past the rim")
	}
	prev := frag.At(0)
	for d := 100; d <= 1900; d += 100 {
		if c := frag.At(d); c > prev {
			t.Fatalf("damage rises at %d: %d > %d", d, c, prev)
		} else {
			prev = c
		}
	}
	if (Def{Kind: Flame, Radius: 1100, Damage: 1}).At(1100) != 1 {
		t.Fatal("a damaging blast deals at least 1 inside")
	}
}

func TestWithinAndReach(t *testing.T) {
	if !frag.Within(1, 1) || frag.Within(2, 0) {
		t.Fatal("1.9: the diagonal neighbour is in (1.41), two tiles out is not")
	}
	molotov := Def{Kind: Flame, Radius: 1100}
	if !molotov.Within(1, 0) || molotov.Within(1, 1) {
		t.Fatal("1.1: the orthogonal neighbours only")
	}
	if frag.Reach() != 2 || molotov.Reach() != 2 || (Def{Radius: 3000}).Reach() != 3 {
		t.Fatal("reach rounds up")
	}
	if Dist(3, 4) != 5000 || Dist(1, 1) != 1414 || Dist(0, 0) != 0 {
		t.Fatalf("dist: %d %d", Dist(3, 4), Dist(1, 1))
	}
}

func TestKinds(t *testing.T) {
	if frag.Type() != damage.Bomb || !frag.Flesh() || frag.Ignites() {
		t.Fatal("frag: bomb damage, wounds, no fire")
	}
	if frag.Against(0, Passable) != 100 || !frag.Structural() {
		t.Fatalf("bomb x2 against passable buildings: %d", frag.Against(0, Passable))
	}
	fl := Def{Kind: Flame, Radius: 1100, Damage: 10}
	if fl.Type() != damage.Heat || !fl.Ignites() || fl.Against(0, Wall) != 10 || fl.Against(0, Plant) != 0 {
		t.Fatal("flame: heat, ignites, x1 on buildings, plants left to the fire")
	}
	emp := Def{Kind: EMP, Radius: 3500, Damage: 50}
	if emp.Flesh() || emp.Structural() || emp.StunTicks(0) != 1500 || frag.StunTicks(0) != 0 {
		t.Fatal("EMP: no wounds or building damage, 30 ticks of stun per point (50 -> 1500)")
	}
	if (Def{Kind: Smoke}).Flesh() || !(Def{}).Zero() || frag.Zero() {
		t.Fatal("smoke harms nothing; the zero def is no blast")
	}
	for k := None; k <= numKinds; k++ {
		_ = k.String()
		_ = Def{Kind: k}.Type()
	}
	if Bomb.String() != "bomb" || None.String() != "" {
		t.Fatal("names")
	}
}

func TestForcedMiss(t *testing.T) {
	for _, c := range []struct{ dist, want int }{{3, 0}, {8, 0}, {12, 1}, {24, 1}, {30, 2}, {60, 2}} {
		if got := ForcedMiss(1900, c.dist); got != c.want {
			t.Errorf("ForcedMiss(1.9, %d) = %d, want %d", c.dist, got, c.want)
		}
	}
}

func TestCloudCrosses(t *testing.T) {
	c := Cloud{X: 5, Y: 0, Radius: 2400}
	if !c.Crosses(0, 0, 10, 0) || !c.Crosses(0, 2, 10, 2) {
		t.Fatal("a shot through the cloud")
	}
	if c.Crosses(0, 3, 10, 3) || c.Crosses(0, 0, 2, 0) || c.Crosses(10, 0, 8, 0) {
		t.Fatal("shots that pass it by or stop short")
	}
	if !c.Crosses(5, 1, 5, 1) || !c.Crosses(4, 0, 0, 0) {
		t.Fatal("standing in it")
	}
}

func TestCloudCrossesLongShot(t *testing.T) {
	c := Cloud{X: 150, Y: 151, Radius: 2400}
	if !c.Crosses(0, 0, 300, 300) || c.Crosses(0, 10, 300, 310) {
		t.Fatal("a long diagonal shot through, and one well clear")
	}
}

// TestAgainstTargets: wiki: Damage types: a bomb deals x2 to passable
// buildings, x4 to impassable ones and plants, x8 in all to walls.
func TestAgainstTargets(t *testing.T) {
	for _, c := range []struct {
		t    Target
		want int
	}{{Passable, 100}, {Impassable, 200}, {Wall, 400}, {Plant, 200}, {numTargets, 0}} {
		if got := frag.Against(0, c.t); got != c.want {
			t.Errorf("Against(%d) = %d, want %d", c.t, got, c.want)
		}
	}
	if (Def{Kind: numKinds, Damage: 5}).Against(0, Wall) != 0 || (Def{Kind: Smoke}).Against(0, Wall) != 0 {
		t.Fatal("unknown and smoke blasts leave structures whole")
	}
	if frag.Against(1000, Wall) != frag.At(1000)*8 {
		t.Fatal("the multiplier applies to the distance-scaled damage")
	}
}

// TestEMPAdaptation: a machine that recovered from a stun is adapted for
// 2200 ticks (wiki: Damage types).
func TestEMPAdaptation(t *testing.T) {
	if Adapted(0, 50) || Adapted(1000, 999) {
		t.Fatal("never stunned, or still stunned: not adapted")
	}
	if !Adapted(1000, 1000) || !Adapted(1000, 1000+EMPAdaptTicks-1) || Adapted(1000, 1000+EMPAdaptTicks) {
		t.Fatal("adapted for EMPAdaptTicks after recovering")
	}
}

// TestItemsPlantsAndCookOff: only a bomb wears stacks and tears plants
// (trees hold three times as long); bombs and flames set explosives off.
func TestItemsPlantsAndCookOff(t *testing.T) {
	fl := Def{Kind: Flame, Radius: 1100, Damage: 10}
	emp := Def{Kind: EMP, Radius: 3500, Damage: 50}
	if frag.Item(0) != 50 || frag.Item(1900) != 25 || fl.Item(0) != 0 || emp.Item(0) != 0 {
		t.Fatal("bomb damage on stacks only")
	}
	if frag.PlantPct(0, false) != 200 || frag.PlantPct(0, true) != 66 || fl.PlantPct(0, false) != 0 {
		t.Fatalf("plant damage: %d %d", frag.PlantPct(0, false), frag.PlantPct(0, true))
	}
	if !frag.Lights() || !fl.Lights() || emp.Lights() || (Def{Kind: Smoke}).Lights() {
		t.Fatal("bombs and flames set explosives off")
	}
}
