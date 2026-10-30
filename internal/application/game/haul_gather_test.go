package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// HAUL-002: a stockpile haul gathers the nearby piles of the same load
// before it delivers.

// woodGrid is a 24x10 map with a 4x3 grid of 12-wood piles at (2..5,
// 2..4), a stockpile at (20..21, 2..3) and a hauler at (1, 1).
func woodGrid(t *testing.T) (*Game, entity.EntityID, []entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 24, Height: 10})
	var piles []entity.EntityID
	for y := 2; y <= 4; y++ {
		for x := 2; x <= 5; x++ {
			sid, err := g.SpawnStack(item.Wood, 12, x, y)
			if err != nil {
				t.Fatal(err)
			}
			piles = append(piles, sid)
		}
	}
	zoneCmd(g, command.DesignateStockpile, 20, 2, 21, 3, 0)
	id, err := g.SpawnNamedPawn(1, 1, "Hauler")
	if err != nil {
		t.Fatal(err)
	}
	return g, id, piles
}

// haulNow plans hauler id's stockpile haul at once (no tick runs).
func haulNow(t *testing.T, g *Game, id entity.EntityID) job.Record {
	t.Helper()
	pos, _ := g.positions.Get(id)
	if !g.haulToStockpile(id, pos) {
		t.Fatal("no haul started")
	}
	j, _ := g.jobs.Get(id)
	return j
}

// loadOf is a planned haul's whole load: the first pile's share (capped
// by Count) plus every pickup.
func loadOf(g *Game, j job.Record) int {
	st, _ := g.stacks.Get(j.Target)
	n := min(st.Qty, j.Count)
	for i := 1; i < len(j.Pickups); i += 2 {
		n += int(j.Pickups[i])
	}
	return n
}

func storedIn(g *Game, kind string, x0, y0, x1, y1 int) int {
	n := 0
	for _, it := range g.World().Items {
		if it.Kind == kind && !it.Carried && it.X >= x0 && it.X <= x1 && it.Y >= y0 && it.Y <= y1 {
			n += it.Qty
		}
	}
	return n
}

// TestHaulerGathersNearbyPiles: 144 wood in twelve piles of 12 goes in
// two trips, the first a full 100 (eight piles and 4 of a ninth), and all
// of it lands in the stockpile.
func TestHaulerGathersNearbyPiles(t *testing.T) {
	t.Parallel()
	g, id, _ := woodGrid(t)
	var loads []int
	carrying := 0
	for i := 0; i < 6000 && storedIn(g, "wood", 20, 2, 21, 3) < 144; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Carry != 0 {
			st, _ := g.stacks.Get(j.Carry)
			carrying = max(carrying, st.Qty)
		} else if carrying > 0 {
			loads = append(loads, carrying)
			carrying = 0
		}
	}
	if got := storedIn(g, "wood", 20, 2, 21, 3); got != 144 {
		t.Fatalf("stored %d of 144 wood", got)
	}
	if len(loads) != 2 || loads[0] != item.CarryLimit || loads[1] != 44 {
		t.Fatalf("loads %v, want [100 44]", loads)
	}
	if n := itemsOnGround(g, "wood"); n != 144 {
		t.Fatalf("wood on the ground %d: none lost, none spilled outside", n)
	}
}

// TestGatherLeavesPilesOutOfReach: a pile more than 8 tiles from the
// chain waits for the next trip.
func TestGatherLeavesPilesOutOfReach(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 8})
	near, _ := g.SpawnStack(item.Wood, 12, 2, 2)
	mid, _ := g.SpawnStack(item.Wood, 12, 9, 2)
	far, _ := g.SpawnStack(item.Wood, 12, 18, 2) // 9 from the last pile
	zoneCmd(g, command.DesignateStockpile, 27, 2, 28, 3, 0)
	id, _ := g.SpawnNamedPawn(1, 2, "Hauler")
	j := haulNow(t, g, id)
	if j.Target != near || len(j.Pickups) != 2 || j.Pickups[0] != mid {
		t.Fatalf("haul %d pickups %v, want %d then %d only", j.Target, j.Pickups, near, mid)
	}
	if _, taken := g.reserve.EntityOwner(far); taken {
		t.Fatal("the pile out of reach is left free")
	}
}

// TestGatherKeepsLoadsAlike: piles of another quality or material never
// join the load; forbidden piles and piles another pawn holds are skipped.
func TestGatherKeepsLoadsAlike(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 8})
	club := func(x int, q uint8, stuff item.Kind) entity.EntityID {
		sid, _ := g.SpawnStack(item.Club, 2, x, 2)
		st, _ := g.stacks.Get(sid)
		st.Quality, st.Stuff = q, stuff
		g.stacks.Set(sid, st)
		return sid
	}
	first := club(2, 3, item.Wood)
	same := club(3, 3, item.Wood)
	club(4, 4, item.Wood)  // better made
	club(5, 3, item.Steel) // steel
	forbid := club(6, 3, item.Wood)
	held := club(7, 3, item.Wood)
	st, _ := g.stacks.Get(forbid)
	st.Forbidden = true
	g.stacks.Set(forbid, st)
	g.reserve.TryEntity(999, held)
	zoneCmd(g, command.DesignateStockpile, 20, 2, 21, 3, 0)
	id, _ := g.SpawnNamedPawn(1, 2, "Hauler")
	j := haulNow(t, g, id)
	if j.Target != first || len(j.Pickups) != 2 || j.Pickups[0] != same {
		t.Fatalf("haul %d pickups %v, want %d then only %d", j.Target, j.Pickups, first, same)
	}
	if owner, _ := g.reserve.EntityOwner(same); owner != id {
		t.Fatal("the gathered pile is claimed for the hauler")
	}
}

// TestTwoHaulersSplitTheCluster: each pile is planned for one hauler only,
// and together they store all of it.
func TestTwoHaulersSplitTheCluster(t *testing.T) {
	t.Parallel()
	g, a, piles := woodGrid(t)
	b, _ := g.SpawnNamedPawn(1, 4, "Second")
	ja, jb := haulNow(t, g, a), haulNow(t, g, b)
	seen := map[entity.EntityID]entity.EntityID{}
	for _, p := range []struct {
		who entity.EntityID
		j   job.Record
	}{{a, ja}, {b, jb}} {
		ids := []entity.EntityID{p.j.Target}
		for i := 0; i < len(p.j.Pickups); i += 2 {
			ids = append(ids, p.j.Pickups[i])
		}
		for _, sid := range ids {
			if w, dup := seen[sid]; dup {
				t.Fatalf("pile %d planned for %d and %d", sid, w, p.who)
			}
			seen[sid] = p.who
		}
	}
	if len(seen) != len(piles) {
		t.Fatalf("%d of %d piles planned: 100 + 44 covers them all", len(seen), len(piles))
	}
	for i := 0; i < 6000 && storedIn(g, "wood", 20, 2, 21, 3) < 144; i++ {
		g.Step(10)
	}
	if got := storedIn(g, "wood", 20, 2, 21, 3); got != 144 || itemsOnGround(g, "wood") != 144 {
		t.Fatalf("stored %d of 144", got)
	}
}

// TestGatherCapsByStackLimitAndRoom: meals (limit 10) load 10; a cell
// holding 60 of a 75-limit kind takes a load of 15, which never spills.
func TestGatherCapsByStackLimitAndRoom(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 8})
	for x := 2; x <= 6; x++ {
		g.SpawnStack(item.FineMeal, 4, x, 2)
	}
	zoneCmd(g, command.DesignateStockpile, 20, 2, 21, 3, 0)
	id, _ := g.SpawnNamedPawn(1, 2, "Hauler")
	if j := haulNow(t, g, id); j.Count != 10 || loadOf(g, j) != 10 {
		t.Fatalf("meal load %d (cap %d), want 10", loadOf(g, j), j.Count)
	}

	h := New(Config{Seed: 1, Width: 24, Height: 8})
	zoneCmd(h, command.DesignateStockpile, 20, 2, 20, 2, 0)
	h.SpawnStack(item.Pemmican, 60, 20, 2)
	for x := 2; x <= 4; x++ {
		h.SpawnStack(item.Pemmican, 10, x, 2)
	}
	hid, _ := h.SpawnNamedPawn(1, 2, "Hauler")
	if j := haulNow(t, h, hid); j.Count != 15 || loadOf(h, j) != 15 {
		t.Fatalf("pemmican load %d (cap %d), want 15", loadOf(h, j), j.Count)
	}
	for range 200 {
		h.Step(10)
	}
	for _, it := range h.World().Items {
		if it.Kind != "pemmican" {
			continue
		}
		if it.Carried || it.Y != 2 || it.X > 4 && it.X != 20 {
			t.Fatalf("pemmican spilled or still carried: %+v", it)
		}
	}
	if got := storedIn(h, "pemmican", 20, 2, 20, 2); got != 75 {
		t.Fatalf("stockpile cell holds %d, want 75", got)
	}
}

// midGather steps g until hauler id carries a load with piles left.
func midGather(t *testing.T, g *Game, id entity.EntityID) job.Record {
	t.Helper()
	for range 3000 {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Carry != 0 && j.PickupI > 0 && j.PickupI < len(j.Pickups) {
			return j
		}
	}
	t.Fatal("never mid-gather")
	return job.Record{}
}

// TestDraftMidGatherDropsTheLoad: drafting a hauler mid-gather sets the
// merged load down and frees every pile it held.
func TestDraftMidGatherDropsTheLoad(t *testing.T) {
	t.Parallel()
	g, id, piles := woodGrid(t)
	j := midGather(t, g, id)
	applyNow(g, command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	g.Step(1)
	if st, ok := g.stacks.Get(j.Carry); ok && st.Carrier != 0 {
		t.Fatalf("the load is still carried: %+v", st)
	}
	for _, sid := range piles {
		if owner, taken := g.reserve.EntityOwner(sid); taken {
			t.Fatalf("pile %d still held by %d", sid, owner)
		}
	}
	if n := itemsOnGround(g, "wood"); n != 144 {
		t.Fatalf("wood on the ground %d, want 144", n)
	}
}

// TestSaveMidGatherContinues: a save mid-gather loads and plays on
// exactly as the original.
func TestSaveMidGatherContinues(t *testing.T) {
	t.Parallel()
	g, id, _ := woodGrid(t)
	j := midGather(t, g, id)
	h := saveLoad(t, g)
	hj, _ := h.jobs.Get(id)
	if hj.PickupI != j.PickupI || len(hj.Pickups) != len(j.Pickups) {
		t.Fatalf("loaded job %+v, want %+v", hj, j)
	}
	g.Step(600)
	h.Step(600)
	if g.StateHash() != h.StateHash() {
		t.Fatal("the loaded game played on differently")
	}
}

// TestForcedHaulGathers: the player's Haul order gathers too.
func TestForcedHaulGathers(t *testing.T) {
	t.Parallel()
	g, id, piles := woodGrid(t)
	applyNow(g, command.Command{Kind: command.Prioritize, Pawn: uint64(id), X: 5, Y: 4, Order: command.OrderHaul})
	j, _ := g.jobs.Get(id)
	if j.Kind != job.Haul || !j.Forced || j.Target != piles[len(piles)-1] || loadOf(g, j) != item.CarryLimit {
		t.Fatalf("forced haul %+v, want a full load from the clicked pile", j)
	}
}
