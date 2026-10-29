package armor

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
)

func TestResolveBreakpoints(t *testing.T) {
	t.Parallel()
	// rating 400, pen 100 -> e=300: deflect <150, halve <300, pass else.
	for _, tc := range []struct {
		roll int
		out  Outcome
		dmg  int
	}{
		{0, Deflected, 0}, {149, Deflected, 0},
		{150, Halved, 6}, {299, Halved, 6},
		{300, Passed, 11}, {999, Passed, 11},
	} {
		o, d := Resolve(11, 100, 400, tc.roll)
		if o != tc.out || d != tc.dmg {
			t.Errorf("roll %d: got %v,%d want %v,%d", tc.roll, o, d, tc.out, tc.dmg)
		}
	}
}

func TestResolveEdges(t *testing.T) {
	t.Parallel()
	if o, d := Resolve(10, 500, 500, 0); o != Passed || d != 10 {
		t.Fatal("pen == rating passes")
	}
	if o, d := Resolve(10, 900, 100, 0); o != Passed || d != 10 {
		t.Fatal("pen > rating passes")
	}
	if o, d := Resolve(10, 0, 0, 0); o != Passed || d != 10 {
		t.Fatal("no armor passes")
	}
	if o, d := Resolve(0, 0, 900, 0); o != Passed || d != 0 {
		t.Fatal("no damage")
	}
	if _, d := Resolve(1, 0, 1000, 600); d != 1 {
		t.Fatalf("halved floors at 1: %d", d)
	}
	if o, _ := Resolve(5, 0, 1500, 999); o != Halved {
		t.Fatal("rating over 100 percent always at least halves")
	}
	if o, _ := Resolve(5, 0, 2000, 999); o != Deflected {
		t.Fatal("rating 200 percent always deflects below 1000 and is the cap")
	}
}

func TestResolveDeterministic(t *testing.T) {
	t.Parallel()
	for roll := 0; roll < RollMax; roll += 37 {
		o1, d1 := Resolve(14, 50, 640, roll)
		o2, d2 := Resolve(14, 50, 640, roll)
		if o1 != o2 || d1 != d2 {
			t.Fatal("not deterministic")
		}
	}
}

func TestSetAndType(t *testing.T) {
	t.Parallel()
	s := Set{Sharp: 700, Blunt: 300, Heat: 100}
	if s.Of(damage.ArmorSharp) != 700 || s.Of(damage.ArmorBlunt) != 300 ||
		s.Of(damage.ArmorHeat) != 100 || s.Of(damage.NoArmor) != 0 {
		t.Fatal("Of")
	}
	if got := s.Add(Set{1, 2, 3}); got != (Set{701, 302, 103}) {
		t.Fatal("Add")
	}
	if r := ResolveType(s, damage.Bomb, 10, 0, 0); r.Outcome != Deflected {
		t.Fatal("bomb uses sharp armor")
	}
	if r := ResolveType(s, damage.EMP, 10, 0, 0); r.Outcome != Passed || r.Damage != 10 {
		t.Fatal("EMP ignores armor")
	}
	if got := (Set{Sharp: 1500}).Add(Set{Sharp: 900}); got.Sharp != MaxRating {
		t.Fatal("cap")
	}
}

func TestSharpHalvedBecomesBlunt(t *testing.T) {
	t.Parallel()
	s := Set{Sharp: 600, Blunt: 100}
	if r := ResolveType(s, damage.Sharp, 10, 0, 400); r.Outcome != Halved || r.Damage != 5 || r.Type != damage.Blunt {
		t.Fatalf("halved sharp: %+v", r)
	}
	if r := ResolveType(s, damage.Sharp, 10, 0, 900); r.Type != damage.Sharp || r.Damage != 10 {
		t.Fatalf("passed sharp: %+v", r)
	}
	if r := ResolveType(s, damage.Sharp, 10, 0, 100); r.Outcome != Deflected || r.Type != damage.Sharp {
		t.Fatalf("deflected sharp: %+v", r)
	}
	if r := ResolveType(Set{Blunt: 600}, damage.Blunt, 10, 0, 400); r.Type != damage.Blunt || r.Damage != 5 {
		t.Fatalf("blunt: %+v", r)
	}
}

func TestResolveLayers(t *testing.T) {
	t.Parallel()
	outer, inner := Set{Sharp: 400}, Set{Sharp: 800}
	layers := []Set{outer, inner}
	// outer halves 20 -> 10 (roll 300 in [200,400)); inner (A=800) halves 10 -> 5.
	r := ResolveLayers(layers, damage.Sharp, 20, 0, []int{300, 500})
	if r.Damage != 5 || r.Outcome != Halved || r.Type != damage.Blunt {
		t.Fatalf("%+v", r)
	}
	// outer passes, inner deflects.
	r = ResolveLayers(layers, damage.Sharp, 20, 0, []int{900, 100})
	if r.Damage != 0 || r.Outcome != Deflected || r.Type != damage.Sharp {
		t.Fatalf("%+v", r)
	}
	// penetration is subtracted in every layer: A = 400-300 = 100.
	r = ResolveLayers(layers, damage.Sharp, 20, 300, []int{40, 900})
	if r.Damage != 0 && r.Outcome != Deflected {
		t.Fatalf("%+v", r)
	}
	// no layers / short rolls pass.
	if r := ResolveLayers(nil, damage.Sharp, 7, 0, nil); r.Damage != 7 || r.Outcome != Passed {
		t.Fatalf("%+v", r)
	}
	if r := ResolveLayers(layers, damage.Sharp, 7, 0, []int{}); r.Damage != 7 {
		t.Fatalf("%+v", r)
	}
}
