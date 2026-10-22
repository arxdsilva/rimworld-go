package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// hauler spawns a colonist that hauls but never builds, so supplied frames
// stay frames for the checks.
func hauler(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, "")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	p := work.DefaultPriorities().Set(work.Construct, 0)
	g.prio[id] = p
	g.labor.Set(id, p.Mask())
	return id
}

func woodWalls(g *Game, x0, y, n int) []entity.EntityID {
	for x := x0; x < x0+n; x++ {
		g.placeFrameStuff(construction.Wall, item.Wood, construction.South, x, y)
	}
	return append([]entity.EntityID(nil), g.frameIDs...)
}

// woodSplit is the wood stored in stockpiles, lying loose elsewhere, in
// frames and carried.
func woodSplit(g *Game) (stored, loose, framed, carried int) {
	for _, iid := range g.itemIDs {
		st, ok := g.stacks.Get(iid)
		if !ok || st.Kind != item.Wood {
			continue
		}
		switch {
		case st.Carrier != 0:
			carried += st.Qty
		case g.inMatchingStockpile(st):
			stored += st.Qty
		default:
			loose += st.Qty
		}
	}
	for _, fid := range g.frameIDs {
		fr, _ := g.frames.Get(fid)
		framed += fr.Wood
	}
	return
}

// trips steps until every frame is supplied, counting pickups (a carry
// starting) and the biggest load seen.
func supplyAll(t *testing.T, g *Game, id entity.EntityID, max int) (trips, biggest int) {
	t.Helper()
	carrying := false
	for i := 0; i < max; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Carry != 0 && !carrying {
			trips++
		}
		carrying = j.Carry != 0
		if st, ok := g.stacks.Get(j.Carry); ok && j.Carry != 0 && st.Qty > biggest {
			biggest = st.Qty
		}
		if !carrying && allSupplied(g) {
			return trips, biggest
		}
	}
	t.Fatalf("frames not supplied after %d ticks", max)
	return
}

func allSupplied(g *Game) bool {
	for _, fid := range g.frameIDs {
		if fr, _ := g.frames.Get(fid); !fr.Ready() {
			return false
		}
	}
	return true
}

func TestDeliveryFeedsManyFramesAndStoresTheRest(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 2, Y: 2, X2: 4, Y2: 4})
	g.SpawnStack(item.Wood, 300, 3, 3)
	id := hauler(t, g, 6, 6)
	woodWalls(g, 10, 7, 6)
	trips, biggest := supplyAll(t, g, id, 4000)
	if trips != 1 || biggest != item.CarryLimit {
		t.Fatalf("trips=%d biggest load=%d: one full load should feed all six walls", trips, biggest)
	}
	for _, fid := range g.frameIDs {
		if fr, _ := g.frames.Get(fid); fr.Wood != fr.Need {
			t.Fatalf("frame at (%d,%d) has %d/%d", fr.X, fr.Y, fr.Wood, fr.Need)
		}
	}
	stored, loose, framed, carried := woodSplit(g)
	if stored != 288 || loose != 0 || framed != 12 || carried != 0 {
		t.Fatalf("stored=%d loose=%d framed=%d carried=%d: the 88 left over go back to storage", stored, loose, framed, carried)
	}
	if len(g.haul.incoming) != 0 || len(g.haul.deliveries) != 0 {
		t.Fatalf("delivery bookkeeping left over: %v %v", g.haul.incoming, g.haul.deliveries)
	}
}

func TestDeliveryWithoutStorageCarriesJustTheNeed(t *testing.T) {
	for _, tc := range []struct {
		name string
		full bool // a stockpile with no room left
	}{{"no stockpile", false}, {"full stockpile", true}} {
		t.Run(tc.name, func(t *testing.T) {
			g := New(Config{Seed: 1, Width: 30, Height: 12})
			if tc.full {
				g.Apply(command.Command{Kind: command.DesignateStockpile, X: 20, Y: 2, X2: 20, Y2: 2})
				g.SpawnStack(item.Wood, item.StackLimit, 20, 2)
			}
			g.SpawnStack(item.Wood, 300, 3, 3)
			id := hauler(t, g, 6, 6)
			woodWalls(g, 10, 7, 6)
			trips, biggest := supplyAll(t, g, id, 4000)
			if trips != 1 || biggest != 12 {
				t.Fatalf("trips=%d load=%d, want one trip of exactly 12", trips, biggest)
			}
			_, loose, framed, _ := woodSplit(g)
			if loose != 288 || framed != 12 {
				t.Fatalf("loose=%d framed=%d", loose, framed)
			}
		})
	}
}

func TestDeliveryRouteLimits(t *testing.T) {
	g := New(Config{Seed: 1, Width: 60, Height: 12})
	g.SpawnStack(item.Wood, 300, 3, 3)
	id := hauler(t, g, 3, 4)
	woodWalls(g, 6, 7, 10) // ten adjacent walls: only eight per load
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 15+routeReach+1, 7)
	pos, _ := g.positions.Get(id)
	rec, shares, ok := g.planDelivery(id, pos)
	if !ok || len(rec.Route) != routeFrames || len(shares) != routeFrames || rec.Count != 2*routeFrames {
		t.Fatalf("route %v shares %v count %d", rec.Route, shares, rec.Count)
	}
	// a lone frame too far from the rest is a route of its own
	g2 := New(Config{Seed: 1, Width: 60, Height: 12})
	g2.SpawnStack(item.Wood, 300, 3, 3)
	id2 := hauler(t, g2, 3, 4)
	g2.placeFrameStuff(construction.Wall, item.Wood, construction.South, 6, 7)
	g2.placeFrameStuff(construction.Wall, item.Wood, construction.South, 6+routeReach+1, 7)
	pos2, _ := g2.positions.Get(id2)
	rec2, _, ok := g2.planDelivery(id2, pos2)
	if !ok || len(rec2.Route) != 1 || rec2.Count != 2 {
		t.Fatalf("far frame chained: route %v count %d", rec2.Route, rec2.Count)
	}
	// the stone wall is never on a wood route
	g2.placeFrameStuff(construction.Wall, item.GraniteBlocks, construction.South, 7, 7)
	rec3, _, _ := g2.planDelivery(id2, pos2)
	if len(rec3.Route) != 1 {
		t.Fatalf("stone frame on a wood route: %v", rec3.Route)
	}
}

func TestTwoHaulersNeverOverDeliver(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	g.SpawnStack(item.Wood, 50, 3, 3)
	hauler(t, g, 2, 5)
	hauler(t, g, 4, 5)
	woodWalls(g, 12, 6, 1)
	for i := 0; i < 2000; i++ {
		g.Step(1)
		fr, _ := g.frames.Get(g.frameIDs[0])
		if fr.Wood > fr.Need {
			t.Fatalf("over-delivered: %d/%d", fr.Wood, fr.Need)
		}
	}
	fr, _ := g.frames.Get(g.frameIDs[0])
	stored, loose, framed, carried := woodSplit(g)
	if fr.Wood != fr.Need || framed != 2 || loose != 48 || stored != 0 || carried != 0 {
		t.Fatalf("frame %d/%d loose=%d carried=%d", fr.Wood, fr.Need, loose, carried)
	}
	if n := len(g.World().Items); n != 1 {
		t.Fatalf("%d piles: no stray leftovers expected", n)
	}
}

// midRoute steps until hauler id has fed at least one frame and still
// carries the rest.
func midRoute(t *testing.T, g *Game, id entity.EntityID) job.Record {
	t.Helper()
	for i := 0; i < 4000; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Carry != 0 && j.RouteI >= 1 && j.RouteI < len(j.Route) {
			return j
		}
	}
	t.Fatal("hauler never got mid-route")
	return job.Record{}
}

func TestDraftedHaulerDropsLoadAndFreesFrames(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.SpawnStack(item.Wood, 300, 3, 3)
	id := hauler(t, g, 6, 6)
	woodWalls(g, 10, 7, 6)
	midRoute(t, g, id)
	if len(g.haul.incoming) == 0 {
		t.Fatal("frames ahead on the route should be booked")
	}
	g.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	g.ApplyPending()
	if len(g.haul.incoming) != 0 || len(g.haul.deliveries) != 0 {
		t.Fatalf("drafting kept the bookings: %v %v", g.haul.incoming, g.haul.deliveries)
	}
	pos, _ := g.positions.Get(id)
	if _, _, _, carried := woodSplit(g); carried != 0 || g.itemAt(pos.X, pos.Y) == 0 {
		t.Fatal("the load should be dropped where the hauler stands")
	}
	other := hauler(t, g, 6, 8)
	if _, _, ok := g.planDelivery(other, Position{X: 6, Y: 8}); !ok {
		t.Fatal("another hauler should take over the frames")
	}
	supplyAll(t, g, other, 4000)
	_, loose, framed, _ := woodSplit(g)
	if framed != 12 || loose != 288 {
		t.Fatalf("framed=%d loose=%d", framed, loose)
	}
}

func TestDeadHaulerFreesFrames(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.SpawnStack(item.Wood, 300, 3, 3)
	id := hauler(t, g, 6, 6)
	woodWalls(g, 10, 7, 6)
	midRoute(t, g, id)
	g.die(id, health.CauseInjury)
	if len(g.haul.incoming) != 0 || len(g.haul.deliveries) != 0 {
		t.Fatalf("death kept the bookings: %v %v", g.haul.incoming, g.haul.deliveries)
	}
	g.Step(1)
	if _, _, _, carried := woodSplit(g); carried != 0 {
		t.Fatal("a dead hauler drops its load")
	}
}

func TestRefuelLeftoverGoesToStockpile(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	if !g.BuildNow("campfire", "wood", "south", 7, 3) {
		t.Fatal("build campfire")
	}
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 1, Y: 5, X2: 3, Y2: 6})
	g.SpawnStack(item.Wood, 300, 2, 5)
	biggest := 0
	for i := 0; i < 1500; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Kind == job.Refuel && j.Carry != 0 {
			if st, _ := g.stacks.Get(j.Carry); st.Qty > biggest {
				biggest = st.Qty
			}
		}
	}
	bid := g.layerBldgAt(7, 3, construction.LayerObject)
	if g.fuel[bid] < (CampfireCapacity-1)*TicksPerWood {
		t.Fatalf("campfire fuel = %d after refuelling", g.fuel[bid])
	}
	stored, loose, _, carried := woodSplit(g)
	if biggest != item.CarryLimit || loose != 0 || carried != 0 || stored != 300-CampfireCapacity {
		t.Fatalf("load=%d stored=%d loose=%d carried=%d", biggest, stored, loose, carried)
	}
}

func TestSupplyOrderChainsFrames(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.SpawnStack(item.Wood, 300, 3, 3)
	id := hauler(t, g, 6, 6)
	g.prio[id] = work.Priorities{} // idle until ordered
	g.labor.Set(id, work.Mask(0).Set(work.Haul, true))
	woodWalls(g, 10, 7, 4)
	g.Apply(command.Command{Kind: command.Prioritize, Pawn: uint64(id), Order: command.OrderSupply, X: 12, Y: 7})
	g.ApplyPending()
	j, _ := g.jobs.Get(id)
	if j.Kind != job.Haul || !j.Forced || len(j.Route) != 4 || j.Route[0] != g.layerFrameAt(12, 7, construction.LayerObject) || j.Count != 8 {
		t.Fatalf("supply job %+v", j)
	}
	supplyAll(t, g, id, 4000)
}

func TestSaveMidRouteContinuesIdentically(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 2, Y: 2, X2: 4, Y2: 4})
	g.SpawnStack(item.Wood, 300, 3, 3)
	id := hauler(t, g, 6, 6)
	hauler(t, g, 8, 9)
	woodWalls(g, 10, 7, 6)
	woodWalls(g, 10, 9, 3)
	midRoute(t, g, id)
	var buf bytes.Buffer
	if err := g.Save(&buf, "route"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.haul.deliveries) == 0 || len(h.haul.incoming) != len(g.haul.incoming) {
		t.Fatalf("bookings lost: %v vs %v", h.haul.incoming, g.haul.incoming)
	}
	sameState(t, "after load", g, h)
	for i := 0; i < 20; i++ {
		g.Step(50)
		h.Step(50)
		sameState(t, "running on", g, h)
	}
}

// tripEnd steps until hauler id puts its load down.
func tripEnd(t *testing.T, g *Game, id entity.EntityID) {
	t.Helper()
	for i := 0; i < 4000; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.Carry == 0 {
			return
		}
	}
	t.Fatal("hauler never put its load down")
}

func TestLeftoverFindsAnotherStockpileWhenTheSpareGoes(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 2, Y: 9, X2: 3, Y2: 10})
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 20, Y: 7, X2: 21, Y2: 8})
	g.SpawnStack(item.Wood, 300, 3, 3) // loose, outside storage
	id := hauler(t, g, 6, 6)
	woodWalls(g, 10, 7, 6)
	j := midRoute(t, g, id)
	if !j.HasSpare || j.SpareX < 20 {
		t.Fatalf("spare (%d,%d) %v: the stockpile nearest the last wall", j.SpareX, j.SpareY, j.HasSpare)
	}
	g.zones.Clear(zone.Rect{MinX: 20, MinY: 7, MaxX: 21, MaxY: 8})
	tripEnd(t, g, id)
	stored, loose, framed, _ := woodSplit(g)
	if stored != 88 || loose != 200 || framed != 12 {
		t.Fatalf("stored=%d loose=%d framed=%d: the rest goes to the other stockpile", stored, loose, framed)
	}
}

func TestRouteSkipsFramesFilledMeanwhile(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	g.SpawnStack(item.Wood, 300, 3, 3)
	id := hauler(t, g, 6, 6)
	woodWalls(g, 10, 7, 6)
	j := midRoute(t, g, id)
	for _, fid := range j.Route[j.RouteI:] {
		fr, _ := g.frames.Get(fid)
		fr.Wood = fr.Need // supplied by someone else
		g.frames.Set(fid, fr)
	}
	tripEnd(t, g, id)
	stored, loose, framed, carried := woodSplit(g)
	if framed != 12 || stored != 0 || carried != 0 || loose+2*j.RouteI != 300 {
		t.Fatalf("framed=%d loose=%d carried=%d after %d fed: the rest is dropped (no storage)", framed, loose, carried, j.RouteI)
	}
	if len(g.haul.incoming) != 0 || len(g.haul.deliveries) != 0 {
		t.Fatalf("bookings left: %v %v", g.haul.incoming, g.haul.deliveries)
	}
}
