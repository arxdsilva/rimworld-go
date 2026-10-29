package kit

import (
	"math/rand/v2"
	"testing"

	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

func roller(seed uint64) func(int) int {
	r := rand.New(rand.NewPCG(seed, 1))
	return r.IntN
}

func TestPickSpendsPointsByTech(t *testing.T) {
	t.Parallel()
	seen := map[faction.Tech]map[item.Kind]int{faction.Neolithic: {}, faction.Industrial: {}}
	for seed := uint64(1); seed <= 60; seed++ {
		for tech := range seen {
			for _, k := range Pick(1<<20, 8, tech, roller(seed)) {
				seen[tech][k]++
				if !k.IsWeapon() {
					t.Fatalf("%v is no weapon", k)
				}
			}
		}
	}
	for k := range seen[faction.Neolithic] {
		if d := weapon.DefOf(weapon.ForItem(k)); d.Ranged && d.Speed > 0 && k != item.ShortBow && k != item.RecurveBow && k != item.Greatbow && k != item.Pila {
			t.Errorf("a tribe carries %v", k)
		}
		if k == item.Mace || k == item.Rifle {
			t.Errorf("a tribe carries %v", k)
		}
	}
	for _, k := range []item.Kind{item.ShortBow, item.Pila, item.Spear} {
		if seen[faction.Neolithic][k] == 0 {
			t.Errorf("tribes never carry %v", k)
		}
	}
	for _, k := range []item.Kind{item.AssaultRifle, item.FragGrenades, item.Minigun, item.Longsword, item.Rifle, item.Club, item.Knife} {
		if seen[faction.Industrial][k] == 0 {
			t.Errorf("industrial raids never carry %v", k)
		}
	}
	if seen[faction.Industrial][item.ShortBow] != 0 || seen[faction.Industrial][item.EMPGrenades] != 0 {
		t.Error("industrial raids carry guns, not bows, and no EMP yet")
	}
}

func TestPickBudget(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 30; seed++ {
		if k := Pick(Cheapest, 8, faction.Industrial, roller(seed)); len(k) != 1 {
			t.Fatalf("knife points buy one raider: %v", k)
		}
		if k := Pick(0, 8, faction.Neolithic, roller(seed)); len(k) != 1 {
			t.Fatalf("at least one raider: %v", k)
		}
		if k := Pick(1<<20, 5, faction.Industrial, roller(seed)); len(k) != 5 {
			t.Fatalf("capped: %v", k)
		}
		pts := 300
		spent := 0
		kit := Pick(pts, 24, faction.Industrial, roller(seed))
		for _, k := range kit[:len(kit)-1] {
			spent += Cost(k)
		}
		if spent > pts {
			t.Fatalf("overspent %d of %d: %v", spent, pts, kit)
		}
	}
	if Cost(item.Knife) != Cheapest || Cost(item.Rifle) != 90 || Cost(item.Club) != 60 || Cost(item.Spear) != 70 || Cost(item.Wood) != 0 {
		t.Fatal("the colony weapons keep their prices")
	}
}

func TestPickDeterministic(t *testing.T) {
	t.Parallel()
	a, b := Pick(900, 8, faction.Industrial, roller(7)), Pick(900, 8, faction.Industrial, roller(7))
	if len(a) != len(b) {
		t.Fatal("same roll, same kit")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("same roll, same kit")
		}
	}
}

func TestLegacy(t *testing.T) {
	t.Parallel()
	seen := map[item.Kind]int{}
	for _, k := range Legacy(200, roller(3)) {
		seen[k]++
	}
	if seen[item.Rifle] == 0 || seen[item.Knife] == 0 || seen[item.Spear] == 0 || len(seen) != 3 {
		t.Fatalf("legacy kit %v", seen)
	}
}
