package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game/orders"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
)

// AX-1: the Architect's remaining orders and zone clears (game/orders).

func ax1Cmd(g *Game, c command.Command) {
	g.Enqueue(c)
	g.Step(1)
}

func ax1Rect(k command.Kind, x1, y1, x2, y2 int, on bool) command.Command {
	return command.Command{Kind: k, X: x1, Y: y1, X2: x2, Y2: y2, Enabled: on}
}

// crateOf finds the crated building on the map (0: none).
func crateOf(g *Game) (entity.EntityID, item.Stack) {
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.MinifiedBuilding {
			return id, st
		}
	}
	return 0, item.Stack{}
}

func TestUninstallCratesAndReinstallKeepsQualityAndWear(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	bid := built(t, g, "bed", "south", 4, 4)
	b, _ := g.bldgs.Get(bid)
	b.Quality, b.Damage = 5, 7
	g.bldgs.Set(bid, b)
	bob := builder(t, g, 1, 1)

	ax1Cmd(g, ax1Rect(command.DesignateUninstall, 4, 4, 4, 4, true))
	if b, _ := g.bldgs.Get(bid); !b.Uninstall {
		t.Fatal("the bed is marked to uninstall")
	}
	var bv BuildingView
	for _, v := range g.World().Buildings {
		if v.ID == bid {
			bv = v
		}
	}
	if !bv.AX.Uninstall {
		t.Fatal("the building view shows the uninstall mark")
	}
	if !runCare(g, 400, func() bool { c, _ := crateOf(g); return c != 0 }, bob) {
		t.Fatal("a builder crates it")
	}
	if g.bldgs.Has(bid) {
		t.Fatal("the building is gone from the map")
	}
	cid, st := crateOf(g)
	if construction.Kind(st.Building) != construction.Bed || st.Stuff != b.Stuff || st.Quality != 5 || st.Wear != 7 {
		t.Fatalf("crate keeps kind, stuff, quality and wear: %+v", st)
	}
	for _, iv := range g.World().Items {
		if iv.ID == cid && iv.AX.Building != "bed" {
			t.Fatalf("item view names the crated kind: %+v", iv.AX)
		}
	}

	ax1Cmd(g, command.Command{Kind: command.Reinstall, Target: uint64(cid), X: 12, Y: 7, Facing: uint8(construction.East)})
	if fid := ordersOf(g).Install.FrameOf(cid); fid == 0 {
		t.Fatal("reinstall lays a blueprint")
	}
	if g.assignConstruct(bob) {
		t.Fatal("a crate's blueprint is never built bare")
	}
	for i := 0; i < 6000 && g.layerBldgAt(12, 7, construction.LayerObject) == 0; i++ {
		g.Step(1) // one tick at a time: builders repair wear right after
		if i%100 == 0 {
			g.SetNeeds(bob, needs.Full, needs.Full)
		}
	}
	if g.layerBldgAt(12, 7, construction.LayerObject) == 0 {
		t.Fatal("the builder carries the crate over and installs it")
	}
	nb, _ := g.bldgs.Get(g.layerBldgAt(12, 7, construction.LayerObject))
	if nb.Kind != construction.Bed || nb.Stuff != b.Stuff || nb.Quality != 5 || nb.Damage != 7 || nb.Facing != construction.East {
		t.Fatalf("installed as it was packed: %+v", nb)
	}
	if c, _ := crateOf(g); c != 0 || len(g.frameIDs) != 0 {
		t.Fatalf("crate %d and frames %v left", c, g.frameIDs)
	}
}

func TestUninstallRefusesFixedPiecesAndUndesignates(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	wall := built(t, g, "wall", "south", 2, 2)
	bed := built(t, g, "bed", "south", 6, 6)
	ax1Cmd(g, ax1Rect(command.DesignateUninstall, 0, 0, 15, 11, true))
	if w, _ := g.bldgs.Get(wall); w.Uninstall {
		t.Fatal("a wall can't be uninstalled")
	}
	if b, _ := g.bldgs.Get(bed); !b.Uninstall {
		t.Fatal("a bed can")
	}
	ax1Cmd(g, command.Command{Kind: command.Undesignate, Order: command.OrderUninstall, X: 0, Y: 0, X2: 15, Y2: 11})
	if b, _ := g.bldgs.Get(bed); b.Uninstall {
		t.Fatal("undesignate takes the mark off")
	}
	ax1Cmd(g, ax1Rect(command.DesignateUninstall, 6, 6, 6, 6, true))
	ax1Cmd(g, ax1Rect(command.DesignateCancel, 6, 6, 6, 6, false))
	if b, _ := g.bldgs.Get(bed); b.Uninstall {
		t.Fatal("cancel takes it off too")
	}
	ax1Cmd(g, ax1Rect(command.DesignateUninstall, 6, 6, 6, 6, true))
	ax1Cmd(g, ax1Rect(command.DesignateDeconstruct, 6, 6, 6, 6, true))
	if b, _ := g.bldgs.Get(bed); b.Uninstall || !b.Deconstruct {
		t.Fatal("a deconstruct mark replaces the uninstall mark")
	}
}

func TestChopWoodMarksGrownTreesOnly(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	d := plant.DefOf(plant.Tree)
	young, _ := g.spawnPlant(plant.Tree, 2, 2, d.GrowTicks*39/100)
	grown, _ := g.spawnPlant(plant.Tree, 3, 2, d.GrowTicks*40/100)
	bush, _ := g.spawnPlant(plant.BerryBush, 4, 2, plant.DefOf(plant.BerryBush).GrowTicks)
	ax1Cmd(g, ax1Rect(command.DesignateChopWood, 0, 0, 15, 11, true))
	cut := func(id entity.EntityID) bool { p, _ := g.plants.Get(id); return p.Cut }
	if cut(young) || !cut(grown) || cut(bush) {
		t.Fatalf("chop wood: young %v grown %v bush %v", cut(young), cut(grown), cut(bush))
	}
	ax1Cmd(g, command.Command{Kind: command.Undesignate, Order: command.OrderChopWood, X: 0, Y: 0, X2: 15, Y2: 11})
	if cut(grown) {
		t.Fatal("undesignate unmarks the tree")
	}
}

func TestSlaughterMarksTameAnimalsOnly(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	wild, _ := g.SpawnAnimal("dustbuck", 3, 3)
	tame, _ := g.SpawnAnimal("dustbuck", 5, 3)
	a, _ := g.animals.Get(tame)
	a.Tame = true
	g.animals.Set(tame, a)
	ax1Cmd(g, ax1Rect(command.DesignateSlaughter, 0, 0, 15, 11, true))
	mark := func(id entity.EntityID) bool { a, _ := g.animals.Get(id); return a.Designated }
	if mark(wild) || !mark(tame) {
		t.Fatalf("slaughter: wild %v tame %v", mark(wild), mark(tame))
	}
	ax1Cmd(g, command.Command{Kind: command.Undesignate, Order: command.OrderSlaughter, X: 0, Y: 0, X2: 15, Y2: 11})
	if mark(tame) {
		t.Fatal("undesignate unmarks it")
	}
}

func TestClaimMakesRuinsTheColonys(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	ruin := g.placeGenerated(construction.Building{Kind: construction.Bed, Stuff: item.Wood, X: 5, Y: 5, Ruin: true})
	camp := g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: 9, Y: 5, Ancient: true})
	before := refundOf(construction.Building{Kind: construction.Bed, Ruin: true})
	ax1Cmd(g, ax1Rect(command.DesignateClaim, 0, 0, 15, 11, true))
	for _, id := range []entity.EntityID{ruin, camp} {
		if b, _ := g.bldgs.Get(id); b.Ruin || b.Ancient || notColonyPiece(b) {
			t.Fatalf("claimed: %+v", b)
		}
	}
	if b, _ := g.bldgs.Get(ruin); refundOf(b) < before {
		t.Fatal("a claimed ruin refunds in full")
	}
	if !g.inHome(5*g.grid.Width() + 5) {
		t.Fatal("a claimed building joins the Home area")
	}
}

func TestStripMarkAHaulerStripsTheDowned(t *testing.T) {
	g, doc, patient := careScene(t)
	g.setPriority(doc, work.Doctor, 0) // a hauler here, not the patient's doctor
	g.equip[patient] = item.Rifle
	pos, _ := g.positions.Get(patient)
	ax1Cmd(g, ax1Rect(command.DesignateStrip, pos.X, pos.Y, pos.X, pos.Y, true))
	if !ordersOf(g).Strip.Marked(patient) {
		t.Fatal("the downed patient is marked")
	}
	marked := false
	for _, pv := range g.Snapshot() {
		marked = marked || pv.ID == patient && pv.AX.StripMarked
	}
	if !marked {
		t.Fatal("the pawn view shows the strip mark")
	}
	if !runCare(g, 400, func() bool { return !careGear{g}.HasGear(patient) }, doc, patient) {
		t.Fatal("a hauler strips the marked pawn")
	}
	if ordersOf(g).Strip.Marked(patient) {
		t.Fatal("the mark is done")
	}
	if v := g.VitalOf(patient); v.Status != health.Downed {
		t.Fatalf("still downed: %+v", v)
	}
}

func TestOpenGraveLiftsTheBodyOut(t *testing.T) {
	g, ada := burGame(t, 24, 12)
	grave := burGrave(t, g, construction.South, 14, 4)
	cy := burKill(t, g, 12, 8, "Cy")
	cid, _, _ := burBody(g, cy)
	g.layInGrave(grave, cid)
	if _, _, ok := burBody(g, cy); ok {
		t.Fatal("fixture: the body lies in the grave")
	}
	ax1Cmd(g, ax1Rect(command.DesignateOpen, 14, 4, 14, 4, true))
	b, _ := g.bldgs.Get(grave)
	if !ordersOf(g).Open.Marked(grave, b) {
		t.Fatal("the grave is marked to open")
	}
	if !burRun(g, 4000, func() bool { _, _, ok := burBody(g, cy); return ok }) {
		t.Fatal("a hauler opens it")
	}
	if g.graveFilled(grave) || ordersOf(g).Open.Marked(grave, b) {
		t.Fatal("the grave is empty and unmarked")
	}
	_ = ada
}

func TestDeleteZoneAndClearRoofAreas(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	ax1Cmd(g, command.Command{Kind: command.DesignateStockpile, X: 2, Y: 2, X2: 6, Y2: 6})
	ax1Cmd(g, ax1Rect(command.DesignateDeleteZone, 2, 2, 4, 6, true))
	if _, ok := g.storageAt(3, 3); ok {
		t.Fatal("deleted cells hold no zone")
	}
	if _, ok := g.storageAt(6, 6); !ok {
		t.Fatal("cells outside the drag keep theirs")
	}
	ax1Cmd(g, ax1Rect(command.DesignateRoof, 8, 2, 10, 4, true))
	ax1Cmd(g, ax1Rect(command.DesignateClearRoofArea, 8, 2, 8, 4, true))
	w := g.grid.Width()
	if g.roofs[2*w+8].Has(roof.AreaBuild) || !g.roofs[2*w+9].Has(roof.AreaBuild) {
		t.Fatal("clear build roof area erases only the dragged cells")
	}
	ax1Cmd(g, ax1Rect(command.DesignateRemoveRoof, 11, 2, 12, 4, false))
	ax1Cmd(g, ax1Rect(command.DesignateClearNoRoofArea, 11, 2, 11, 4, true))
	if g.roofs[2*w+11].Has(roof.NoAuto) || !g.roofs[2*w+12].Has(roof.NoAuto) {
		t.Fatal("clear remove roof area erases only the dragged cells")
	}
	if !g.roofs[2*w+9].Has(roof.AreaBuild) {
		t.Fatal("clearing the remove area leaves the build area")
	}
}

func TestRemoveFloorMarksFloorsOnly(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	floor := built(t, g, "floor", "south", 4, 4)
	bed := built(t, g, "bed", "south", 4, 4)
	ax1Cmd(g, ax1Rect(command.DesignateRemoveFloor, 0, 0, 15, 11, true))
	if f, _ := g.bldgs.Get(floor); !f.Deconstruct {
		t.Fatal("the floor is marked")
	}
	if b, _ := g.bldgs.Get(bed); b.Deconstruct {
		t.Fatal("the bed on it is not")
	}
	ax1Cmd(g, command.Command{Kind: command.Undesignate, Order: command.OrderRemoveFloor, X: 0, Y: 0, X2: 15, Y2: 11})
	if f, _ := g.bldgs.Get(floor); f.Deconstruct {
		t.Fatal("undesignate unmarks it")
	}
	if got := refundOf(construction.Building{Kind: construction.Flagstone, Stuff: item.GraniteBlocks}); got != 0 {
		t.Fatalf("flagstone refunds nothing, got %d", got)
	}
	if got := refundOf(construction.Building{Kind: construction.SilverTile, Stuff: item.Silver}); got != construction.DefOf(construction.SilverTile).Wood/2 {
		t.Fatalf("silver tile refunds half, got %d", got)
	}
}

func TestOrdersMarksSurviveSave(t *testing.T) {
	g, _, patient := careScene(t)
	grave := burGrave(t, g, construction.South, 2, 2)
	cy := burKill(t, g, 3, 8, "Cy")
	cid, _, _ := burBody(g, cy)
	g.layInGrave(grave, cid)
	pos, _ := g.positions.Get(patient)
	ax1Cmd(g, ax1Rect(command.DesignateStrip, pos.X, pos.Y, pos.X, pos.Y, true))
	ax1Cmd(g, ax1Rect(command.DesignateOpen, 2, 2, 2, 2, true))
	var buf bytes.Buffer
	if err := g.Save(&buf, "ax1"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	got := ordersOf(h).Export()
	if len(got.Strip) != 1 || got.Strip[0] != patient || len(got.Open) != 1 || got.Open[0] != grave {
		t.Fatalf("marks after load: %+v", got)
	}
}

func TestInstallJobsAreOrdersJobs(t *testing.T) {
	for _, k := range []job.Kind{job.Uninstall, job.Install, job.OpenContainer} {
		if !postponable(k) {
			t.Fatalf("%v is put off when out of reach", k)
		}
	}
	if orders.WorkTotal(job.OpenContainer, nil) != orders.OpenWork {
		t.Fatal("opening shows its work")
	}
}

// TestBuyCratedBigScreenFromOrbitalTrader: an exotic orbital trader's
// crated big screen television shows as its own trade row, is bought,
// comes down in the drop pod as a crated building of that kind, and
// Reinstall places its blueprint.
func TestBuyCratedBigScreenFromOrbitalTrader(t *testing.T) {
	t.Parallel()
	g, _, _, pawn := orbitalGame(t)
	g.orbitalArrive(uint64(g.clock.Now()))
	buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, 22, 14)
	spawn(t, g, item.Silver, 3000, 23, 15)
	openComms(t, g, pawn)
	g.Enqueue(command.Command{Kind: command.CommsCallTrader})
	g.ApplyPending()
	tv := trade.Goods{Kind: item.MinifiedBuilding, Building: uint8(construction.BigScreenTV)}
	g.trade.orbital.Stock.AddGoods(tv, 1) // whatever the dice gave, one crate for sure
	v, _ := g.TradeView()
	var row TradeRowView
	for _, r := range v.Rows {
		if r.Kind == tv.Name() {
			row = r
		}
	}
	if row.Label != "Big screen television (crated)" || row.TraderQty < 1 || !row.WillBuy || row.WillSell || row.BuyC <= 1485*100 {
		t.Fatalf("crate row %+v", row)
	}
	offer := []trade.Delta{{Kind: tv.Kind, Qty: 1, Building: tv.Building}}
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: trade.EncodeOffer(offer)})
	g.ApplyPending()
	if v, _ := g.TradeView(); v.Reason != "" {
		t.Fatalf("refused: %s", v.Reason)
	}
	g.Step(tradePodFall + 2)
	cid, st := crateOf(g)
	if cid == 0 || construction.Kind(st.Building) != construction.BigScreenTV || st.Stuff != construction.DefaultStuff(construction.BigScreenTV) {
		t.Fatalf("delivered crate %+v", st)
	}
	g.Enqueue(command.Command{Kind: command.Reinstall, Target: uint64(cid), X: 6, Y: 4})
	g.ApplyPending()
	if fid := ordersOf(g).Install.FrameOf(cid); fid == 0 {
		t.Fatal("the bought crate takes a reinstall blueprint")
	}
}
