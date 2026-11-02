package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// stacksOn lists the loose stacks on tile (x, y).
func stacksOn(g *Game, x, y int) []item.Stack {
	var out []item.Stack
	for _, id := range g.itemIDs {
		if st := g.stacks.Ref(id); st != nil && st.Carrier == 0 && st.X == x && st.Y == y {
			out = append(out, *st)
		}
	}
	return out
}

// TestStockpileTileTakes1000 (STK-ST): a one-tile stockpile fills to one
// stack of 1000 wood over many hauls (a load is at most 100), the rest
// stays outside, and no load tops the tile past 1000.
func TestStockpileTileTakes1000(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	for i := range 12 {
		spawn(t, g, item.Wood, 100, 2+i%6, 4+i/6)
	}
	zoneCmd(g, command.DesignateStockpile, 12, 4, 12, 4, 0)
	id, err := g.SpawnNamedPawn(1, 1, "Hauler")
	if err != nil {
		t.Fatal(err)
	}
	onlyWork(g, id, work.Haul)
	hauls, carrying := 0, false
	for i := 0; i < 400 && hauls < 30; i++ {
		runFed(g, 1, id)
		j, _ := g.jobs.Get(id)
		if j.Carry != 0 && !carrying {
			hauls++
		}
		carrying = j.Carry != 0
		if on := stacksOn(g, 12, 4); len(on) == 1 && on[0].Qty == item.StackLimit {
			break
		}
	}
	on := stacksOn(g, 12, 4)
	if len(on) != 1 || on[0].Kind != item.Wood || on[0].Qty != item.StackLimit {
		t.Fatalf("stockpile tile holds %+v, want one stack of 1000 wood", on)
	}
	if hauls < item.StackLimit/item.CarryLimit {
		t.Fatalf("%d hauls, want at least %d", hauls, item.StackLimit/item.CarryLimit)
	}
	runFed(g, 40, id)
	if on := stacksOn(g, 12, 4); len(on) != 1 || on[0].Qty != item.StackLimit {
		t.Fatalf("a full tile took more: %+v", on)
	}
	if n := itemsOnGround(g, "wood"); n != 1200 {
		t.Fatalf("%d wood on the ground, want all 1200", n)
	}
}

// TestChunkTileHoldsOneChunk (STK-ST): chunks stay one per tile, merging
// and hauling alike: a chunk set down on a chunk spills, and a two-tile
// dump stores two of three chunks, one per tile.
func TestChunkTileHoldsOneChunk(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	spawn(t, g, item.GraniteChunk, 1, 5, 5)
	sid, err := g.SpawnStack(item.GraniteChunk, 1, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(sid)
	g.mergeOrSet(sid, st)
	if on := stacksOn(g, 5, 5); len(on) != 1 || on[0].Qty != 1 {
		t.Fatalf("chunk tile holds %+v, want one chunk", on)
	}
	if s, _ := g.stacks.Get(sid); s.Qty != 1 || s.X == 5 && s.Y == 5 {
		t.Fatalf("the second chunk %+v did not spill", s)
	}
	spawn(t, g, item.GraniteChunk, 1, 3, 8)
	zoneCmd(g, command.DesignateStockpile, 14, 4, 15, 4, StockpileVariantDump)
	id, err := g.SpawnNamedPawn(1, 1, "Hauler")
	if err != nil {
		t.Fatal(err)
	}
	onlyWork(g, id, work.Haul)
	runFed(g, 200, id)
	for _, x := range []int{14, 15} {
		if on := stacksOn(g, x, 4); len(on) != 1 || on[0].Kind != item.GraniteChunk || on[0].Qty != 1 {
			t.Fatalf("dump tile %d holds %+v, want one chunk", x, on)
		}
	}
	if n := itemsOnGround(g, item.GraniteChunk.String()); n != 3 {
		t.Fatalf("%d chunks, want 3", n)
	}
}

// TestHopperKeepsOneVanillaStack (STK-ST): a hopper is no stockpile tile;
// it holds one raw-food stack of zone.HopperStack (75), and merging into
// it spills the rest although potatoes stack to 1000 elsewhere.
func TestHopperKeepsOneVanillaStack(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	buildNow(t, g, construction.Hopper, item.Steel, construction.South, 9, 4)
	spawn(t, g, item.Potatoes, 70, 9, 4)
	sid, err := g.SpawnStack(item.Potatoes, 30, 9, 4)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(sid)
	g.mergeOrSet(sid, st)
	if on := stacksOn(g, 9, 4); len(on) != 1 || on[0].Qty != zone.HopperStack {
		t.Fatalf("hopper holds %+v, want one stack of %d", on, zone.HopperStack)
	}
	if n := itemsOnGround(g, item.Potatoes.String()); n != 100 {
		t.Fatalf("%d potatoes, want 100", n)
	}
	if cellCap(g, item.Potatoes, 9, 4) != zone.HopperStack || cellCap(g, item.Potatoes, 2, 2) != item.StackLimit ||
		cellCap(g, item.Rifle, 2, 2) != 1 {
		t.Fatal("cell caps: hopper 75, ground 1000, a rifle 1")
	}
}
