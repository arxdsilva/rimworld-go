package item

import (
	"testing"

	"rimworld-go/internal/domain/economy/quality"
)

// TestApparelWikiInsulation (TEMP-4): a garment made from a material is
// its wiki apparel factors of the material's insulation (wiki:
// Temperature, Apparel); armor keeps its fixed numbers; quality scales.
func TestApparelWikiInsulation(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		k, stuff   Kind
		cold, heat int
	}{
		{Parka, Leather, 320, 0}, {Parka, Cloth, 360, 0}, // wiki: Parka -32 / -36 C
		{Duster, Leather, 96, 136}, {Tuque, Cloth, 90, 0}, {Tuque, 0, 80, 0},
		{SimpleHelmet, Steel, 4, 0}, {AdvancedHelmet, 0, 4, 0}, {PlateArmor, 0, 0, 0},
		{TShirt, 0, 39, 18}, {ButtonDownShirt, Cloth, 46, 18}, {Pants, Cloth, 36, 14},
		{Jacket, Cloth, 144, 54}, {Jacket, Leather, 128, 48}, {CowboyHat, Cloth, 18, 90}, {BowlerHat, Cloth, 18, 72},
		{Tribalwear, Cloth, 99, 99}, {Tribalwear, Wool, 143, 55},
		{FlakVest, 0, 10, 0}, {FlakJacket, 0, 144, 30}, {FlakPants, 0, 35, 10}, {MarineArmor, 0, 680, 100}, {MarineHelmet, 0, 40, 20},
		{WoolParka, 0, 520, 0},
	} {
		if cold, heat := Insulation(c.k, c.stuff, 0); cold != c.cold || heat != c.heat {
			t.Errorf("%s of %s: %d/%d, want %d/%d", c.k, c.stuff, cold, heat, c.cold, c.heat)
		}
	}
	if c, h := Insulation(TShirt, Cloth, quality.Stored(quality.Legendary)); c != 39*18/10 || h != 18*18/10 {
		t.Errorf("legendary T-shirt %d/%d: x1.8", c, h)
	}
	if c, _ := Insulation(TShirt, Cloth, quality.Stored(quality.Poor)); c != 39*9/10 {
		t.Errorf("poor T-shirt %d: x0.9", c)
	}
	for _, k := range Apparel() {
		if c, h := k.Insulation(); c < 0 || h < 0 {
			t.Errorf("%s moves a limit the wrong way: %d/%d", k, c, h)
		}
	}
}

// TestApparelLayers (TEMP-4): the slots, what one-pieces and shells block,
// the regions covered, hit points and the wiki market values.
func TestApparelLayers(t *testing.T) {
	t.Parallel()
	slots := map[Kind]Slot{TShirt: SkinTorso, ButtonDownShirt: SkinTorso, Tribalwear: SkinTorso, Pants: SkinLegs,
		FlakVest: Middle, FlakPants: Legs, Parka: Outer, Duster: Outer, Jacket: Outer, WoolParka: Outer, FlakJacket: Outer,
		PlateArmor: Outer, MarineArmor: Outer, Tuque: Head, CowboyHat: Head, BowlerHat: Head, SimpleHelmet: Head}
	for k, s := range slots {
		if k.Slot() != s || !k.Occupies().Has(s) || !k.IsApparel() || !HasQuality(k) {
			t.Errorf("%s: slot %s, occupies %b", k, k.Slot(), k.Occupies())
		}
	}
	if o := Tribalwear.Occupies(); !o.Has(SkinLegs) || TShirt.Occupies().Has(SkinLegs) {
		t.Errorf("tribalwear %b: a one-piece blocks the pants", o)
	}
	if o := PlateArmor.Occupies(); !o.Has(Middle) || !o.Has(Legs) || o.Has(SkinTorso) || Parka.Occupies().Has(Middle) {
		t.Errorf("plate armor %b: a shell blocks the middle layer and the flak pants", o)
	}
	if Duster.Covers() != CoverTorso|CoverLegs || Parka.Covers() != CoverTorso || Pants.Covers() != CoverLegs || CowboyHat.Covers() != CoverHead {
		t.Error("covers")
	}
	for k, hp := range map[Kind]int{TShirt: 100, Pants: 100, Jacket: 160, CowboyHat: 80, BowlerHat: 80, Tribalwear: 100} {
		if MaxHP(k) != hp {
			t.Errorf("%s hp %d, want %d", k, MaxHP(k), hp)
		}
	}
	for k, v := range map[Kind]int{TShirt: 66, ButtonDownShirt: 77, Pants: 66, Jacket: 130, CowboyHat: 44, BowlerHat: 37,
		Tribalwear: 97, Parka: 197, Duster: 205} {
		if k.Value() != v || DefaultStuff(k) == 0 && k != Parka {
			t.Errorf("%s value %d, want %d (wiki)", k, k.Value(), v)
		}
	}
	if s := (Stack{Kind: Pants, Stuff: Leather, Qty: 1}); StackValue(s) != 66*2100/1500 {
		t.Errorf("leather pants %d", StackValue(s))
	}
}
