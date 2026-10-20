package weapon

import "testing"

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
