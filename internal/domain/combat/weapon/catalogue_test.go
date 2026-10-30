package weapon

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/economy/item"
)

// TestCatalogueComplete (DEF-2): every catalogue weapon carries a full
// wiki profile: a name, damage, penetration, cooldown and range; guns and
// bows warmup, accuracy bands and a projectile; explosives a blast, a
// forced-miss radius and a projectile, and no bands.
func TestCatalogueComplete(t *testing.T) {
	t.Parallel()
	names := map[string]bool{}
	for k := Mace; k <= lastCatalogue; k++ {
		d := DefOf(k)
		if d.Name == "" || names[d.Name] || d.Damage <= 0 || d.ArmorPen < 0 || d.Cooldown <= 0 || d.Range < 1 || d.Burst < 1 {
			t.Errorf("%d %q incomplete: %+v", k, d.Name, d)
		}
		names[d.Name] = true
		switch {
		case !d.Ranged:
			if d.Range != 1 || d.Warmup != 0 || d.Speed != 0 || d.Explosive() || d.ArmorPen == 0 {
				t.Errorf("melee %s: %+v", d.Name, d)
			}
		case d.Explosive():
			if !d.Legacy() || d.Speed <= 0 || d.ForcedMiss != 1900 || d.Blast.Radius <= 0 || d.DamageType != d.Blast.Type() || d.Warmup <= 0 {
				t.Errorf("explosive %s: %+v", d.Name, d)
			}
		default:
			if d.Legacy() || d.Speed <= 0 || d.Warmup <= 0 || d.ArmorPen <= 0 || d.DamageType != damage.Sharp {
				t.Errorf("gun %s: %+v", d.Name, d)
			}
			if (d.Burst > 1) != (d.BurstTicks > 0) {
				t.Errorf("%s burst %d gap %d", d.Name, d.Burst, d.BurstTicks)
			}
		}
	}
}

// TestCatalogueWikiSamples pins a few rows value for value. wiki: Weapons
func TestCatalogueWikiSamples(t *testing.T) {
	t.Parallel()
	ar := DefOf(AssaultRifle)
	if ar.Damage != 11 || ar.ArmorPen != 160 || ar.Warmup != 60 || ar.Cooldown != 102 || ar.Range != 30 ||
		ar.Burst != 3 || ar.BurstTicks != 10 || ar.Bands.Short.Acc != 700 || ar.Speed != 1166 {
		t.Fatalf("assault rifle %+v", ar)
	}
	mg := DefOf(Minigun)
	if mg.Burst != 25 || mg.BurstTicks != 5 || mg.Bands.Long.Acc != 180 {
		t.Fatalf("minigun %+v", mg)
	}
	frag := DefOf(FragGrenade)
	if frag.Blast != (explosion.Def{Kind: explosion.Bomb, Radius: 1900, Damage: 50, Pen: 100}) || frag.Range != 12 || frag.Speed != 200 {
		t.Fatalf("frag %+v", frag)
	}
	if b := DefOf(EMPGrenade).Blast; b.Kind != explosion.EMP || b.Radius != 3500 {
		t.Fatalf("EMP grenade %+v", b)
	}
	if b := DefOf(Molotov).Blast; b.Kind != explosion.Flame || b.Radius != 1100 || DefOf(Molotov).DamageType != damage.Heat {
		t.Fatalf("molotov %+v", b)
	}
	if ls := DefOf(Longsword); ls.Damage != 23 || ls.ArmorPen != 340 || ls.Cooldown != 156 {
		t.Fatalf("longsword %+v", ls)
	}
}

// TestForItemCatalogue: every weapon item maps to its own weapon (the two
// enums share the catalogue's order), and every weapon of the catalogue
// has an item.
func TestForItemCatalogue(t *testing.T) {
	t.Parallel()
	seen := map[Kind]bool{}
	for k := item.Mace; k <= item.EMPLauncher; k++ {
		if !k.IsWeapon() {
			t.Fatalf("%v is no weapon item", k)
		}
		w := ForItem(k)
		if w < Mace || seen[w] {
			t.Fatalf("%v -> %v", k, w)
		}
		seen[w] = true
	}
	if len(seen) != int(lastCatalogue-Mace+1) {
		t.Fatalf("%d items for %d catalogue weapons", len(seen), lastCatalogue-Mace+1)
	}
	for _, c := range []struct {
		k    item.Kind
		name string
	}{{item.AssaultRifle, "assault rifle"}, {item.FragGrenades, "frag grenade"}, {item.HeavySMG, "heavy smg"},
		{item.MolotovCocktails, "molotov cocktail"}, {item.EMPLauncher, "emp launcher"}, {item.Mace, "mace"}} {
		if n := DefOf(ForItem(c.k)).Name; n != c.name {
			t.Errorf("%v wields as %q, want %q", c.k, n, c.name)
		}
	}
	if ForItem(item.FlakVest) != Fists {
		t.Fatal("armor is no weapon")
	}
}

func TestStats(t *testing.T) {
	t.Parallel()
	v := Stats(AssaultRifle)
	if v.Name != "assault rifle" || v.Damage != 11 || v.ArmorPen != 160 || v.Burst != 3 || v.Range != 30 ||
		v.Accuracy != [4]int{600, 700, 650, 550} || v.Melee || v.Blast != "" {
		t.Fatalf("assault rifle view %+v", v)
	}
	if v := Stats(Club); !v.Melee || v.Accuracy != [4]int{} || v.ArmorPen != 210 || v.Burst != 1 {
		t.Fatalf("club view %+v", v)
	}
	if v := Stats(FragGrenade); v.Blast != "bomb" || v.BlastRadius != 19 || v.Accuracy != [4]int{} {
		t.Fatalf("frag view %+v", v)
	}
	if v := Stats(Fists); v.Accuracy != [4]int{} || v.ArmorPen != 4*PenPerDamage {
		t.Fatalf("fists view %+v", v)
	}
	if v := Stats(Autocannon); v.MinRange != 8 || v.Range != 32 || v.SingleUse {
		t.Fatalf("autocannon view %+v", v)
	}
}
