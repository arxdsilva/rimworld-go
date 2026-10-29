package armor

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
)

func rolls(rs ...int) func(int) int {
	return func(n int) int {
		if n != RollMax {
			panic("armor rolls permille")
		}
		r := rs[0]
		rs = rs[1:]
		return r
	}
}

func TestApparelRatings(t *testing.T) {
	t.Parallel()
	for k, want := range map[item.Kind]Layer{
		item.Parka: {Set{162, 48, 300}, Torso}, item.WoolParka: {Set{72, 0, 220}, Torso},
		item.Duster: {Set{243, 72, 450}, Torso | Legs}, item.Tuque: {Set{162, 48, 300}, Head},
	} {
		if got, ok := Apparel(k); !ok || got != want {
			t.Errorf("%v = %+v %v", k, got, ok)
		}
	}
	if _, ok := Apparel(item.Wood); ok {
		t.Fatal("wood is no armor")
	}
}

func TestRegionOf(t *testing.T) {
	t.Parallel()
	if RegionOf(0) != Head || RegionOf(99) != Head || RegionOf(100) != Legs || RegionOf(299) != Legs ||
		RegionOf(300) != Torso || RegionOf(999) != Torso {
		t.Fatal("regions")
	}
}

func TestHitThroughLayers(t *testing.T) {
	t.Parallel()
	duster, _ := Apparel(item.Duster)
	tuque, _ := Apparel(item.Tuque)
	worn := []Layer{duster, tuque}
	// torso: only the duster rolls. A=243-180=63: deflect <31, halve <63.
	if r := Hit(worn, Torso, damage.Sharp, 12, 180, rolls(10)); r.Outcome != Deflected || r.Damage != 0 {
		t.Fatalf("deflect %+v", r)
	}
	if r := Hit(worn, Torso, damage.Sharp, 12, 180, rolls(40)); r.Outcome != Halved || r.Damage != 6 || r.Type != damage.Blunt {
		t.Fatalf("halve sharp becomes blunt %+v", r)
	}
	if r := Hit(worn, Torso, damage.Sharp, 12, 180, rolls(63)); r.Outcome != Passed || r.Damage != 12 || r.Type != damage.Sharp {
		t.Fatalf("pass %+v", r)
	}
	// blunt against the duster: A=72-0 -> halve at 50
	if r := Hit(worn, Legs, damage.Blunt, 10, 0, rolls(50)); r.Outcome != Halved || r.Damage != 5 || r.Type != damage.Blunt {
		t.Fatalf("blunt %+v", r)
	}
	// penetration beyond the rating: passes without effect
	if r := Hit(worn, Head, damage.Sharp, 18, 270, rolls(0)); r.Outcome != Passed || r.Damage != 18 {
		t.Fatalf("penetrated %+v", r)
	}
	// nothing covers the legs of a parka wearer: no roll drawn
	parka, _ := Apparel(item.Parka)
	if r := Hit([]Layer{parka}, Legs, damage.Sharp, 9, 0, rolls()); r.Damage != 9 {
		t.Fatalf("uncovered %+v", r)
	}
	if r := Hit(nil, Torso, damage.Sharp, 9, 0, rolls()); r.Damage != 9 {
		t.Fatal("naked")
	}
}
