package weapon

import (
	"testing"

	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/economy/item"
)

// TestTurretGuns pins the heavy turrets to the wiki. wiki: Autocannon
// turret, Uranium slug turret
func TestTurretGuns(t *testing.T) {
	t.Parallel()
	ac := DefOf(Autocannon)
	if ac.Name != "autocannon" || ac.Damage != 27 || ac.ArmorPen != 400 || ac.Cooldown != 210 || ac.Range != 32 ||
		ac.MinRange != 8 || ac.Burst != 3 || ac.BurstTicks != 17 || ac.Bands.Touch.Acc != 280 || ac.Warmup != 0 {
		t.Fatalf("autocannon %+v", ac)
	}
	sl := DefOf(SlugTurret)
	if sl.Name != "slug turret" || sl.Damage != 55 || sl.ArmorPen != 820 || sl.Cooldown != 240 || sl.Range != 45 ||
		sl.MinRange != 11 || sl.Burst != 1 || sl.Bands.Long.Acc != 950 {
		t.Fatalf("slug turret %+v", sl)
	}
	if InRange(ac, 7) || !InRange(ac, 8) || !InRange(ac, 32) || InRange(ac, 33) || InRange(ac, 0) {
		t.Fatal("autocannon range band")
	}
	if !InRange(DefOf(Rifle), 0) {
		t.Fatal("no minimum range: point blank is in range")
	}
}

// TestMortarShells: one mortar weapon per shell, all named "mortar" (the
// sound cue), with the wiki's range, timing and blasts. wiki: Mortar
func TestMortarShells(t *testing.T) {
	t.Parallel()
	want := map[item.Kind]explosion.Def{
		item.MortarShells:     {Kind: explosion.Bomb, Radius: 2900, Damage: 50, Pen: 100},
		item.IncendiaryShells: {Kind: explosion.Flame, Radius: 2900, Damage: 10},
		item.EMPShells:        {Kind: explosion.EMP, Radius: 8900, Damage: 50},
		item.SmokeShells:      {Kind: explosion.Smoke, Radius: 7200},
	}
	for k, b := range want {
		wk, ok := ForShell(k)
		d := DefOf(wk)
		if !ok || d.Name != "mortar" || d.Blast != b || !d.Explosive() || d.MinRange != 29 || d.Range != 500 ||
			d.Warmup != 240 || d.Cooldown != 1680 || d.Speed != 683 || d.ForcedMiss != 9000 {
			t.Errorf("%v: %v %+v", k, ok, d)
		}
	}
	if _, ok := ForShell(item.Steel); ok {
		t.Fatal("steel is no shell")
	}
	if InRange(DefOf(MortarHE), 28) || !InRange(DefOf(MortarHE), 29) {
		t.Fatal("mortar minimum range")
	}
}
