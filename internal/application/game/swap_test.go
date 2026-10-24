package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// In-place material swaps (PLAY-011 §2): the same piece placed on itself
// in another material is a swap frame; a builder converts the standing
// building, which keeps its id, and the old material is refunded.

func swapGame(t *testing.T) *Game {
	t.Helper()
	return New(Config{Seed: 1, Width: 16, Height: 10})
}

// objAt is the id of the building on layer at (x, y).
func objAt(g *Game, x, y int, layer construction.Layer) entity.EntityID {
	return g.layerBldgAt(x, y, layer)
}

// swapFrameOn is the frame on layer at (x, y) and its id.
func swapFrameOn(t *testing.T, g *Game, x, y int, layer construction.Layer) (entity.EntityID, construction.Frame) {
	t.Helper()
	fid := g.layerFrameAt(x, y, layer)
	fr, ok := g.frames.Get(fid)
	if !ok {
		t.Fatalf("no frame at %d,%d", x, y)
	}
	return fid, fr
}

// commandBuild is a DesignateBuild of kind k in stuff at (x, y).
func commandBuild(k construction.Kind, stuff item.Kind, x, y int) command.Command {
	return command.Command{Kind: command.DesignateBuild, Build: uint8(k), Stuff: uint8(stuff), X: x, Y: y}
}

// finishFrame fills a frame's materials and completes it.
func finishFrame(g *Game, fid entity.EntityID) {
	fr, _ := g.frames.Get(fid)
	fr.Wood, fr.Comp = fr.Need, fr.CompNeed
	g.frames.Set(fid, fr)
	g.completeFrame(fid, fr)
}

func TestSwapWoodWallToSteelInPlace(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	bid := objAt(g, 5, 4, construction.LayerObject)
	b, _ := g.bldgs.Get(bid)
	b.Damage = 30
	g.bldgs.Set(bid, b)
	if _, err := g.SpawnNamedPawn(2, 2, "Ada"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Steel, 10, 3, 6); err != nil {
		t.Fatal(err)
	}
	if ok, r := g.CanPlaceAt("wall", "steel", "south", 5, 4); !ok {
		t.Fatalf("steel over a wood wall: %q", r)
	}
	if got := g.ReplacesAt("wall", "steel", 5, 4); got != "wood wall → steel" {
		t.Fatalf("ReplacesAt = %q", got)
	}
	msgs := len(g.messages)
	g.Enqueue(commandBuild(construction.Wall, item.Steel, 5, 4))
	g.Step(1)
	_, fr := swapFrameOn(t, g, 5, 4, construction.LayerObject)
	if fr.Swaps != bid || fr.Replaces != 0 || fr.ReplaceMarked || fr.Stuff != item.Steel {
		t.Fatalf("swap frame = %+v", fr)
	}
	if fr.Need != construction.WoodCost(construction.Wall) || fr.CompNeed != 0 ||
		fr.WorkLeft != construction.WorkFor(construction.Wall, item.Steel) {
		t.Fatalf("swap frame costs = %+v", fr)
	}
	if b, _ := g.bldgs.Get(bid); b.Deconstruct {
		t.Fatal("a swap must not mark the wall for deconstruction")
	}
	if len(g.frameIDs) != 1 {
		t.Fatalf("frames = %d, want one", len(g.frameIDs))
	}
	done := false
	for i := 0; i < 30000 && !done; i++ {
		g.Step(1)
		if g.grid.Walkable(5, 4) {
			t.Fatalf("tick %d: the wall cell opened during the swap", i)
		}
		if objAt(g, 5, 4, construction.LayerObject) != bid {
			t.Fatalf("tick %d: the wall changed identity", i)
		}
		b, _ := g.bldgs.Get(bid)
		done = b.Stuff == item.Steel
	}
	if !done {
		t.Fatalf("swap never finished; frames=%+v", g.World().Frames)
	}
	b, _ = g.bldgs.Get(bid)
	if b.Damage != 0 || b.HP() != construction.MaxHP(construction.Wall, item.Steel) {
		t.Fatalf("hp after swap = %d (damage %d)", b.HP(), b.Damage)
	}
	if len(g.frameIDs) != 0 {
		t.Fatalf("frame left behind: %v", g.frameIDs)
	}
	if got := itemTotals(g)[item.Wood]; got != construction.Refund(construction.Wall) {
		t.Fatalf("wood refund = %d, want %d", got, construction.Refund(construction.Wall))
	}
	if sid := g.itemAt(5, 4); sid != 0 {
		t.Fatal("the refund was dropped inside the wall")
	}
	if len(g.messages) != msgs {
		t.Fatalf("a swap logged messages: %+v", g.messages[msgs:])
	}
}

func TestSwapCompletionKeepsBuildingAndRescoresRooms(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	bid := objAt(g, 5, 4, construction.LayerObject)
	g.placeFrameStuff(construction.Wall, item.Steel, construction.South, 5, 4)
	fid, _ := swapFrameOn(t, g, 5, 4, construction.LayerObject)
	g.roomDirty = false
	ver := g.structVer
	finishFrame(g, fid)
	if !g.roomDirty || g.structVer == ver {
		t.Fatal("a swap must re-score rooms and bump structVer")
	}
	if b, ok := g.bldgs.Get(bid); !ok || b.Stuff != item.Steel || len(g.bldgIDs) != 1 {
		t.Fatalf("building after swap = %+v ok=%v ids=%v", b, ok, g.bldgIDs)
	}
}

func TestSwapDoorKeepsWorkingDuringJob(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 3)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 5)
	placeBuilt(t, g, construction.Door, construction.South, 5, 4)
	bid := objAt(g, 5, 4, construction.LayerObject)
	walk := g.grid.Walkable(5, 4)
	if ok, r := g.CanPlaceAt("door", "steel", "south", 5, 4); !ok {
		t.Fatalf("steel door over a wood door: %q", r)
	}
	if got := g.ReplacesAt("door", "steel", 5, 4); got != "wood door → steel" {
		t.Fatalf("ReplacesAt = %q", got)
	}
	g.placeFrameStuff(construction.Door, item.Steel, construction.South, 5, 4)
	fid, fr := swapFrameOn(t, g, 5, 4, construction.LayerObject)
	if fr.Swaps != bid || fr.Replaces != 0 || fr.Facing != construction.East {
		t.Fatalf("door swap frame = %+v", fr)
	}
	if b, _ := g.bldgs.Get(bid); b.Deconstruct || g.grid.Walkable(5, 4) != walk {
		t.Fatal("the door stopped working while being changed")
	}
	finishFrame(g, fid)
	b, _ := g.bldgs.Get(bid)
	if b.Stuff != item.Steel || b.Facing != construction.East || g.grid.Walkable(5, 4) != walk {
		t.Fatalf("door after swap = %+v", b)
	}
}

func TestSwapBedKeepsClaimantHospitalBedNeedsNoComponents(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	pid, err := g.SpawnNamedPawn(1, 1, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	placeBuilt(t, g, construction.Bed, construction.South, 3, 3)
	bid := objAt(g, 3, 3, construction.LayerObject)
	b, _ := g.bldgs.Get(bid)
	b.Claimant = uint64(pid)
	g.bldgs.Set(bid, b)
	g.placeFrameStuff(construction.Bed, item.Steel, construction.South, 3, 3)
	fid, fr := swapFrameOn(t, g, 3, 3, construction.LayerObject)
	if fr.Swaps != bid {
		t.Fatalf("bed swap frame = %+v", fr)
	}
	finishFrame(g, fid)
	if b, _ := g.bldgs.Get(bid); b.Claimant != uint64(pid) || b.Stuff != item.Steel {
		t.Fatalf("bed after swap = %+v", b)
	}
	placeBuilt(t, g, construction.HospitalBed, construction.South, 8, 3)
	hid := objAt(g, 8, 3, construction.LayerObject)
	g.placeFrameStuff(construction.HospitalBed, item.Steel, construction.South, 8, 3)
	_, hf := swapFrameOn(t, g, 8, 3, construction.LayerObject)
	if hf.Swaps != hid || hf.CompNeed != 0 || hf.Need != construction.WoodCost(construction.HospitalBed) {
		t.Fatalf("hospital bed swap frame = %+v", hf)
	}
}

func TestSwapFloorAndCarpet(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.Floor, construction.South, 4, 4)
	fl := objAt(g, 4, 4, construction.LayerFloor)
	if ok, r := g.CanPlaceAt("floor", "granite blocks", "south", 4, 4); !ok {
		t.Fatalf("stone floor over wood: %q", r)
	}
	g.placeFrameStuff(construction.Floor, item.GraniteBlocks, construction.South, 4, 4)
	fid, fr := swapFrameOn(t, g, 4, 4, construction.LayerFloor)
	if fr.Swaps != fl {
		t.Fatalf("floor swap frame = %+v", fr)
	}
	finishFrame(g, fid)
	if b, _ := g.bldgs.Get(fl); b.Stuff != item.GraniteBlocks {
		t.Fatalf("floor after swap = %+v", b)
	}
	if got := itemTotals(g)[item.Wood]; got != construction.Refund(construction.Floor) {
		t.Fatalf("floor refund = %d", got)
	}
	g.placeFrameStuff(construction.Carpet, item.CarpetStraw, construction.South, 8, 4)
	cfid, _ := swapFrameOn(t, g, 8, 4, construction.LayerObject)
	finishFrame(g, cfid)
	cid := objAt(g, 8, 4, construction.LayerObject)
	if got := g.ReplacesAt("carpet", "cloth carpet", 8, 4); got != "straw carpet → cloth" {
		t.Fatalf("carpet ReplacesAt = %q", got)
	}
	g.placeFrameStuff(construction.Carpet, item.CarpetCloth, construction.South, 8, 4)
	cfid, cf := swapFrameOn(t, g, 8, 4, construction.LayerObject)
	if cf.Swaps != cid {
		t.Fatalf("carpet swap frame = %+v", cf)
	}
	finishFrame(g, cfid)
	if b, _ := g.bldgs.Get(cid); b.Stuff != item.CarpetCloth {
		t.Fatalf("carpet after swap = %+v", b)
	}
}

func TestSwapMultiTileNeedsIdenticalFootprint(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.Bed, construction.South, 5, 3) // covers 5,3 and 5,4
	if ok, r := g.CanPlaceAt("bed", "steel", "south", 5, 3); !ok {
		t.Fatalf("same footprint: %q", r)
	}
	if ok, r := g.CanPlaceAt("bed", "steel", "south", 5, 4); ok || r != ReasonOccupied {
		t.Fatalf("offset bed: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("bed", "steel", "north", 5, 3); ok || r != ReasonOccupied {
		t.Fatalf("other facing: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("bed", "wood", "south", 5, 3); ok || r != ReasonSame {
		t.Fatalf("same material bed: %v %q", ok, r)
	}
	placeBuilt(t, g, construction.Table, construction.South, 9, 3)
	if ok, r := g.CanPlaceAt("table", "steel", "south", 9, 3); !ok {
		t.Fatalf("table swap: %q", r)
	}
	if ok, r := g.CanPlaceAt("chair", "steel", "south", 9, 3); ok || r != ReasonOccupied {
		t.Fatalf("chair over a table: %v %q", ok, r)
	}
}

func TestSwapSameMaterialIsNeutral(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	placeBuilt(t, g, construction.FenceGate, construction.South, 9, 4)
	msgs := len(g.messages)
	if ok, r := g.CanPlaceAt("wall", "wood", "south", 5, 4); ok || r != ReasonSame {
		t.Fatalf("wood over wood: %v %q", ok, r)
	}
	if got := g.ReplacesAt("wall", "wood", 5, 4); got != "" {
		t.Fatalf("ReplacesAt same material = %q", got)
	}
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 5, 4)
	if len(g.frameIDs) != 0 || len(g.messages) != msgs {
		t.Fatalf("same material queued %v / logged", g.frameIDs)
	}
	if ok, r := g.CanPlaceAt("fence gate", "wood", "east", 9, 4); ok || r != ReasonSame {
		t.Fatalf("gate over gate: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceStuff("wall", "wood", 5, 4); ok || r != ReasonSame {
		t.Fatalf("CanPlaceStuff same: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceStuff("wall", "steel", 5, 4); !ok {
		t.Fatalf("CanPlaceStuff swap: %q", r)
	}
}

func TestSwapPendingRefusesMove(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	chair := built(t, g, "chair", "south", 4, 4)
	if ok, _ := g.CanMoveTo(chair, "south", 8, 8); !ok {
		t.Fatal("a chair may move")
	}
	g.placeFrameStuff(construction.Chair, item.Steel, construction.South, 4, 4)
	if ok, why := g.CanMoveTo(chair, "south", 8, 8); ok || why != ReasonSwapPending {
		t.Fatalf("move during swap: %v %q", ok, why)
	}
}

func TestSwapTargetGoneBuildsNew(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.HospitalBed, construction.South, 4, 3)
	hid := objAt(g, 4, 3, construction.LayerObject)
	g.placeFrameStuff(construction.HospitalBed, item.Steel, construction.South, 4, 3)
	fid, _ := swapFrameOn(t, g, 4, 3, construction.LayerObject)
	g.removeBuilding(hid) // deconstructed before the swap was done
	fr, _ := g.frames.Get(fid)
	if fr.Swaps != 0 || fr.CompNeed != construction.ComponentCost(construction.HospitalBed) {
		t.Fatalf("frame after its target went = %+v", fr)
	}
	finishFrame(g, fid)
	nb, ok := g.bldgs.Get(fid)
	if !ok || nb.Kind != construction.HospitalBed || nb.Stuff != item.Steel {
		t.Fatalf("new building = %+v ok=%v", nb, ok)
	}
	// a wall frame whose wall was smashed completes as a new blocking wall
	placeBuilt(t, g, construction.Wall, construction.South, 10, 4)
	wid := objAt(g, 10, 4, construction.LayerObject)
	g.placeFrameStuff(construction.Wall, item.Steel, construction.South, 10, 4)
	wfid, _ := swapFrameOn(t, g, 10, 4, construction.LayerObject)
	g.damageBuilding(wid, 10000)
	if g.bldgs.Has(wid) {
		t.Fatal("wall not smashed")
	}
	if len(g.frameIDs) != 1 {
		t.Fatalf("frames after smash = %v", g.frameIDs)
	}
	for id := g.itemAt(10, 4); id != 0; id = g.itemAt(10, 4) {
		g.deleteItem(id)
	}
	finishFrame(g, wfid)
	if nb, ok := g.bldgs.Get(wfid); !ok || nb.Stuff != item.Steel || g.grid.Walkable(10, 4) {
		t.Fatalf("rebuilt wall = %+v ok=%v", nb, ok)
	}
}

func TestSwapCancelRefundsAndKeepsBuilding(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	bid := objAt(g, 5, 4, construction.LayerObject)
	g.placeFrameStuff(construction.Wall, item.Steel, construction.South, 5, 4)
	fid, fr := swapFrameOn(t, g, 5, 4, construction.LayerObject)
	fr.Wood = 1
	g.frames.Set(fid, fr)
	g.cancelFrame(fid)
	b, ok := g.bldgs.Get(bid)
	if !ok || b.Stuff != item.Wood || b.Deconstruct || g.grid.Walkable(5, 4) {
		t.Fatalf("wall after cancel = %+v ok=%v", b, ok)
	}
	if got := itemTotals(g)[item.Steel]; got != 1 {
		t.Fatalf("steel refund = %d", got)
	}
	// the cancel tool too
	g.placeFrameStuff(construction.Wall, item.Steel, construction.South, 5, 4)
	g.cancelRect(5, 4, 5, 4)
	if len(g.frameIDs) != 0 || !g.bldgs.Has(bid) {
		t.Fatalf("cancel rect: frames %v", g.frameIDs)
	}
}

func TestRepairSkipsBuildingWithSwapPending(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	pid, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	bid := objAt(g, 5, 4, construction.LayerObject)
	g.damageBuilding(bid, 20)
	g.placeFrameStuff(construction.Wall, item.Steel, construction.South, 5, 4)
	if g.assignRepair(pid) {
		t.Fatal("repair started on a wall being changed")
	}
	g.cancelRect(5, 4, 5, 4)
	if !g.assignRepair(pid) {
		t.Fatal("repair should start once the swap is cancelled")
	}
}

func TestSwapViewsAndSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	bid := objAt(g, 5, 4, construction.LayerObject)
	g.placeFrameStuff(construction.Wall, item.Steel, construction.South, 5, 4)
	fid, fr := swapFrameOn(t, g, 5, 4, construction.LayerObject)
	total := fr.WorkLeft
	fr.Wood, fr.WorkLeft = fr.Need, total/2
	g.frames.Set(fid, fr)
	w := g.World()
	var bv BuildingView
	for _, b := range w.Buildings {
		if b.ID == bid {
			bv = b
		}
	}
	if bv.SwapTo != "steel" || bv.SwapPct != (total-total/2)*100/total {
		t.Fatalf("building view swap = %q %d", bv.SwapTo, bv.SwapPct)
	}
	if len(w.Frames) != 1 || w.Frames[0].Status != "Changing to steel" {
		t.Fatalf("frame status = %+v", w.Frames)
	}
	l := saveLoad(t, g)
	if lf, ok := l.frames.Get(fid); !ok || lf.Swaps != bid {
		t.Fatalf("Swaps lost in save: %+v", lf)
	}
	for _, b := range l.World().Buildings {
		if b.ID == bid && b.SwapTo != "steel" {
			t.Fatalf("loaded view swap = %q", b.SwapTo)
		}
	}
}
