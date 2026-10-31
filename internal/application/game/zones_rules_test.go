package game

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
)

// PLAY-023: zone placement as RimWorld (grow zones on soil only, the map
// edge, fog, buildings and blueprints), cells lost under a finished wall,
// floor or new ground, the dumping stockpile, storage priority and the
// sowing temperature.

// TestGrowZoneOnlyOnSoil: a growing zone takes grow-soil terrain only; a
// stockpile takes everything but rock and deep water.
func TestGrowZoneOnlyOnSoil(t *testing.T) {
	t.Parallel()
	for ty := terrain.Type(0); int(ty) < len(terrain.Names()); ty++ {
		t.Run(ty.String(), func(t *testing.T) {
			t.Parallel()
			g := New(Config{Seed: 1, Width: 10, Height: 8})
			g.setTerrain(4, 4, ty)
			grow, stock := g.ZoneRefusal(ZoneGrow, 4, 4), g.ZoneRefusal(ZoneStockpile, 4, 4)
			wantGrow := ""
			switch {
			case terrain.IsRockWall(ty):
				wantGrow = ZoneRefuseRock
			case terrain.IsWater(ty):
				wantGrow = ZoneRefuseWater
			case !ty.GrowSoil():
				wantGrow = ZoneRefuseNotSoil
			}
			if grow != wantGrow {
				t.Errorf("grow zone on %v: refusal %q, want %q", ty, grow, wantGrow)
			}
			wantStock := ""
			switch {
			case terrain.IsRockWall(ty):
				wantStock = ZoneRefuseRock
			case ty == terrain.DeepWater:
				wantStock = ZoneRefuseWater
			}
			if stock != wantStock {
				t.Errorf("stockpile on %v: refusal %q, want %q", ty, stock, wantStock)
			}
			if g.CanZoneAt(ZoneGrow, 4, 4) != (wantGrow == "") {
				t.Error("CanZoneAt follows ZoneRefusal")
			}
		})
	}
}

// TestGrowZoneRefusesFloor: a built floor, or a floor's blueprint, refuses
// a growing zone; a stockpile goes on a floor.
func TestGrowZoneRefusesFloor(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	placeBuilt(t, g, construction.Floor, construction.South, 3, 3)
	if r := g.ZoneRefusal(ZoneGrow, 3, 3); r != ZoneRefuseFloor {
		t.Fatalf("grow zone on a floor: %q", r)
	}
	if !g.CanZoneAt(ZoneStockpile, 3, 3) {
		t.Fatal("a stockpile goes on a floor")
	}
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Floor), X: 5, Y: 3})
	g.Step(1)
	if r := g.ZoneRefusal(ZoneGrow, 5, 3); r != ZoneRefuseFloor {
		t.Fatalf("grow zone on a planned floor: %q", r)
	}
	zoneCmd(g, command.DesignateGrowZone, 2, 2, 6, 4, 0)
	if zoneOwner(t, g, 3, 3).ID != 0 || zoneOwner(t, g, 5, 3).ID != 0 || zoneOwner(t, g, 4, 3).Kind != ZoneGrow {
		t.Fatal("painting skips the floor cells")
	}
}

// TestStockpileAllowsShallowWater: shallow water takes a stockpile (and
// keeps what it stores there), deep water does not; a growing zone takes
// neither.
func TestStockpileAllowsShallowWater(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	g.setTerrain(3, 3, terrain.ShallowWater)
	g.setTerrain(4, 3, terrain.DeepWater)
	if !g.CanZoneAt(ZoneStockpile, 3, 3) || !g.CanZoneAt(ZoneDump, 3, 3) {
		t.Fatal("shallow water takes a stockpile")
	}
	if r := g.ZoneRefusal(ZoneStockpile, 4, 3); r != ZoneRefuseWater {
		t.Fatalf("deep water: %q", r)
	}
	if r := g.ZoneRefusal(ZoneGrow, 3, 3); r != ZoneRefuseWater {
		t.Fatalf("grow zone on shallow water: %q", r)
	}
	zoneCmd(g, command.DesignateStockpile, 3, 3, 4, 3, 0)
	if zoneOwner(t, g, 3, 3).Kind != ZoneStockpile || zoneOwner(t, g, 4, 3).ID != 0 {
		t.Fatal("the stockpile takes the shallow cell only")
	}
	// a load set down there stays (no floating off to dry ground)
	id, _ := g.SpawnStack(item.Wood, 10, 6, 6)
	st, _ := g.stacks.Get(id)
	st.X, st.Y = 3, 3
	g.mergeOrSet(id, st)
	if st, _ := g.stacks.Get(id); st.X != 3 || st.Y != 3 {
		t.Fatalf("a stored load floated to %d,%d", st.X, st.Y)
	}
	// outside a stockpile it still floats
	id2, _ := g.SpawnStack(item.Wood, 10, 6, 6)
	g.setTerrain(7, 5, terrain.ShallowWater)
	st2, _ := g.stacks.Get(id2)
	st2.X, st2.Y = 7, 5
	g.mergeOrSet(id2, st2)
	if st2, _ := g.stacks.Get(id2); g.isWaterAt(st2.X, st2.Y) {
		t.Fatal("an unstored load in water floats to dry ground")
	}
}

// TestZoneRefusesFogAndEdge: no zone on the map's edge ring or in fog;
// already zoned cells say so.
func TestZoneRefusesFogAndEdge(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	for _, c := range [][2]int{{0, 3}, {11, 3}, {5, 0}, {5, 7}, {-1, 3}, {12, 3}} {
		if r := g.ZoneRefusal(ZoneStockpile, c[0], c[1]); r != ZoneRefuseEdge {
			t.Errorf("%v: %q, want the edge refusal", c, r)
		}
	}
	if !g.CanZoneAt(ZoneStockpile, 1, 1) || !g.CanZoneAt(ZoneGrow, 10, 6) {
		t.Fatal("one cell in from the edge is fine")
	}
	for y := 0; y < 8; y++ {
		g.setTerrain(7, y, terrain.Rock) // a cliff hides the east side
	}
	g.fogFromStart(1, 1) // everything fogged but what (1,1) sees
	if !g.fogged(9, 5) {
		t.Fatal("fixture: (9,5) should be fogged")
	}
	if r := g.ZoneRefusal(ZoneGrow, 9, 5); r != ZoneRefuseFog {
		t.Fatalf("fogged cell: %q", r)
	}
	zoneCmd(g, command.DesignateStockpile, 0, 0, 11, 7, 0)
	for _, v := range g.World().Zones {
		for _, c := range v.Cells {
			if g.fogged(c[0], c[1]) || c[0] == 0 || c[1] == 0 || c[0] == 11 || c[1] == 7 {
				t.Fatalf("zoned cell %v is fogged or on the edge", c)
			}
		}
	}
	if r := g.ZoneRefusal(ZoneStockpile, 1, 1); r != ZoneRefuseTaken {
		t.Fatalf("a zoned cell: %q", r)
	}
}

// TestZoneRefusesPlannedWall: a wall's blueprint and a built wall refuse
// every zone kind.
func TestZoneRefusesPlannedWall(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	g.placeFrame(construction.Wall, 3, 3)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 3)
	for _, k := range []string{ZoneStockpile, ZoneDump, ZoneGrow} {
		if r := g.ZoneRefusal(k, 3, 3); r != ZoneRefusePlanned {
			t.Errorf("%s on a wall blueprint: %q", k, r)
		}
		if r := g.ZoneRefusal(k, 5, 3); r != ZoneRefuseBuilding {
			t.Errorf("%s on a wall: %q", k, r)
		}
	}
	// a non-blocking blueprint (a floor) is no reason for a stockpile
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Floor), X: 7, Y: 3})
	g.Step(1)
	if !g.CanZoneAt(ZoneStockpile, 7, 3) {
		t.Fatal("a stockpile goes on a planned floor")
	}
}

// TestWallBuiltOverZoneRemovesCell: a wall blueprint leaves the zone be;
// the finished wall takes its cell out and a message says so.
func TestWallBuiltOverZoneRemovesCell(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	zoneCmd(g, command.DesignateStockpile, 2, 2, 5, 4, 0)
	id := zoneOwner(t, g, 3, 3).ID
	g.placeFrame(construction.Wall, 3, 3)
	if zoneOwner(t, g, 3, 3).ID != id {
		t.Fatal("a blueprint never takes a zoned cell")
	}
	finishFrame(g, g.layerFrameAt(3, 3, construction.LayerObject))
	if zoneOwner(t, g, 3, 3).ID != 0 {
		t.Fatal("the finished wall's cell is still zoned")
	}
	if v := zoneOwner(t, g, 2, 2); v.ID != id || len(v.Cells) != 11 {
		t.Fatalf("the rest of the stockpile stays: %+v", v)
	}
	if !hasMessage(g, "Stockpile shrank: wall built") {
		t.Fatalf("no shrink message: %+v", g.World().Messages)
	}
	// a non-blocking building (a floor) leaves a stockpile cell zoned
	placeBuilt(t, g, construction.Floor, construction.South, 4, 3)
	if zoneOwner(t, g, 4, 3).ID != id {
		t.Fatal("a floor under a stockpile keeps the cell")
	}
}

// TestFloorOverGrowZoneRemovesCell: a finished floor or ground turned to
// water takes a growing zone's cell.
func TestFloorOverGrowZoneRemovesCell(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	zoneCmd(g, command.DesignateGrowZone, 2, 2, 6, 4, 0)
	placeBuilt(t, g, construction.Floor, construction.South, 3, 3)
	if zoneOwner(t, g, 3, 3).ID != 0 {
		t.Fatal("the floored cell is still in the growing zone")
	}
	if !hasMessage(g, "Growing zone shrank: floor built") {
		t.Fatalf("no shrink message: %+v", g.World().Messages)
	}
	g.setTerrain(5, 3, terrain.ShallowWater)
	if zoneOwner(t, g, 5, 3).ID != 0 {
		t.Fatal("the flooded cell is still in the growing zone")
	}
	if !hasMessage(g, "Growing zone shrank: water") {
		t.Fatalf("no shrink message for the water: %+v", g.World().Messages)
	}
	// ground a zone still takes changes nothing
	n := len(g.World().Zones[0].Cells)
	g.setTerrain(4, 2, terrain.RichSoil)
	if len(g.World().Zones[0].Cells) != n {
		t.Fatal("rich soil keeps the cell")
	}
}

// TestZoneSplitsWhenCut: a wall line across a growing zone splits it in
// two; the bigger piece keeps the id and both keep the crop.
func TestZoneSplitsWhenCut(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	radish, _ := CropID("radish")
	zoneCmd(g, command.DesignateGrowZone, 2, 2, 10, 4, radish)
	id := zoneOwner(t, g, 2, 2).ID
	for y := 2; y <= 4; y++ {
		if !g.BuildNow("wall", "wood", "south", 4, y) {
			t.Fatalf("build wall at 4,%d", y)
		}
	}
	left, right := zoneOwner(t, g, 2, 3), zoneOwner(t, g, 8, 3)
	if left.ID == 0 || right.ID == 0 || left.ID == right.ID {
		t.Fatalf("not split: %+v %+v", left, right)
	}
	if right.ID != id || len(right.Cells) != 18 || len(left.Cells) != 6 {
		t.Fatalf("the bigger piece keeps the id: left %d cells id %d, right %d cells id %d (was %d)",
			len(left.Cells), left.ID, len(right.Cells), right.ID, id)
	}
	if left.Crop != "radish" || right.Crop != "radish" {
		t.Fatal("both pieces keep the crop")
	}
}

// TestDumpingZoneDefaults: the dumping stockpile tool paints a dump that
// takes corpses (STO-2: human ones and bones too) and stone chunks only,
// rotten ones included, on stockpile ground.
func TestDumpingZoneDefaults(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	zoneCmd(g, command.DesignateStockpile, 2, 2, 4, 4, StockpileVariantDump)
	v := zoneOwner(t, g, 3, 3)
	if v.Kind != ZoneDump || !slices.Contains(v.Filter, "Corpses") || !slices.Contains(v.Filter, "Chunks") ||
		!slices.Contains(v.Special, "allow rotten") || v.Priority != "Normal" {
		t.Fatalf("dump %+v", v)
	}
	z, _ := g.zones.Zone(zone.ID(v.ID))
	for k, want := range map[item.Kind]bool{
		item.GraniteChunk: true, item.CorpseAnimal: true, item.CorpseHuman: true, item.Bones: true,
		item.Wood: false, item.Steel: false, item.GraniteBlocks: false, item.Meal: false,
	} {
		if allows(z, k) != want {
			t.Errorf("dump takes %v: %v, want %v", k, !want, want)
		}
	}
	// a plain stockpile beside it stays apart and keeps its own filters
	zoneCmd(g, command.DesignateStockpile, 5, 2, 6, 4, 0)
	if s := zoneOwner(t, g, 5, 3); s.Kind != ZoneStockpile || s.ID == v.ID || !slices.Contains(s.Filter, "Foods") {
		t.Fatalf("stockpile beside a dump: %+v", s)
	}
	// a chunk goes to the dump; wood to the stockpile
	if x, _, ok := g.stockpileCell(item.Stack{Kind: item.GraniteChunk}, 9, 3); !ok || x > 6 {
		t.Fatal("no cell for a chunk")
	}
	if x, _, ok := g.stockpileCell(item.Stack{Kind: item.Wood}, 1, 3); !ok || x < 5 {
		t.Fatalf("wood goes to the stockpile, not the dump (x=%d)", x)
	}
	if g.ZoneRefusal(ZoneDump, 0, 3) != ZoneRefuseEdge {
		t.Fatal("dumps follow the zone rules")
	}
}

// TestHaulPrefersHigherPriority: the hauler fills the Critical stockpile,
// though a Normal one is nearer.
func TestHaulPrefersHigherPriority(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 8})
	zoneCmd(g, command.DesignateStockpile, 4, 2, 5, 3, 0)
	zoneCmd(g, command.DesignateStockpile, 18, 2, 19, 3, 0)
	far := zoneOwner(t, g, 18, 2).ID
	g.Enqueue(command.Command{Kind: command.SetZonePriority, Target: uint64(far), Index: 0}) // Critical
	g.ApplyPending()
	if v := zoneOwner(t, g, 18, 2); v.Priority != "Critical" {
		t.Fatalf("priority %q", v.Priority)
	}
	if x, _, ok := g.stockpileCell(item.Stack{Kind: item.Wood}, 6, 3); !ok || x < 18 {
		t.Fatalf("the critical stockpile first, even far: x=%d", x)
	}
	if _, err := g.SpawnNamedPawn(2, 4, "Hauler"); err != nil {
		t.Fatal(err)
	}
	sid, _ := g.SpawnStack(item.Wood, 20, 3, 5)
	for i := 0; i < 200; i++ {
		g.Step(20)
		if st, ok := g.stacks.Get(sid); ok && st.Carrier == 0 && st.X >= 18 {
			return
		}
	}
	st, _ := g.stacks.Get(sid)
	t.Fatalf("the wood never reached the critical stockpile: %+v", st)
}

// TestRehaulToHigherPriorityNoFlap: a stack in a Low stockpile is carried
// up to a Critical one with room, once, and never back; a stack just set
// down below the best zone waits the minimum stay first.
func TestRehaulToHigherPriorityNoFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 8})
	zoneCmd(g, command.DesignateStockpile, 3, 2, 4, 3, 0)
	zoneCmd(g, command.DesignateStockpile, 18, 2, 19, 3, 0)
	low, high := zoneOwner(t, g, 3, 2).ID, zoneOwner(t, g, 18, 2).ID
	g.Enqueue(command.Command{Kind: command.SetZonePriority, Target: uint64(low), Index: 4})  // Low
	g.Enqueue(command.Command{Kind: command.SetZonePriority, Target: uint64(high), Index: 0}) // Critical
	g.ApplyPending()
	sid, _ := g.SpawnStack(item.Wood, 20, 3, 2)
	// just set down below the best zone: it stays the minimum stay
	g.noteStored(3, 2)
	var prio haulPrio
	st, _ := g.stacks.Get(sid)
	if !g.staysStored(sid, &st, &prio) {
		t.Fatal("a fresh stack in a lower zone waits the minimum stay")
	}
	if _, err := g.SpawnNamedPawn(6, 4, "Hauler"); err != nil {
		t.Fatal(err)
	}
	g.Step(rehaulMinStay / 2)
	if st, _ := g.stacks.Get(sid); st.X != 3 || st.Y != 2 || st.Carrier != 0 {
		t.Fatalf("moved during the minimum stay: %+v", st)
	}
	// once the stay is over it goes up, and never back
	moves, lastX, arrived := 0, 3, false
	for i := 0; i < 600; i++ {
		g.Step(10)
		st, ok := g.stacks.Get(sid)
		if !ok {
			t.Fatal("the stack vanished")
		}
		if st.Carrier == 0 && st.X != lastX {
			moves++
			lastX = st.X
		}
		if st.Carrier == 0 && st.X >= 18 {
			arrived = true
		}
		if arrived && (st.Carrier != 0 || st.X < 18) {
			t.Fatalf("the stack left the critical stockpile again: %+v", st)
		}
	}
	if !arrived || moves != 1 {
		t.Fatalf("arrived %v after %d moves, want one move up", arrived, moves)
	}
	// equal priorities never trade stacks
	g.Enqueue(command.Command{Kind: command.SetZonePriority, Target: uint64(low), Index: 0})
	g.ApplyPending()
	g.Step(rehaulMinStay * 2)
	if st, _ := g.stacks.Get(sid); st.X < 18 || st.Carrier != 0 {
		t.Fatalf("moved between zones of one priority: %+v", st)
	}
}

// TestSetZonePriorityIgnoresBadTargets: grow zones and bad indexes are
// ignored.
func TestSetZonePriorityIgnoresBadTargets(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	zoneCmd(g, command.DesignateGrowZone, 2, 2, 3, 3, 0)
	zoneCmd(g, command.DesignateStockpile, 6, 2, 7, 3, 0)
	gz, sp := zoneOwner(t, g, 2, 2).ID, zoneOwner(t, g, 6, 2).ID
	for _, c := range []command.Command{
		{Kind: command.SetZonePriority, Target: uint64(gz), Index: 0},
		{Kind: command.SetZonePriority, Target: uint64(sp), Index: -1},
		{Kind: command.SetZonePriority, Target: uint64(sp), Index: 5},
		{Kind: command.SetZonePriority, Target: 999, Index: 0},
	} {
		g.Enqueue(c)
	}
	g.ApplyPending()
	if z, _ := g.zones.Zone(zone.ID(gz)); z.Priority != zone.PriorityNormal {
		t.Fatal("a growing zone has no priority")
	}
	if v := zoneOwner(t, g, 6, 2); v.Priority != "Normal" {
		t.Fatalf("bad index changed the priority: %q", v.Priority)
	}
	if len(StoragePriorities()) != 5 || StoragePriorities()[0] != "Critical" || StoragePriorities()[4] != "Low" {
		t.Fatalf("priorities %v", StoragePriorities())
	}
}

// TestSowOnlyInGrowingTemperature: growers sow only while the growing
// temperature is above 0 and below 50 °C.
func TestSowOnlyInGrowingTemperature(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	hut(t, g, 1, 1, 7, 7)
	g.ensureRooms()
	r := g.roomAt(4, 4)
	if r < 0 || r >= len(g.roomTemps) {
		t.Fatal("fixture: no room")
	}
	for _, c := range []struct {
		temp int
		sow  bool
	}{{-50, false}, {0, false}, {1, true}, {250, true}, {579, true}, {580, false}, {600, false}} { // FOOD-2: 0-58 °C (wiki: Plants)
		g.roomTemps[r] = temperature.Temp(c.temp)
		if got := g.growingSeason(4, 4); got != c.sow {
			t.Errorf("room at %d tenths °C: sow %v, want %v", c.temp, got, c.sow)
		}
	}
}

// TestZoneSaveRoundTripPriorityAndDump: priority, dump filters and the
// minimum stays survive a save.
func TestZoneSaveRoundTripPriorityAndDump(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	zoneCmd(g, command.DesignateStockpile, 2, 2, 3, 3, StockpileVariantDump)
	zoneCmd(g, command.DesignateStockpile, 8, 2, 9, 3, 0)
	dump := zoneOwner(t, g, 2, 2).ID
	g.Enqueue(command.Command{Kind: command.SetZonePriority, Target: uint64(dump), Index: 4})
	g.ApplyPending()
	sid, _ := g.SpawnStack(item.Wood, 10, 8, 2)
	g.haul.storedAt = map[entity.EntityID]uint64{sid: 7}
	var buf bytes.Buffer
	if err := g.Save(&buf, "zones"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	v := zoneOwner(t, h, 2, 2)
	if v.Kind != ZoneDump || v.Priority != "Low" || !slices.Contains(v.Filter, "Corpses") || !slices.Contains(v.Filter, "Chunks") {
		t.Fatalf("dump after load %+v", v)
	}
	if h.haul.storedAt[sid] != 7 {
		t.Fatal("minimum stays lost in the save")
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after the round trip")
	}
}

// TestLowOnlyStorageStays: with Low the best priority there is, a stored
// stack stays put.
func TestLowOnlyStorageStays(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	zoneCmd(g, command.DesignateStockpile, 2, 2, 3, 3, 0)
	g.Enqueue(command.Command{Kind: command.SetZonePriority, Target: uint64(zoneOwner(t, g, 2, 2).ID), Index: 4})
	g.ApplyPending()
	sid, _ := g.SpawnStack(item.Wood, 10, 2, 2)
	var prio haulPrio
	st, _ := g.stacks.Get(sid)
	if !g.staysStored(sid, &st, &prio) {
		t.Fatal("a stack in the only (Low) stockpile must stay")
	}
	if top, ok := prio.topFor(g, item.Wood); !ok || top != zone.PriorityLow {
		t.Fatalf("top = %v %v, want Low", top, ok)
	}
}
