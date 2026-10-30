package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Frame inspector read model (UI-015) and blueprint cancelling.

func frameView(g *Game, id entity.EntityID) (FrameView, bool) {
	for _, f := range g.World().Frames {
		if f.ID == id {
			return f, true
		}
	}
	return FrameView{}, false
}

func TestFrameViewMaterialsAndWork(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	onlyWork(g, ada, work.Haul, work.Construct)
	g.SpawnStack(item.Wood, 20, 2, 5)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 10, 4)
	fid := g.layerFrameAt(10, 4, construction.LayerObject)
	fv, ok := frameView(g, fid)
	want := construction.WorkFor(construction.Wall, item.Wood)
	if !ok || fv.Wood != 0 || fv.Need != construction.WoodCost(construction.Wall) || fv.WorkTotal != want || fv.WorkLeft != want {
		t.Fatalf("fresh frame view %+v", fv)
	}
	if fv.InStock != 20 || fv.Incoming != 0 || fv.Status != frameNoHauler {
		t.Fatalf("stock %d, incoming %d, status %q", fv.InStock, fv.Incoming, fv.Status)
	}
	sawIncoming, sawHauler, sawBuilder, sawWork := false, false, false, false
	for i := 0; i < 2000; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(1)
		fv, ok := frameView(g, fid)
		if !ok {
			break
		}
		if fv.Incoming > 0 {
			sawIncoming = true
			if fv.Status != frameOnTheWay {
				t.Fatalf("status while hauling %q", fv.Status)
			}
		}
		if len(fv.Haulers) == 1 && strings.HasPrefix(fv.Haulers[0], "Ada (") && strings.HasSuffix(fv.Haulers[0], " wood)") {
			sawHauler = true
		}
		if fv.Builder == "Ada" {
			sawBuilder = true
			if fv.Status != "Being built by Ada" {
				t.Fatalf("status while building %q", fv.Status)
			}
		}
		if fv.WorkLeft < fv.WorkTotal {
			sawWork = true
			if fv.Wood != fv.Need {
				t.Fatalf("work before materials: %+v", fv)
			}
		}
	}
	if !sawIncoming || !sawHauler || !sawBuilder || !sawWork {
		t.Fatalf("incoming %v hauler %v builder %v work %v", sawIncoming, sawHauler, sawBuilder, sawWork)
	}
	if g.layerBldgAt(10, 4, construction.LayerObject) == 0 {
		t.Fatal("the wall was built")
	}
}

func TestFrameStatusLines(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	onlyWork(g, ada) // stands still: the test sets every job
	status := func(fid entity.EntityID) string {
		fv, _ := frameView(g, fid)
		return fv.Status
	}
	// 1 a reinstall
	placeBuilt(t, g, construction.Bed, construction.South, 2, 6)
	bed := g.layerBldgAt(2, 6, construction.LayerObject)
	g.moveBuilding(bed, construction.South, 6, 6)
	mv := g.frameIDs[len(g.frameIDs)-1]
	if s := status(mv); s != "Moving bed here: needs no materials" {
		t.Fatalf("reinstall status %q", s)
	}
	if fv, _ := frameView(g, mv); fv.WorkTotal != max(minReinstallWork, construction.WorkFor(construction.Bed, item.Wood)/2) {
		t.Fatalf("reinstall work %d", fv.WorkTotal)
	}
	// 2 nothing in stock
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 10, 2)
	fid := g.layerFrameAt(10, 2, construction.LayerObject)
	if s := status(fid); s != "No wood in stock" {
		t.Fatalf("no stock status %q", s)
	}
	// 3 on the way: a hauler carries the last of it
	sid, _ := g.SpawnStack(item.Wood, 2, 4, 4)
	g.haul.incoming = map[entity.EntityID]int{fid: 3}
	g.haul.deliveries = map[entity.EntityID][]haulShare{ada: {{Frame: fid, Qty: 3}}}
	g.jobs.Set(ada, job.Record{Kind: job.Haul, State: job.Goto, Target: sid, Dest: fid, Route: []entity.EntityID{fid}})
	fv, _ := frameView(g, fid)
	if fv.Status != frameOnTheWay || fv.Incoming != 3 || len(fv.Haulers) != 1 || fv.Haulers[0] != "Ada (3 wood)" {
		t.Fatalf("on the way: %+v", fv)
	}
	// 4 materials free, nobody carrying
	g.releaseDelivery(ada)
	g.jobs.Set(ada, job.Record{})
	if s := status(fid); s != frameNoHauler {
		t.Fatalf("waiting for a hauler %q", s)
	}
	// 5 and 6: materials in
	fr, _ := g.frames.Get(fid)
	fr.Wood = fr.Need
	g.frames.Set(fid, fr)
	if s := status(fid); s != frameNoBuilder {
		t.Fatalf("waiting for a builder %q", s)
	}
	g.jobs.Set(ada, job.Record{Kind: job.Construct, State: job.Do, Target: fid, TX: 10, TY: 2})
	if s := status(fid); s != "Being built by Ada" {
		t.Fatalf("being built %q", s)
	}
}

func TestFrameStatusNoCarpetInStock(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	carpetFrameAt(g, item.CarpetCloth, 5, 4)
	carpetFrameAt(g, item.CarpetStraw, 6, 4)
	fv, _ := frameView(g, g.layerFrameAt(5, 4, construction.LayerObject))
	if fv.Status != "No cloth carpet in stock: sew one at a tailoring bench" {
		t.Fatalf("cloth carpet status %q", fv.Status)
	}
	fv, _ = frameView(g, g.layerFrameAt(6, 4, construction.LayerObject))
	if fv.Status != "No straw carpet in stock: weave one at a workbench" {
		t.Fatalf("straw carpet status %q", fv.Status)
	}
	if g.StockOf("cloth carpet") != 0 {
		t.Fatal("no carpets yet")
	}
	g.SpawnStack(item.CarpetCloth, 2, 1, 1)
	if g.StockOf("cloth carpet") != 2 {
		t.Fatalf("StockOf = %d", g.StockOf("cloth carpet"))
	}
	if fv, _ = frameView(g, g.layerFrameAt(5, 4, construction.LayerObject)); fv.Status != frameNoHauler || fv.InStock != 2 {
		t.Fatalf("with a carpet in stock: %+v", fv)
	}
}

func TestCancelFrameRefundsMaterials(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"command", "cancel area"} {
		g := New(Config{Seed: 1, Width: 16, Height: 8})
		ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
		onlyWork(g, ada)
		g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 8, 4)
		fid := g.layerFrameAt(8, 4, construction.LayerObject)
		fr, _ := g.frames.Get(fid)
		fr.Wood = 1
		g.frames.Set(fid, fr)
		// a hauler is bringing the rest; another pawn holds the frame
		sid, _ := g.SpawnStack(item.Wood, 10, 2, 2)
		rec, shares, ok := g.planDelivery(ada, Position{1, 1})
		if !ok || len(shares) != 1 || shares[0].Frame != fid || rec.Target != sid {
			t.Fatalf("%s: delivery %+v %v", how, shares, ok)
		}
		g.haul.incoming = map[entity.EntityID]int{fid: shares[0].Qty}
		g.haul.deliveries = map[entity.EntityID][]haulShare{ada: shares}
		g.reserve.TryEntity(999, fid)
		if how == "command" {
			g.Enqueue(command.Command{Kind: command.CancelFrame, Target: uint64(fid)})
		} else {
			g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 7, Y: 3, X2: 9, Y2: 5})
		}
		g.Step(1)
		if _, ok := g.frames.Get(fid); ok {
			t.Fatalf("%s: the frame is gone", how)
		}
		refund := 0
		for _, id := range g.itemIDs {
			if st, _ := g.stacks.Get(id); id != sid && st.Kind == item.Wood && st.Carrier == 0 {
				refund += st.Qty
				if pathDist := max(abs(st.X-8), abs(st.Y-4)); pathDist > 1 {
					t.Fatalf("%s: refund dropped at %d,%d", how, st.X, st.Y)
				}
			}
		}
		if refund != 1 {
			t.Fatalf("%s: refunded %d wood, want 1", how, refund)
		}
		if g.haul.incoming[fid] != 0 || len(g.haul.deliveries[ada]) != 0 {
			t.Fatalf("%s: haul shares kept: %v %v", how, g.haul.incoming, g.haul.deliveries)
		}
		if _, taken := g.reserve.EntityOwner(fid); taken {
			t.Fatalf("%s: the frame's reservation is kept", how)
		}
	}
}

func TestFrameInStockCountsOncePerKind(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	for x := 6; x <= 8; x++ {
		g.placeFrameStuff(construction.Wall, item.Wood, construction.South, x, 4)
	}
	carpetFrameAt(g, item.CarpetCloth, 10, 4)
	g.SpawnStack(item.Wood, 10, 1, 1)
	r, _ := g.SpawnStack(item.Wood, 5, 2, 1)
	g.reserve.TryEntity(999, r)
	f, _ := g.SpawnStack(item.Wood, 7, 3, 1)
	g.setForbidden(f, true)
	for _, fv := range g.World().Frames {
		switch fv.Stuff {
		case "wood":
			if fv.InStock != 15 || fv.Reserved != 5 {
				t.Fatalf("wall frame stock %d reserved %d, want 15/5", fv.InStock, fv.Reserved)
			}
		case "cloth carpet":
			if fv.InStock != 0 || fv.Reserved != 0 {
				t.Fatalf("carpet frame stock %d/%d", fv.InStock, fv.Reserved)
			}
		}
	}
}

// A carpet sewn earlier lies in a stockpile across the map; its blueprint
// sits on the tile just inside a door. A colonist brings it and lays it.
func TestCarpetBuiltFromStockpiledCarpetBesideDoor(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 12})
	ada, _ := g.SpawnNamedPawn(2, 6, "Ada")
	onlyWork(g, ada, work.Haul, work.Construct)
	// a small room, door on its west wall at (20, 5)
	for x := 20; x <= 24; x++ {
		g.BuildNow("wall", "wood", "south", x, 3)
		g.BuildNow("wall", "wood", "south", x, 7)
	}
	for y := 4; y <= 6; y++ {
		g.BuildNow("wall", "wood", "south", 24, y)
		if y != 5 {
			g.BuildNow("wall", "wood", "south", 20, y)
		}
	}
	if !g.BuildNow("door", "wood", "south", 20, 5) {
		t.Fatal("door not built")
	}
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 1, Y: 1, X2: 3, Y2: 2})
	g.SpawnStack(item.CarpetCloth, 1, 2, 1)
	g.Step(1)
	if ok, r := g.CanPlaceAt("carpet", "cloth carpet", "south", 21, 5); !ok {
		t.Fatalf("carpet inside the door: %q", r)
	}
	carpetFrameAt(g, item.CarpetCloth, 21, 5)
	for i := 0; i < 400 && g.layerBldgAt(21, 5, construction.LayerObject) == 0; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	bid := g.layerBldgAt(21, 5, construction.LayerObject)
	if b, _ := g.bldgs.Get(bid); bid == 0 || b.Kind != construction.Carpet || b.Stuff != item.CarpetCloth {
		fv, _ := frameView(g, g.layerFrameAt(21, 5, construction.LayerObject))
		t.Fatalf("carpet not installed; frame %+v", fv)
	}
	if g.StockOf("cloth carpet") != 0 {
		t.Fatal("the carpet item went into the floor covering")
	}
}
