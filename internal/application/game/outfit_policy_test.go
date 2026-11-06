package game

import (
	"bytes"
	"os"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/outfit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// GEAR-3 (OUT-001…005): apparel policies decide what auto-dress may wear.

func setOutfit(g *Game, id entity.EntityID, pol int) {
	applyNow(g, command.Command{Kind: command.SetApparelPolicy, Pawn: uint64(id), Index: pol})
}

// forceWear puts garment k on colonist id as the player's choice.
func forceWear(t *testing.T, g *Game, id entity.EntityID, k item.Kind) {
	t.Helper()
	sid, err := g.SpawnStack(k, 1, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	g.wear(id, sid, *g.stacks.Ref(sid), true)
}

// A garment the policy refuses comes off one slot per check; a forced one
// stays until the player clears it (OUT-004, OUT-005).
func TestOutfitPolicyShedsOneSlotPerCheckForcedStays(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 250) // 25 C: nobody is freezing or sweltering bare
	forceWear(t, g, id, item.Duster)
	setOutfit(g, id, outfit.Nudist)
	if v := pawnView(g, id).Gear; v.ApparelPolicy != outfit.Nudist || v.ApparelPolicyName != "Nudist" {
		t.Fatalf("view %+v", v)
	}
	if g.dressCheck(id); len(g.Wearing(id)) != 2 || wears(g, id, item.SkinTorso) != 0 {
		t.Fatalf("one check: wearing %v, want the T-shirt off", g.Wearing(id))
	}
	if st, ok := stackOf(g, item.TShirt); !ok || st.X != 2 || st.Y != 2 {
		t.Fatal("the T-shirt should be dropped at its feet")
	}
	if g.dressCheck(id); len(g.Wearing(id)) != 1 || wears(g, id, item.Body) != item.Duster {
		t.Fatalf("second check: wearing %v, want the forced duster only", g.Wearing(id))
	}
	dressRun(g, 3*clock.TicksPerHour, nil, id)
	if wears(g, id, item.Body) != item.Duster {
		t.Fatalf("the forced duster came off: %v", g.Wearing(id))
	}
	applyNow(g, command.Command{Kind: command.ClearForcedApparel, Pawn: uint64(id)})
	if g.apparel[id].Forced != 0 {
		t.Fatal("ClearForcedApparel left a forced slot")
	}
	if dressRun(g, 600, func() bool { return len(g.Wearing(id)) == 0 }, id); len(g.Wearing(id)) != 0 {
		t.Fatalf("released, the duster stays on: %v", g.Wearing(id))
	}
}

// Only garments the policy allows go on.
func TestOutfitPolicyKeepsCandidatesOut(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 20) // 2 C: a parka or a flak jacket is worth it
	g.SpawnStack(item.Parka, 1, 6, 3)
	setOutfit(g, id, outfit.Nudist)
	dressRun(g, 600, nil, id)
	if wears(g, id, item.Body) == item.Parka {
		t.Fatal("a nudist put on the parka")
	}
	setOutfit(g, id, outfit.Worker)
	if dressRun(g, 600, func() bool { return wears(g, id, item.Body) == item.Parka }, id); wears(g, id, item.Body) != item.Parka {
		t.Fatalf("a worker never put on the parka: %v", g.Wearing(id))
	}
}

// No flapping when the policy changes back and forth every half hour: no
// slot changes twice within its cooldown, yet the changes are followed.
func TestOutfitPolicyChangeNoFlap(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 250)
	var last [len(wearSlots)]uint64
	var changed [len(wearSlots)]bool
	prev, changes := g.apparel[id], 0
	for i := 0; i < 12*clock.TicksPerHour; i++ {
		if i%(clock.TicksPerHour/2) == 0 {
			pol := outfit.Nudist
			if i/(clock.TicksPerHour/2)%2 == 1 {
				pol = outfit.Anything
			}
			setOutfit(g, id, pol)
		}
		dressRun(g, 1, nil, id)
		w, now := g.apparel[id], uint64(g.clock.Now())
		for n, s := range wearSlots {
			if w.Kind(s) == prev.Kind(s) {
				continue
			}
			if changed[n] && now-last[n] < dressCool {
				t.Fatalf("tick %d: slot %v changed again %d ticks after its last change", i, s, now-last[n])
			}
			last[n], changed[n] = now, true
			changes++
		}
		prev = w
	}
	if changes < 4 {
		t.Fatalf("%d slot changes in 12 hours: the policy should still be followed once each cooldown", changes)
	}
}

// The book and each colonist's policy survive a save (v14).
func TestOutfitPoliciesSurviveSave(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 16, 8, 250)
	applyNow(g, command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyCreate, Index: outfit.Soldier, Name: "Guard"})
	applyNow(g, command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyHP, Index: 4, X: 70, Y: 100})
	applyNow(g, command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDefault, Index: outfit.Worker})
	setOutfit(g, id, 4)
	var buf bytes.Buffer
	if err := g.Save(&buf, "outfits"); err != nil {
		t.Fatal(err)
	}
	h, info, err := Load(&buf)
	if err != nil || info.Version != SaveVersion {
		t.Fatalf("load: %v, %+v", err, info)
	}
	if a, b := g.World().Gear, h.World().Gear; !reflect.DeepEqual(a, b) || len(b.Policies) != 5 || b.Policies[4].HPMin != 70 || !b.Policies[outfit.Worker].Default {
		t.Fatalf("policies after load %+v, before %+v", b, a)
	}
	if v := pawnView(h, id).Gear; v.ApparelPolicyName != "Guard" {
		t.Fatalf("after load on %q", v.ApparelPolicyName)
	}
}

// A save from before policies loads with the default book, everyone on
// Anything.
func TestOutfitPoliciesMigrateOlderSave(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(saveV12Fixture)
	if err != nil {
		t.Fatal(err)
	}
	g, info, err := Load(bytes.NewReader(data))
	if err != nil || info.Version >= 14 {
		t.Fatalf("load: %v, %+v", err, info)
	}
	pols := g.World().Gear.Policies
	if len(pols) != 4 || pols[0].Name != "Anything" || !pols[0].Default {
		t.Fatalf("policies %+v", pols)
	}
	colonists := 0
	for _, p := range g.Snapshot() {
		if !slices.Contains(g.pawnIDs, p.ID) {
			continue // a raider or a guest: no policy of its own
		}
		if colonists++; p.Gear.ApparelPolicyName != "Anything" {
			t.Fatalf("%s on %q", p.Name, p.Gear.ApparelPolicyName)
		}
	}
	if colonists == 0 {
		t.Fatal("no colonists in the fixture")
	}
}
