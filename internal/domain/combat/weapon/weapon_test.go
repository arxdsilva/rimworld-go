package weapon

import (
	"testing"

	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
)

func TestRangesAndAccuracy(t *testing.T) {
	t.Parallel()
	r, c := DefOf(Rifle), DefOf(Club)
	if !InRange(r, 36) || InRange(r, 37) || !InRange(c, 1) || InRange(c, 2) {
		t.Fatal("ranges")
	}
	// the rifle's bands (wiki: bolt-action rifle 65/80/90/80 at 3/12/25/40)
	for dist, want := range map[int]int{1: 650, 3: 650, 12: 800, 25: 900, 36: 900 - 100*11/15} {
		if got := HitPermille(r, dist); got != want {
			t.Errorf("rifle at %d: %d want %d", dist, got, want)
		}
	}
	if HitChance(c, 1) != 85 {
		t.Fatal("melee accuracy is flat")
	}
	if Distance(0, 0, 3, -5) != 5 {
		t.Fatal("chebyshev")
	}
	if DefOf(Kind(200)).Name != "fists" {
		t.Fatal("unknown kinds fall back to fists")
	}
}

// wikiStats are the weapons DEF-1 gave the wiki's numbers: damage,
// cooldown, warmup, armor penetration, damage type, range.
var wikiStats = map[Kind]struct {
	dmg, cooldown, warmup, pen, rng int
	typ                             damage.Type
}{
	Knife: {12, 90, 0, 180, 1, damage.Sharp},
	Club:  {14, 120, 0, 210, 1, damage.Blunt},
	Spear: {23, 156, 0, 500, 1, damage.Sharp},
	Rifle: {18, 90, 102, 270, 36, damage.Sharp},
}

func TestKnifeSpearStats(t *testing.T) {
	t.Parallel()
	k, s := DefOf(Knife), DefOf(Spear)
	if k.Name != "knife" || k.Ranged || k.Range != 1 || k.Accuracy != 90 {
		t.Fatalf("knife = %+v", k)
	}
	if s.Name != "spear" || s.Ranged || s.Range != 1 || s.Accuracy != 85 {
		t.Fatalf("spear = %+v", s)
	}
	for wk, w := range wikiStats {
		d := DefOf(wk)
		if d.Damage != w.dmg || d.Cooldown != w.cooldown || d.Warmup != w.warmup || d.ArmorPen != w.pen ||
			d.Range != w.rng || d.DamageType != w.typ || d.Burst != 1 || d.BurstTicks != 0 {
			t.Errorf("%s = %+v, want %+v", d.Name, d, w)
		}
		if Penetration(d, 99) != w.pen {
			t.Errorf("%s keeps its own penetration", d.Name)
		}
	}
	if r := DefOf(Rifle); r.Legacy() || r.Speed != 1166 || r.Bands != attack.DefaultBands(650, 800, 900, 800) {
		t.Fatalf("rifle bands/speed = %+v", r)
	}
}

func TestLegacyWeaponsReproduceToday(t *testing.T) {
	t.Parallel()
	for k := Kind(0); k < numKinds; k++ {
		d := DefOf(k)
		if _, wiki := wikiStats[k]; wiki {
			continue // TestKnifeSpearStats pins the wiki weapons
		}
		if !d.Legacy() || d.Speed != 0 {
			t.Errorf("%s has bands or a projectile", d.Name)
		}
		if d.Warmup != d.Cooldown/3 {
			t.Errorf("%s warmup %d != cooldown/3 %d", d.Name, d.Warmup, d.Cooldown/3)
		}
		if d.Burst != 1 || d.BurstTicks != 0 || d.ArmorPen != 0 {
			t.Errorf("%s burst/pen changed: %+v", d.Name, d)
		}
		if Penetration(d, d.Damage) != d.Damage*15 || Penetration(d, -3) != 0 {
			t.Errorf("%s calculated penetration %d", d.Name, Penetration(d, d.Damage))
		}
		for dist := 0; dist <= 45; dist++ {
			c := d.Accuracy
			if d.Ranged && dist > 1 {
				c -= d.Falloff * (dist - 1)
			}
			if c < MinHitChance {
				c = MinHitChance
			}
			if c > 100 {
				c = 100
			}
			if HitChance(d, dist) != c || HitPermille(d, dist) != c*10 {
				t.Fatalf("%s dist %d: %d / %d want %d", d.Name, dist, HitChance(d, dist), HitPermille(d, dist), c)
			}
		}
	}
	// the legacy table, value for value (unchanged since before DEF-1)
	for k, w := range map[Kind][4]int{ // damage, cooldown, accuracy, range
		Fists: {4, 90, 80, 1}, Kick: {3, 120, 70, 1}, Bite: {9, 100, 80, 1}, Claw: {7, 110, 75, 1},
		Gore: {9, 130, 70, 1}, Headbutt: {6, 130, 75, 1}, Peck: {2, 100, 80, 1}, SentryGun: {9, 60, 75, 10},
	} {
		if d := DefOf(k); [4]int{d.Damage, d.Cooldown, d.Accuracy, d.Range} != w {
			t.Errorf("%s = %+v, want %v", d.Name, d, w)
		}
	}
}

func TestBandedWeaponUsesBands(t *testing.T) {
	t.Parallel()
	d := DefOf(Rifle)
	d.Bands = attack.DefaultBands(700, 800, 600, 300)
	if d.Legacy() || HitPermille(d, 12) != 800 || HitPermille(d, 1) != 700 {
		t.Fatal("bands")
	}
}

func TestForItem(t *testing.T) {
	t.Parallel()
	for k, want := range map[item.Kind]Kind{item.Club: Club, item.Knife: Knife, item.Spear: Spear, item.Rifle: Rifle, item.Wood: Fists} {
		if ForItem(k) != want {
			t.Errorf("ForItem(%v) = %v", k, ForItem(k))
		}
	}
}
