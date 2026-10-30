package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// TestEquippedWeaponKeepsQuality (CBT-3): an equipped weapon keeps its
// quality and stuff in the view, across a save and load, and when it is
// swapped for another it falls as the same stack.
func TestEquippedWeaponKeepsQuality(t *testing.T) {
	g, ada := def4Game(t, 30, 20, 10, 10)
	sid, _ := g.SpawnStack(item.Mace, 1, 11, 10)
	st, _ := g.stacks.Get(sid)
	st.Quality, st.Stuff, st.Wear = quality.Stored(quality.Masterwork), item.Silver, 7
	g.putStack(sid, st)
	g.doEquip(ada, job.Record{Kind: job.Equip, Target: sid})
	if v := def4View(g, ada); v.WeaponQuality != quality.Stored(quality.Masterwork) || v.WeaponStuff != "silver" || v.Weapon != "mace" {
		t.Fatalf("view %d %q %q", v.WeaponQuality, v.WeaponStuff, v.Weapon)
	}
	h := saveLoad(t, g)
	if v := def4View(h, ada); v.WeaponQuality != quality.Stored(quality.Masterwork) || v.WeaponStuff != "silver" {
		t.Fatalf("after load %d %q", v.WeaponQuality, v.WeaponStuff)
	}
	kid, _ := h.SpawnStack(item.Knife, 1, 10, 11)
	h.doEquip(ada, job.Record{Kind: job.Equip, Target: kid})
	found := false
	for _, id := range h.itemIDs {
		if s, ok := h.stacks.Get(id); ok && s.Kind == item.Mace {
			found = s.Quality == quality.Stored(quality.Masterwork) && s.Stuff == item.Silver && s.Wear == 7
		}
	}
	if !found || def4View(h, ada).WeaponQuality != 0 {
		t.Fatal("the mace falls with its quality, stuff and wear; the knife is plain")
	}
	if err := h.ArmQuality(ada, item.ChargeRifle, quality.Legendary); err != nil || def4View(h, ada).WeaponQuality != quality.Stored(quality.Legendary) {
		t.Fatalf("dev hook: %v", err)
	}
	if h.ArmQuality(ada, item.Steel, quality.Good) == nil {
		t.Fatal("steel is no weapon")
	}
}

// TestRaidersSpawnArmored (CBT-3): industrial raiders wear flak and
// helmets by their weapon's power, a slasher a shield belt, shown in
// their views.
func TestRaidersSpawnArmored(t *testing.T) {
	g, _ := def4Game(t, 40, 30, 20, 15)
	g.raidWith([]item.Kind{item.SniperRifle, item.Longsword}, raid.Assault)
	var dressed [][]string
	for _, v := range g.raiderViews() {
		dressed = append(dressed, v.Apparel)
	}
	has := func(name string) bool {
		for _, a := range dressed {
			if slices.Contains(a, name) {
				return true
			}
		}
		return false
	}
	if len(dressed) != 2 || !has(item.FlakJacket.String()) || !has(item.SimpleHelmet.String()) || !has(item.ShieldBelt.String()) {
		t.Fatalf("raiders wear %v", dressed)
	}
}

// TestShieldBeltWornByOrder (CBT-3): a colonist puts a shield belt on
// like any garment, keeps it through its own dressing and shows its
// charge.
func TestShieldBeltWornByOrder(t *testing.T) {
	g, ada := def4Game(t, 30, 20, 10, 10)
	sid, _ := g.SpawnStack(item.ShieldBelt, 1, 11, 10)
	g.doEquip(ada, job.Record{Kind: job.Equip, Target: sid, Forced: true})
	v := def4View(g, ada)
	if !slices.Contains(v.Apparel, item.ShieldBelt.String()) || v.ShieldPct != 100 {
		t.Fatalf("worn: %v %d", v.Apparel, v.ShieldPct)
	}
	def4Step(g, 600, 60)
	if v := def4View(g, ada); !slices.Contains(v.Apparel, item.ShieldBelt.String()) {
		t.Fatalf("still worn: %v", v.Apparel)
	}
}
