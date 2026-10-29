package game

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
)

func TestDryingRackDriesMeat(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	jumpTo(g, 15, 6) // warm summer morning
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	if !g.BuildNow("drying rack", "wood", "south", 6, 3) {
		t.Fatal("could not build a drying rack")
	}
	if _, err := g.SpawnStack(item.Meat, 30, 3, 6); err != nil {
		t.Fatal(err)
	}
	dried, onRack := 0, false
	for i := 0; i < 48 && dried == 0; i++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(id, needs.Full, needs.Full)
		for _, b := range g.World().Buildings {
			if b.DryQty > 0 {
				onRack = true
			}
		}
		for _, iid := range g.itemIDs {
			if st, ok := g.stacks.Get(iid); ok && st.Dried {
				dried += st.Qty
			}
		}
	}
	if !onRack {
		t.Fatal("meat never went onto the rack")
	}
	if dried != 25 {
		t.Fatalf("dried = %d, want one full rack (25)", dried)
	}
	// dried meat keeps through days of summer heat (the colonist kept fed
	// and rested: a starving loner binges, which isn't what this tests)
	for h := 0; h < 5*24; h++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(id, needs.Full, needs.Full)
	}
	n := 0
	for _, iid := range g.itemIDs {
		if st, ok := g.stacks.Get(iid); ok && st.Kind == item.Meat && st.Dried {
			n += st.Qty
		}
	}
	if n == 0 {
		t.Fatal("dried meat should never rot")
	}
}

func TestDeconstructedRackReturnsBatch(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	if !g.BuildNow("drying rack", "wood", "south", 6, 3) {
		t.Fatal("could not build a drying rack")
	}
	bid := g.bldgIDs[len(g.bldgIDs)-1]
	g.craft.Drying.Set(bid, craft.Batch{Kind: item.Berries, Qty: 10, Left: item.Berries.DryTicks()})
	g.removeBuilding(bid)
	st, ok := stackOf(g, item.Berries)
	if !ok || st.Qty != 10 || st.Dried {
		t.Fatalf("batch not returned undried: %+v %v", st, ok)
	}
}
