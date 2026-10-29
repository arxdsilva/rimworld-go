package armor

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// TestArmorCatalogue (DEF-2): every armor kind rates as the wiki has it
// (steel for the stuffed ones) and covers its body regions. wiki: Armor,
// Plate armor (steel 81/40.5/54)
func TestArmorCatalogue(t *testing.T) {
	t.Parallel()
	for k, want := range map[item.Kind]Layer{
		item.FlakVest:       {Set{1000, 360, 270}, Torso},
		item.FlakPants:      {Set{550, 80, 100}, Legs},
		item.FlakJacket:     {Set{550, 80, 100}, Torso},
		item.PlateArmor:     {Set{810, 405, 540}, Torso | Legs},
		item.MarineArmor:    {Set{1060, 450, 540}, Torso | Legs},
		item.MarineHelmet:   {Set{1060, 450, 540}, Head},
		item.SimpleHelmet:   {Set{450, 225, 300}, Head},
		item.AdvancedHelmet: {Set{630, 315, 420}, Head},
	} {
		if got, ok := Apparel(k); !ok || got != want {
			t.Errorf("%v = %+v %v, want %+v", k, got, ok, want)
		}
		if !k.IsArmor() || !k.IsApparel() {
			t.Errorf("%v is armor apparel", k)
		}
		if s, ok := Rating(k); !ok || s != want.Set {
			t.Errorf("Rating(%v) = %+v", k, s)
		}
	}
	for _, k := range item.Apparel() {
		if _, ok := Apparel(k); !ok && k.IsArmor() {
			t.Errorf("armor %v has no ratings", k)
		}
	}
	if _, ok := Rating(item.Steel); ok {
		t.Fatal("steel is no garment")
	}
}

// TestPieceStuffAndQuality: a stuffed garment rates by its material and
// every armor by its quality; a fixed one ignores the material.
func TestPieceStuffAndQuality(t *testing.T) {
	t.Parallel()
	steel, _ := Piece(item.PlateArmor, 0, 0)
	if want, _ := Apparel(item.PlateArmor); steel != want {
		t.Fatalf("no stuff, no quality: the default %+v", steel)
	}
	wood, _ := Piece(item.PlateArmor, item.Wood, 0)
	if wood.Sharp != 486 || wood.Blunt != 486 || wood.Heat != 360 { // 0.9 x wood 540/540/400
		t.Fatalf("wooden plate %+v", wood)
	}
	if l, _ := Piece(item.PlateArmor, item.GraniteBlocks, 0); l != steel {
		t.Fatal("a material that gives no armor falls back to the default")
	}
	good, _ := Piece(item.FlakVest, item.Wood, quality.Stored(quality.Legendary))
	if good.Sharp != 1800 || good.Blunt != 648 {
		t.Fatalf("legendary flak vest x1.8: %+v", good)
	}
	awful, _ := Piece(item.SimpleHelmet, 0, quality.Stored(quality.Awful))
	if awful.Sharp != 450*600/1000 {
		t.Fatalf("awful simple helmet x0.6: %+v", awful)
	}
	if l, _ := Piece(item.Parka, item.Wool, 0); l.Sharp != 72 || l.Heat != 220 {
		t.Fatalf("a wool parka of the stuffed kind rates as wool: %+v", l)
	}
	if _, ok := Piece(item.Wood, 0, 0); ok {
		t.Fatal("wood is no garment")
	}
	if got := AppendPiece(nil, item.FlakPants, 0, 0); len(got) != 1 || got[0].Covers != Legs {
		t.Fatalf("AppendPiece %+v", got)
	}
	if got := AppendPiece(nil, item.Wood, 0, 0); len(got) != 0 {
		t.Fatal("non-armor adds nothing")
	}
}

// TestPlateDeflectsKnife: over every roll, plate armor stops more of a
// knife's damage than a bare torso does.
func TestPlateDeflectsKnife(t *testing.T) {
	t.Parallel()
	plate, _ := Apparel(item.PlateArmor)
	bare, armored := 0, 0
	for r := 0; r < RollMax; r++ {
		bare += Hit(nil, Torso, damage.Sharp, 12, 180, func(int) int { return r }).Damage
		armored += Hit([]Layer{plate}, Torso, damage.Sharp, 12, 180, func(int) int { return r }).Damage
	}
	if armored*10 >= bare*6 { // A = 810-180: about half deflected or halved

		t.Fatalf("plate let %d of %d through", armored, bare)
	}
}
