package weapon

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
)

// TestToolsTables (CBT-2): the wiki's tools, main attack first; a gun's
// stock; a human's fists, head and teeth; mechanoid blades have none.
func TestToolsTables(t *testing.T) {
	t.Parallel()
	k := ToolsOf(Knife)
	if len(k) != 3 || k[0].Name != "blade" || k[0].Damage != 12 || k[0].Form != damage.FormCut || k[0].Pen != 180 ||
		k[0].Cooldown != 90 || k[2].Form != damage.FormStab || k[1].Form != damage.FormBlunt {
		t.Fatalf("knife %+v", k)
	}
	if s := ToolsOf(Spear); s[0].Damage != 23 || s[0].Pen != 500 || s[0].Cooldown != 156 {
		t.Fatalf("spear %+v", s)
	}
	for _, wk := range []Kind{Rifle, AssaultRifle, LMG, SniperRifle, IncendiaryLauncher} {
		if ts := ToolsOf(wk); len(ts) != 2 || ts[0].Name != "stock" || ts[0].Damage != 9 || ts[0].Cooldown != 120 || !Stock(wk) {
			t.Fatalf("%v stock %+v", wk, ts)
		}
	}
	if ToolsOf(Revolver)[0].Name != "grip" || ToolsOf(ShortBow)[0].Name != "limb" || ToolsOf(Pila)[0].Name != "shaft" {
		t.Fatal("grip, limb, shaft")
	}
	if f := ToolsOf(Fists); len(f) != 3 || f[0].Damage != DefOf(Fists).Damage || f[2].Form != damage.FormBite || !Stock(FragGrenade) {
		t.Fatalf("fists %+v", f)
	}
	if ToolsOf(MechBlades) != nil || ToolsOf(SentryGun) != nil || ToolsOf(Autocannon) != nil || Stock(Knife) || Stock(SentryGun) {
		t.Fatal("single-attack and building weapons have no tools")
	}
	for _, wk := range []Kind{Club, Mace, Gladius, Longsword} {
		if len(ToolsOf(wk)) < 2 {
			t.Fatalf("%v tools", wk)
		}
	}
}

func TestPickAndNatural(t *testing.T) {
	t.Parallel()
	k := ToolsOf(Knife)
	at := func(r int) func(int) int { return func(int) int { return r } }
	if Pick(k, at(0)).Name != "blade" || Pick(k, at(749)).Name != "blade" || Pick(k, at(750)).Name != "handle" ||
		Pick(k, at(875)).Name != "point" || Pick(k, at(10_000)).Name != "point" {
		t.Fatal("knife picks by chance")
	}
	n := 0
	if Pick(ToolsOf(Pila), func(int) int { n++; return 0 }).Name != "shaft" || n != 0 {
		t.Fatal("one tool draws nothing")
	}
	if Pick([]Tool{{Name: "a"}, {Name: "b"}}, at(0)).Name != "a" {
		t.Fatal("no chances: the first")
	}
	d := DefOf(Bite)
	d.Damage = 20
	ts, c := NaturalTools(Bite, d)
	if c != 2 || ts[0].Damage != 20 || ts[0].Form != damage.FormBite || ts[1].Damage != 12 || ts[1].Form != damage.FormScratch {
		t.Fatalf("jaws and paws %+v", ts)
	}
	if _, c := NaturalTools(MechBlades, DefOf(MechBlades)); c != 1 {
		t.Fatal("blades: one attack")
	}
	m := k[0].Melee(DefOf(Knife))
	if m.Ranged || m.Range != 1 || m.Damage != 12 || m.ArmorPen != 180 || m.Name != "knife" || m.DamageType != damage.Sharp {
		t.Fatalf("melee def %+v", m)
	}
	if h := k[0].Hit("knife", 11); h.Amount != 11 || h.Form != damage.FormCut || h.Pen != 180 || h.Weapon != "knife" {
		t.Fatalf("hit %+v", h)
	}
}
