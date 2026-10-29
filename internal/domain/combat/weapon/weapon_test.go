package weapon

import (
	"testing"

	"rimworld-go/internal/domain/combat/attack"
)

func TestRangesAndAccuracy(t *testing.T) {
	t.Parallel()
	r, c := DefOf(Rifle), DefOf(Club)
	if !InRange(r, 12) || InRange(r, 13) || !InRange(c, 1) || InRange(c, 2) {
		t.Fatal("ranges")
	}
	if HitChance(r, 1) != 95 || HitChance(r, 6) != 75 || HitChance(r, 40) != MinHitChance {
		t.Fatalf("rifle falloff: %d %d %d", HitChance(r, 1), HitChance(r, 6), HitChance(r, 40))
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

func TestKnifeSpearStats(t *testing.T) {
	t.Parallel()
	k, s := DefOf(Knife), DefOf(Spear)
	if k.Name != "knife" || k.Ranged || k.Range != 1 || k.Damage != 8 || k.Cooldown != 70 || k.Accuracy != 90 {
		t.Fatalf("knife = %+v", k)
	}
	if s.Name != "spear" || s.Ranged || s.Range != 1 || s.Damage != 13 || s.Cooldown != 125 || s.Accuracy != 85 {
		t.Fatalf("spear = %+v", s)
	}
}

func TestLegacyWeaponsReproduceToday(t *testing.T) {
	t.Parallel()
	for k := Kind(0); k < numKinds; k++ {
		d := DefOf(k)
		if !d.Legacy() {
			t.Errorf("%s has bands before DEF-1", d.Name)
		}
		if d.Warmup != d.Cooldown/3 {
			t.Errorf("%s warmup %d != cooldown/3 %d", d.Name, d.Warmup, d.Cooldown/3)
		}
		if d.Burst != 1 || d.BurstTicks != 0 || d.ArmorPen != 0 {
			t.Errorf("%s burst/pen changed: %+v", d.Name, d)
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
}

func TestBandedWeaponUsesBands(t *testing.T) {
	t.Parallel()
	d := DefOf(Rifle)
	d.Bands = attack.DefaultBands(700, 800, 600, 300)
	if d.Legacy() || HitPermille(d, 12) != 800 || HitPermille(d, 1) != 700 {
		t.Fatal("bands")
	}
}
