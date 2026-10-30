package kit

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

// TestArmorByTechAndPower: tribes go bare but the chief; industrial
// raiders wear more flak the dearer they are, a slasher a shield belt;
// spacers dress as troopers; at most one piece per slot; the seed picks
// the optional pieces the same way every time. wiki: Raider
func TestArmorByTechAndPower(t *testing.T) {
	t.Parallel()
	if a := Armor(item.Spear, faction.Neolithic, 1); a != nil {
		t.Fatalf("a tribal spearman wears %v", a)
	}
	if a := Armor(item.Greatbow, faction.Neolithic, 1); !slices.Equal(a, []item.Kind{item.PlateArmor}) {
		t.Fatalf("the chief wears %v", a)
	}
	if a := Armor(item.Knife, faction.Industrial, 1); a != nil {
		t.Fatalf("a knife raider wears %v", a)
	}
	if a := Armor(item.SniperRifle, faction.Industrial, 1); !slices.Equal(a, []item.Kind{item.FlakJacket, item.SimpleHelmet}) {
		t.Fatalf("a sniper wears %v", a)
	}
	if a := Armor(item.Longsword, faction.Industrial, 1); !slices.Contains(a, item.ShieldBelt) {
		t.Fatalf("a slasher wears %v", a)
	}
	if a := Armor(item.AssaultRifle, faction.Spacer, 1); !slices.Contains(a, item.MarineArmor) {
		t.Fatalf("a spacer heavy wears %v", a)
	}
	if a := Armor(item.Revolver, faction.Spacer, 1); !slices.Contains(a, item.FlakPants) {
		t.Fatalf("a trooper wears %v", a)
	}
	vests, helmets, jackets := 0, 0, 0
	for seed := uint64(0); seed < 400; seed++ {
		for _, w := range []item.Kind{item.Revolver, item.Rifle, item.Minigun, item.Gladius, item.ShortBow} {
			a := Armor(w, faction.Industrial, seed)
			if !slices.Equal(a, Armor(w, faction.Industrial, seed)) {
				t.Fatal("the same seed dresses the same")
			}
			slots := map[item.Slot]bool{}
			for _, k := range a {
				if !k.IsApparel() || slots[k.Slot()] {
					t.Fatalf("%v: %v (two in a slot, or no apparel)", w, a)
				}
				slots[k.Slot()] = true
				switch k {
				case item.FlakVest:
					vests++
				case item.FlakJacket:
					jackets++
				case item.SimpleHelmet:
					helmets++
				}
			}
		}
	}
	if vests == 0 || jackets == 0 || helmets == 0 {
		t.Fatalf("optional pieces never picked: %d vests, %d jackets, %d helmets", vests, jackets, helmets)
	}
}

func TestTechOfAndAllies(t *testing.T) {
	t.Parallel()
	if TechOf(nil) != faction.Industrial || TechOf(&faction.Faction{Type: faction.Clans}) != faction.Neolithic {
		t.Fatal("tech of")
	}
	if Ally(faction.Neolithic, 0) != item.Spear || Ally(faction.Neolithic, 1) != item.Club ||
		Ally(faction.Industrial, 0) != item.Rifle || Ally(faction.Industrial, 2) != item.Knife {
		t.Fatal("allies' weapons")
	}
	if Casket(0) != item.Club || Casket(4) != item.Knife || Casket(-1) != item.Spear {
		t.Fatal("casket blades")
	}
}
