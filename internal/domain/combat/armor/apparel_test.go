package armor

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
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
		item.Parka: {Set{162, 48, 300}, coatCover, outer}, item.WoolParka: {Set{72, 0, 220}, coatCover, outer},
		item.Duster: {Set{243, 72, 450}, longCover, outer}, item.Tuque: {Set{162, 48, 300}, CoverUpperHead, headgear},
	} {
		if got, ok := Apparel(k); !ok || got != want {
			t.Errorf("%v = %+v %v", k, got, ok)
		}
	}
	if _, ok := Apparel(item.Wood); ok {
		t.Fatal("wood is no armor")
	}
}

// wear is a garment kind's layer at normal quality.
func wear(k item.Kind) Layer { l, _ := Apparel(k); return l }

// TestHelmetOnlyHead: a helmet takes hits on the brain and ears, never on
// the torso or the nose; a marine helmet covers the whole head.
func TestHelmetOnlyHead(t *testing.T) {
	t.Parallel()
	helmet := []Layer{wear(item.SimpleHelmet)}
	cut := damage.Hit{Form: damage.FormCut, Amount: 10}
	if l := Struck(helmet, GroupOf(body.Brain), cut, rolls(0), nil); l.Outcome != Deflected {
		t.Fatalf("brain hit through the helmet %+v", l)
	}
	if l := Struck(helmet, GroupOf(body.EarL), cut, rolls(0), nil); l.Outcome != Deflected {
		t.Fatalf("ear %+v", l)
	}
	for _, p := range []body.Part{body.Torso, body.Nose, body.EyeL, body.LegL, body.ArmR} {
		if l := Struck(helmet, GroupOf(p), cut, rolls(), nil); l.Amount != 10 {
			t.Fatalf("%v is not under a simple helmet: %+v", p, l)
		}
	}
	marine := []Layer{wear(item.MarineHelmet)}
	for _, p := range []body.Part{body.Nose, body.EyeR, body.Jaw, body.Brain} {
		if l := Struck(marine, GroupOf(p), cut, rolls(0), nil); l.Outcome != Deflected {
			t.Fatalf("%v under a marine helmet %+v", p, l)
		}
	}
}

// TestFlakVestTorsoNotLegs: the vest covers the torso and its organs,
// not the legs or the arms.
func TestFlakVestTorsoNotLegs(t *testing.T) {
	t.Parallel()
	vest := []Layer{wear(item.FlakVest)}
	bullet := damage.Hit{Form: damage.FormBullet, Amount: 10, Pen: 150}
	for _, p := range []body.Part{body.Torso, body.Heart, body.LungL} {
		if l := Struck(vest, GroupOf(p), bullet, rolls(0), nil); l.Outcome != Deflected {
			t.Fatalf("%v %+v", p, l)
		}
	}
	for _, p := range []body.Part{body.LegL, body.FootR, body.ArmL} {
		if l := Struck(vest, GroupOf(p), bullet, rolls(), nil); l.Amount != 10 {
			t.Fatalf("%v is bare under a vest %+v", p, l)
		}
	}
}

// TestLayersOutermostFirst: a duster over flak pants takes a leg hit
// first, whatever order they are listed in, and each garment's share is
// what reached it.
func TestLayersOutermostFirst(t *testing.T) {
	t.Parallel()
	pants, duster := wear(item.FlakPants), wear(item.Duster)
	worn := []Layer{pants, duster} // inner listed first
	cut := damage.Hit{Form: damage.FormCut, Amount: 20}
	took := make([]int, 2)
	// duster (A=243) halves at roll 200: 20 -> 10; pants (A=550) halve at 400: 10 -> 5
	l := Struck(worn, GroupOf(body.LegL), cut, rolls(200, 400), took)
	if l.Amount != 5 || l.Outcome != Halved || l.Form != damage.FormBlunt || took[1] != 20 || took[0] != 10 {
		t.Fatalf("stacked %+v took %v", l, took)
	}
	// the torso: only the duster covers it
	took = make([]int, 2)
	if l := Struck(worn, GroupOf(body.Torso), cut, rolls(999), took); l.Amount != 20 || took[0] != 0 || took[1] != 20 {
		t.Fatalf("torso %+v took %v", l, took)
	}
	// a deflection spares the inner layer: no roll, no share
	took = make([]int, 2)
	if l := Struck(worn, GroupOf(body.LegR), cut, rolls(0), took); !l.Stopped() || took[0] != 0 || took[1] != 20 {
		t.Fatalf("deflected %+v took %v", l, took)
	}
	if l := Struck(nil, CoverTorso, cut, rolls(), nil); l.Amount != 20 {
		t.Fatal("naked")
	}
}

func TestConflicts(t *testing.T) {
	t.Parallel()
	if !wear(item.MarineArmor).Conflicts(wear(item.FlakPants)) || !wear(item.FlakVest).Conflicts(wear(item.PlateArmor)) {
		t.Fatal("same ply, overlapping groups conflict")
	}
	if wear(item.Duster).Conflicts(wear(item.FlakPants)) || wear(item.FlakVest).Conflicts(wear(item.Parka)) ||
		wear(item.SimpleHelmet).Conflicts(wear(item.FlakJacket)) {
		t.Fatal("different plies or groups do not")
	}
	if GroupOf(body.NumParts) != 0 || GroupOf(body.Spine) != CoverTorso {
		t.Fatal("GroupOf")
	}
}

// TestLayeredClothing (TEMP-4): the core clothing rates its material's
// armor by the wiki factor (0.2, the jacket 0.3), so a cloth T-shirt and
// pants absorb a little, as in vanilla; plies and groups keep the layers
// apart: shirts conflict on the skin, not with pants or a parka;
// tribalwear conflicts with both; the jacket with the parka.
func TestLayeredClothing(t *testing.T) {
	t.Parallel()
	for k, want := range map[item.Kind]Layer{
		item.TShirt: {Set{72, 0, 36}, teeCover, skin}, item.ButtonDownShirt: {Set{72, 0, 36}, coatCover, skin},
		item.Pants: {Set{72, 0, 36}, CoverLegs, skin}, item.Tribalwear: {Set{72, 0, 36}, tribalCover, skin},
		item.Jacket: {Set{108, 0, 54}, coatCover, outer}, item.CowboyHat: {Set{72, 0, 36}, CoverUpperHead, headgear},
		item.BowlerHat: {Set{72, 0, 36}, CoverUpperHead, headgear},
	} {
		if got, ok := Apparel(k); !ok || got != want {
			t.Errorf("%v = %+v %v, want %+v", k, got, ok, want)
		}
	}
	if l, _ := Piece(item.Pants, item.Leather, 0); l.Set != (Set{162, 48, 300}) {
		t.Errorf("leather pants %+v", l.Set)
	}
	everyday := []Layer{wear(item.TShirt), wear(item.Pants)}
	cut := damage.Hit{Form: damage.FormCut, Amount: 10}
	if l := Struck(everyday, GroupOf(body.Torso), cut, rolls(0), nil); l.Outcome != Deflected {
		t.Fatalf("a T-shirt turns a weak cut on a lucky roll: %+v", l)
	}
	if l := Struck(everyday, GroupOf(body.LegL), cut, rolls(999), nil); l.Amount != 10 {
		t.Fatalf("pants rarely stop a cut: %+v", l)
	}
	if l := Struck(everyday, GroupOf(body.Brain), cut, rolls(), nil); l.Amount != 10 {
		t.Fatalf("nothing on the head: %+v", l)
	}
	conflict := func(a, b item.Kind) bool { return wear(a).Conflicts(wear(b)) }
	if !conflict(item.TShirt, item.ButtonDownShirt) || !conflict(item.Tribalwear, item.Pants) || !conflict(item.Tribalwear, item.TShirt) ||
		!conflict(item.Jacket, item.Parka) || !conflict(item.CowboyHat, item.Tuque) {
		t.Fatal("same ply, overlapping groups conflict")
	}
	if conflict(item.TShirt, item.Pants) || conflict(item.TShirt, item.Parka) || conflict(item.ButtonDownShirt, item.FlakVest) ||
		conflict(item.Pants, item.FlakPants) || conflict(item.Jacket, item.FlakVest) {
		t.Fatal("different plies or groups do not")
	}
	for k := range map[item.Kind]bool{item.TShirt: true, item.Pants: true, item.Tribalwear: true, item.Jacket: true, item.Parka: true,
		item.PlateArmor: true, item.FlakVest: true, item.FlakPants: true, item.CowboyHat: true} {
		for o := range map[item.Kind]bool{item.TShirt: true, item.ButtonDownShirt: true, item.Pants: true, item.Tribalwear: true,
			item.Jacket: true, item.Duster: true, item.MarineArmor: true, item.FlakVest: true, item.FlakPants: true, item.BowlerHat: true} {
			if k != o && conflict(k, o) != (k.Occupies()&o.Occupies() != 0) {
				t.Errorf("%v and %v: armor conflict %v, slots %b / %b", k, o, conflict(k, o), k.Occupies(), o.Occupies())
			}
		}
	}
}
