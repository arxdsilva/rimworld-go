package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
)

// TestDeadColonistsSpareFollowsTheHomeRule: the spare weapon a colonist
// carried falls under the same rule as the one it wielded: forbidden outside
// the home area, allowed inside it.
func TestDeadColonistsSpareFollowsTheHomeRule(t *testing.T) {
	t.Parallel()
	for _, home := range []bool{false, true} {
		g, _ := raidGame(t)
		p := g.pawnIDs[0]
		g.paintHome(10, 5, 20, 15, home) // the colonist's at (15, 10)
		g.equip[p] = item.Rifle
		g.combat.Drafts.DropWeapon(p, true) // stows it
		if k, _, ok := g.combat.Drafts.Spare(p); !ok || k != item.Rifle {
			t.Fatalf("no spare: %v %v", k, ok)
		}
		g.die(p, health.CauseInjury)
		st, ok := stackOf(g, item.Rifle)
		if !ok || st.Forbidden == home {
			t.Fatalf("in Home %v the spare fell as %+v (%v)", home, st, ok)
		}
	}
}

// TestShovedStackStaysTheSame: a stack pushed off a footprint is the same
// stack on another tile: nothing but its place changes.
func TestShovedStackStaysTheSame(t *testing.T) {
	t.Parallel()
	g, _ := raidGame(t)
	const x, y = 8, 8
	id, err := g.SpawnStack(item.Rifle, 1, x, y)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(id)
	st.Forbidden, st.Unclaimed, st.HaulMarked = true, true, true
	st.Of, st.Animal, st.Yield, st.DiedAt, st.Rot = 77, 3, 40, 900, 5
	st.Quality, st.Stuff, st.Wear = 4, item.Steel, 12
	g.stacks.Set(id, st)

	g.clearItemsFrom(x, y, [][2]int{{x, y}})
	if g.itemAt(x, y) != 0 {
		t.Fatal("the stack stayed on the footprint")
	}
	var got item.Stack
	found := false
	for _, sid := range g.itemIDs {
		if s, _ := g.stacks.Get(sid); s.Kind == item.Rifle {
			got, found = s, true
		}
	}
	if !found {
		t.Fatal("the stack is gone")
	}
	got.X, got.Y = x, y
	if !reflect.DeepEqual(got, st) {
		t.Fatalf("shoved %+v, was %+v", got, st)
	}
}

// TestSettlementLootPacksUp: a settlement's goods lie forbidden, so nothing
// goes for them unasked, and the pack-up still takes them home.
func TestSettlementLootPacksUp(t *testing.T) {
	t.Parallel()
	g, m, _, _, _ := besieged(t, faction.Settlers)
	silver := 0
	for _, id := range m.itemIDs {
		if st, ok := m.stacks.Get(id); ok && st.Kind == item.Silver {
			if !st.Forbidden {
				t.Fatalf("settlement loot lies allowed: %+v", st)
			}
			silver += st.Qty
		}
	}
	if silver == 0 {
		t.Fatal("the settlement has no silver")
	}
	fell(g, defenders(m)...)
	nextHours(g, 4)
	meta := settleMeta(g, m)
	if meta.PackBy == 0 {
		t.Fatalf("the settlement did not fall: %+v", meta)
	}
	meta.PackBy = uint64(g.clock.Now()) + 1
	nextHours(g, 1)
	if len(g.maps) != 1 {
		t.Fatalf("the map did not close: %d maps", len(g.maps))
	}
	packed := 0
	for _, h := range g.worldSt.ax9.Export().Cargo {
		for _, st := range h.Hold.Items {
			if st.Kind == item.Silver {
				if st.Forbidden {
					t.Fatalf("packed forbidden: %+v", st)
				}
				packed += st.Qty
			}
		}
	}
	if packed < silver {
		t.Fatalf("packed %d silver of the %d on the map", packed, silver)
	}
}
