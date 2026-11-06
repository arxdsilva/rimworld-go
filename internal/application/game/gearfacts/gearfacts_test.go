package gearfacts

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/stat"
)

func outfit(ks ...item.Kind) craft.Outfit {
	var o craft.Outfit
	for _, k := range ks {
		o.On[k.Slot()] = craft.Piece{Kind: k}
	}
	return o
}

// TestCapacityWiki: 75 kg × Manipulation × body size (wiki: Carrying
// capacity).
func TestCapacityWiki(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ manip, size, want int }{
		{1000, 1000, 75000}, {500, 1000, 37500}, {1000, 2000, 150000}, {800, 500, 30000}, {0, 1000, 0}, {-5, 1000, 0},
	} {
		if got := Capacity(c.manip, c.size); got != c.want {
			t.Errorf("Capacity(%d, %d) = %d, want %d", c.manip, c.size, got, c.want)
		}
	}
}

// TestGramsWiki: the wiki's masses (item.MassGrams), nothing for an empty
// hand.
func TestGramsWiki(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		k         item.Kind
		qty, want int
	}{
		{item.AssaultRifle, 1, 3500}, {item.FlakVest, 1, 6000}, {item.PlateArmor, 1, 18000}, {item.Knife, 2, 1000},
		{0, 1, 0}, {item.Rifle, 0, 0},
	} {
		if got := Grams(c.k, c.qty); got != c.want {
			t.Errorf("Grams(%v, %d) = %d, want %d", c.k, c.qty, got, c.want)
		}
	}
}

// TestMovesWiki: armor penalties add (wiki: Armor, cells/s).
func TestMovesWiki(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		o        craft.Outfit
		hundreds int
		permille int32
	}{
		{outfit(), 0, 0},
		{outfit(item.TShirt, item.Pants, item.Parka), 0, 0},
		{outfit(item.FlakVest), -12, -26},
		{outfit(item.FlakVest, item.FlakPants, item.FlakJacket, item.AdvancedHelmet), -24, -52},
		{outfit(item.MarineArmor, item.MarineHelmet), -25, -54},
		{outfit(item.PlateArmor), -80, -174},
	} {
		if got := Moves(c.o); got != c.hundreds {
			t.Errorf("%v: %d, want %d", c.o, got, c.hundreds)
		}
		if got := MovePermille(c.o); got != c.permille {
			t.Errorf("%v: %d permille, want %d", c.o, got, c.permille)
		}
	}
	if Mods(0) != nil {
		t.Error("no armor: a gear step")
	}
	if m := Mods(-174); len(m) != 1 || m[0] != (stat.Mod{Stat: stat.MoveSpeed, Add: -174}) {
		t.Errorf("plate step: %+v", m)
	}
}

// TestOf: the summary's mass, capacity, armour and speed for a flak set
// with a rifle and pack.
func TestOf(t *testing.T) {
	t.Parallel()
	o := outfit(item.FlakVest, item.FlakPants, item.TShirt)
	got := Of(o, 4200+1000, 1000, 1000)
	if want := 6000 + 3000 + 250 + 5200; got.GearGrams != want {
		t.Errorf("grams %d, want %d", got.GearGrams, want)
	}
	if got.CapacityGrams != 75000 || got.MovePct != -5 {
		t.Errorf("capacity %d move %d%%, want 75000 and -5%%", got.CapacityGrams, got.MovePct)
	}
	var layers []armor.Layer
	for _, k := range []item.Kind{item.FlakVest, item.FlakPants, item.TShirt} {
		l, _ := armor.Piece(k, 0, 0)
		layers = append(layers, l)
	}
	if want := armor.Overall(layers); got.OverallArmor != want || want.Sharp <= 490 {
		t.Errorf("armor %+v, want %+v", got.OverallArmor, want)
	}
	if bare := Of(craft.Outfit{}, 0, 500, 1000); bare.GearGrams != 0 || bare.CapacityGrams != 37500 || bare.OverallArmor != (armor.Set{}) || bare.MovePct != 0 {
		t.Errorf("bare: %+v", bare)
	}
}
