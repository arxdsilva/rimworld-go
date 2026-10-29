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
	if frag.Building(0) != 100 {
		t.Fatalf("bomb x2 against buildings: %d", frag.Building(0))
	}
	fl := Def{Kind: Flame, Radius: 1100, Damage: 10}
	if fl.Type() != damage.Heat || !fl.Ignites() || fl.Building(0) != 10 {
		t.Fatal("flame: heat, ignites, x1 on buildings")
	}
	emp := Def{Kind: EMP, Radius: 3500, Damage: 50}
	if emp.Flesh() || emp.Building(0) != 0 || emp.StunTicks(0) != 750 || frag.StunTicks(0) != 0 {
		t.Fatal("EMP: no wounds or building damage, 0.25 s stun per point")
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
