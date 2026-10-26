package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// PLAY-020 zone management: sowing and cutting toggles, the crop, Expand,
// Remove, Delete, Rename and Hide, saves and the state hash.

// sowSpan is long enough for Ada (at 1,1 on clearGame's map) to walk to
// and sow a handful of cells.
const sowSpan = TicksPerCell*40 + 4*SowWork + 200

// zoneID is the id of the zone on tile (x, y).
func zoneID(t *testing.T, g *Game, x, y int) uint64 {
	t.Helper()
	v := zoneOwner(t, g, x, y)
	if v.ID == 0 {
		t.Fatalf("no zone on %d,%d", x, y)
	}
	return uint64(v.ID)
}

// zcmd applies a zone command with a flag on zone id.
func zcmd(g *Game, k command.Kind, id uint64, on bool) {
	g.Enqueue(command.Command{Kind: k, Target: id, Enabled: on})
	g.ApplyPending()
}

// zrect applies Expand or Remove over x1,y1..x2,y2 on zone id.
func zrect(g *Game, k command.Kind, id uint64, x1, y1, x2, y2 int) {
	g.Enqueue(command.Command{Kind: k, Target: id, X: x1, Y: y1, X2: x2, Y2: y2})
	g.ApplyPending()
}

// zoneAndApply draws a grow zone and applies the command.
func zoneAndApply(t *testing.T, g *Game, crop string, x1, y1, x2, y2 int) uint64 {
	t.Helper()
	drawZone(t, g, crop, x1, y1, x2, y2)
	g.ApplyPending()
	return zoneID(t, g, x1, y1)
}

// soil makes x1,y1..x2,y2 plain fertile ground.
func soil(g *Game, x1, y1, x2, y2 int) {
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			g.setTerrain(x, y, terrain.Soil)
		}
	}
}

// sownIn counts the zone-sown plants of species s on x1,y1..x2,y2.
func sownIn(g *Game, s plant.Species, x1, y1, x2, y2 int) int {
	n := 0
	for _, pid := range g.plantIDs {
		if p, ok := g.plants.Get(pid); ok && p.Zone && p.Species == s && p.X >= x1 && p.X <= x2 && p.Y >= y1 && p.Y <= y2 {
			n++
		}
	}
	return n
}

func totalFood(g *Game) int {
	n := 0
	for _, q := range g.World().FoodByKind {
		n += q
	}
	return n
}

// ---- sowing ----

func TestSetZoneSowOffStopsSowingUntilItIsOnAgain(t *testing.T) {
	g, _ := clearGame(t)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 3)
	zcmd(g, command.SetZoneSow, id, false)
	if v := zoneOwner(t, g, 4, 2); !v.NoSow || v.NoCut || v.Hidden {
		t.Fatalf("view %+v, want only NoSow", v)
	}
	g.Step(sowSpan)
	if n := sownIn(g, plant.Potato, 4, 2, 5, 3); n != 0 {
		t.Fatalf("%d cells sown with sowing off", n)
	}
	zcmd(g, command.SetZoneSow, id, true)
	if zoneOwner(t, g, 4, 2).NoSow {
		t.Fatal("NoSow still set after allowing sowing")
	}
	g.Step(sowSpan)
	if n := sownIn(g, plant.Potato, 4, 2, 5, 3); n != 4 {
		t.Fatalf("%d of 4 cells sown once sowing is allowed", n)
	}
}

func TestSowingOffOnlyAffectsThatZone(t *testing.T) {
	g, _ := clearGame(t)
	off := zoneAndApply(t, g, "potato", 4, 1, 5, 1)
	zoneAndApply(t, g, "radish", 4, 4, 5, 4)
	zcmd(g, command.SetZoneSow, off, false)
	g.Step(sowSpan)
	if sownIn(g, plant.Potato, 4, 1, 5, 1) != 0 || sownIn(g, plant.Radish, 4, 4, 5, 4) != 2 {
		t.Fatal("the other zone keeps being sown")
	}
}

func TestSetZoneSowOffCancelsAnInFlightSow(t *testing.T) {
	g, ada := clearGame(t)
	id := zoneAndApply(t, g, "potato", 10, 4, 10, 4)
	for i := 0; i < 600 && jobOf(g, ada).Kind != job.Sow; i++ {
		g.Step(1)
	}
	if j := jobOf(g, ada); j.Kind != job.Sow || j.TX != 10 || j.TY != 4 {
		t.Fatalf("Ada never set out to sow: %+v", j)
	}
	if _, taken := g.reserve.TileOwner(10, 4); !taken {
		t.Fatal("the cell should be claimed while Ada is on her way")
	}
	zcmd(g, command.SetZoneSow, id, false)
	if j := jobOf(g, ada); j.Kind == job.Sow {
		t.Fatalf("the sow job survived: %+v", j)
	}
	if _, taken := g.reserve.TileOwner(10, 4); taken {
		t.Fatal("the cancelled sow still claims its cell")
	}
	g.Step(sowSpan)
	if g.plantAt(10, 4) != 0 {
		t.Fatal("the cell was sown after sowing was turned off")
	}
}

// doSow itself refuses once sowing is off, even if nothing interrupted the job.
func TestDoSowFailsWhenSowingIsOff(t *testing.T) {
	g, ada := clearGame(t)
	id := zoneAndApply(t, g, "potato", 4, 2, 4, 2)
	j := job.Record{Kind: job.Sow, State: job.Do, TX: 4, TY: 2, WorkLeft: 1}
	g.jobs.Set(ada, j)
	g.zones.Update(zone.ID(id), func(z *zone.Zone) { z.NoSow = true }) // not through the command: no interrupt
	g.doSow(ada, j)
	if got := jobOf(g, ada); got.State != job.Fail || got.Fail != job.TargetGone {
		t.Fatalf("job after doSow: %+v, want Fail/TargetGone", got)
	}
	if g.plantAt(4, 2) != 0 {
		t.Fatal("the cell was sown")
	}
}

func TestForcedSowAndSowableRefuseASowingOffZone(t *testing.T) {
	g, ada := clearGame(t)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 2)
	if _, why := g.forcedJob(ada, command.OrderSow, 4, 2, construction.LayerObject); why != "" {
		t.Fatalf("forced sow refused before the toggle: %q", why)
	}
	zcmd(g, command.SetZoneSow, id, false)
	if g.sowable(4, 2) {
		t.Fatal("sowable in a zone with sowing off")
	}
	if _, why := g.forcedJob(ada, command.OrderSow, 4, 2, construction.LayerObject); why != ReasonNotSowable {
		t.Fatalf("forced sow: reason %q, want %q", why, ReasonNotSowable)
	}
	disabled := false
	for _, o := range g.OrdersAt(ada, 4, 2) {
		if o.Cmd.Order == command.OrderSow {
			disabled = o.Disabled
		}
	}
	if !disabled {
		t.Fatal("the Sow menu item is still offered")
	}
	zcmd(g, command.SetZoneSow, id, true)
	if !g.sowable(4, 2) {
		t.Fatal("not sowable once allowed again")
	}
}

func TestSowingOffStopsTreeZonesReplanting(t *testing.T) {
	g, _ := treeZoneGame(t)
	g.Step(1)
	id := zoneID(t, g, 4, 2)
	zcmd(g, command.SetZoneSow, id, false)
	g.Step(TicksPerCell*40 + 4*plant.DefOf(plant.Tree).SowWork)
	if n, _ := treesAt(g); n != 0 {
		t.Fatalf("%d saplings planted with sowing off", n)
	}
	zcmd(g, command.SetZoneSow, id, true)
	g.Step(TicksPerCell*40 + 4*plant.DefOf(plant.Tree).SowWork)
	if n, z := treesAt(g); n != 4 || z != 4 {
		t.Fatalf("saplings = %d (zone %d), want 4", n, z)
	}
}

// Harvesting and the ripe mark go on whichever toggle is off.
func TestToggledOffZonesStillHarvestRipeCrops(t *testing.T) {
	for _, tc := range []struct {
		name        string
		sow, cut    bool // allowed
		wantResowed bool
	}{
		{"sowing off", false, true, false},
		{"cutting off", true, false, true},
		{"both off", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := clearGame(t)
			orig := putPlant(t, g, plant.Potato, 4, 2, plant.DefOf(plant.Potato).GrowTicks, true)
			id := zoneAndApply(t, g, "potato", 4, 2, 4, 2)
			zcmd(g, command.SetZoneSow, id, tc.sow)
			zcmd(g, command.SetZoneCut, id, tc.cut)
			if p, _ := g.plants.Get(orig); !p.Designated {
				t.Fatal("the ripe crop under a new zone was not marked for harvest")
			}
			for i := 0; i < 2500 && alive(g, orig); i++ {
				g.Step(1)
			}
			if alive(g, orig) || totalFood(g) < 1 {
				t.Fatalf("ripe crop not harvested: alive %v, food %v", alive(g, orig), g.World().FoodByKind)
			}
			g.Step(sowSpan)
			if resown := g.plantAt(4, 2) != 0; resown != tc.wantResowed {
				t.Fatalf("cell resown = %v, want %v", resown, tc.wantResowed)
			}
		})
	}
}

// ---- cutting ----

func TestSetZoneCutOffLeavesWildPlantsAndTheirCellsUnsown(t *testing.T) {
	g, _ := clearGame(t)
	moss := putPlant(t, g, plant.Moss, 4, 2, 0, false)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 2)
	zcmd(g, command.SetZoneCut, id, false)
	if v := zoneOwner(t, g, 4, 2); !v.NoCut || v.NoSow {
		t.Fatalf("view %+v, want only NoCut", v)
	}
	g.Step(sowSpan + 600)
	if !alive(g, moss) || g.plantAt(4, 2) != moss {
		t.Fatal("the wild plant was cut with cutting off")
	}
	if sownIn(g, plant.Potato, 5, 2, 5, 2) != 1 {
		t.Fatal("the free cell is still sown")
	}
	zcmd(g, command.SetZoneCut, id, true)
	for i := 0; i < 1500 && sownIn(g, plant.Potato, 4, 2, 4, 2) == 0; i++ {
		g.Step(1)
	}
	if alive(g, moss) || sownIn(g, plant.Potato, 4, 2, 4, 2) != 1 {
		t.Fatal("with cutting allowed again the moss is cleared and the cell sown")
	}
}

func TestSetZoneCutOffStopsZoneTreeChopping(t *testing.T) {
	g, pid := ripeZoneTree(t)
	zcmd(g, command.SetZoneCut, zoneID(t, g, 4, 2), false)
	span := TicksPerCell*40 + plant.DefOf(plant.Tree).CutWork + 60
	g.Step(span)
	if !alive(g, pid) {
		t.Fatal("the mature zone tree was chopped with cutting off")
	}
	zcmd(g, command.SetZoneCut, zoneID(t, g, 4, 2), true)
	g.Step(span)
	if alive(g, pid) {
		t.Fatal("the mature zone tree should be chopped once cutting is allowed")
	}
}

func TestSetZoneCutOffCancelsAnInFlightClear(t *testing.T) {
	g, ada := clearGame(t)
	moss := putPlant(t, g, plant.Moss, 10, 4, 0, false)
	id := zoneAndApply(t, g, "potato", 10, 4, 10, 4)
	for i := 0; i < 600 && jobOf(g, ada).Kind != job.Cut; i++ {
		g.Step(1)
	}
	if j := jobOf(g, ada); j.Kind != job.Cut || j.Target != moss {
		t.Fatalf("Ada never set out to clear the moss: %+v", j)
	}
	zcmd(g, command.SetZoneCut, id, false)
	if j := jobOf(g, ada); j.Kind == job.Cut {
		t.Fatalf("the clearing job survived: %+v", j)
	}
	g.Step(sowSpan)
	if !alive(g, moss) {
		t.Fatal("the moss was cut")
	}
}

// A plant the player designated for cutting is not the zone's business:
// a cut already under way on it goes on when cutting is turned off.
func TestSetZoneCutOffKeepsDesignatedCuts(t *testing.T) {
	g, ada := clearGame(t)
	moss := putPlant(t, g, plant.Moss, 10, 4, 0, false)
	id := zoneAndApply(t, g, "potato", 10, 4, 10, 4)
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: 10, Y: 4, X2: 10, Y2: 4})
	for i := 0; i < 600 && jobOf(g, ada).Kind != job.Cut; i++ {
		g.Step(1)
	}
	if j := jobOf(g, ada); j.Kind != job.Cut || j.Target != moss {
		t.Fatalf("Ada never set out to cut the marked moss: %+v", j)
	}
	zcmd(g, command.SetZoneCut, id, false)
	if j := jobOf(g, ada); j.Kind != job.Cut || j.Target != moss {
		t.Fatalf("the designated cut was cancelled: %+v", j)
	}
	for i := 0; i < 1500 && alive(g, moss); i++ {
		g.Step(1)
	}
	if alive(g, moss) {
		t.Fatal("the designated cut never finished")
	}
}

// Sowing off alone does not stop the clearing; the cells just stay empty.
func TestSowingOffStillClearsWrongPlants(t *testing.T) {
	g, _ := clearGame(t)
	moss := putPlant(t, g, plant.Moss, 4, 2, 0, false)
	id := zoneAndApply(t, g, "potato", 4, 2, 4, 2)
	zcmd(g, command.SetZoneSow, id, false)
	g.Step(sowSpan)
	if alive(g, moss) || g.plantAt(4, 2) != 0 {
		t.Fatalf("moss alive %v, cell holds %d: want the moss cut and nothing sown", alive(g, moss), g.plantAt(4, 2))
	}
}

// The mature trees of a deleted tree zone are ordinary trees again.
func TestZoneTreesOutsideAnyZoneAreNotChopped(t *testing.T) {
	g, pid := ripeZoneTree(t)
	zcmd(g, command.DeleteZone, zoneID(t, g, 4, 2), false)
	g.Step(TicksPerCell*40 + plant.DefOf(plant.Tree).CutWork + 60)
	if !alive(g, pid) {
		t.Fatal("a tree left by a deleted zone was chopped")
	}
}

// ---- crop ----

// A crop research has not unlocked cannot be picked, as for a new zone.
func TestSetZoneCropRefusesAResearchLockedCrop(t *testing.T) {
	g, _ := clearGame(t)
	g.ResetResearch() // New leaves every project done; a fresh colony knows none
	id := zoneAndApply(t, g, "potato", 1, 4, 2, 4)
	healroot, ok := ZoneSpeciesID("healroot")
	if !ok || !g.cropLocked(plant.Species(healroot)) {
		t.Fatalf("healroot should be a zone crop that research locks (ok %v)", ok)
	}
	g.Enqueue(command.Command{Kind: command.SetZoneCrop, Target: id, Build: healroot})
	g.ApplyPending()
	if v := zoneOwner(t, g, 1, 4); v.Crop != "potato" {
		t.Fatalf("a locked crop was set: %q", v.Crop)
	}
}

func TestSetZoneCropMakesGrowersClearAndResow(t *testing.T) {
	g, _ := clearGame(t)
	old := []entity.EntityID{
		putPlant(t, g, plant.Potato, 4, 2, 100, true),
		putPlant(t, g, plant.Potato, 5, 2, 100, true),
	}
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 2)
	radish, _ := CropID("radish")
	g.Enqueue(command.Command{Kind: command.SetZoneCrop, Target: id, Build: radish})
	g.ApplyPending()
	if v := zoneOwner(t, g, 4, 2); v.Crop != "radish" || len(v.Cells) != 2 {
		t.Fatalf("zone %+v", v)
	}
	g.Step(2 * sowSpan)
	for _, pid := range old {
		if alive(g, pid) {
			t.Fatalf("old potato %d still stands", pid)
		}
	}
	if sownIn(g, plant.Radish, 4, 2, 5, 2) != 2 {
		t.Fatal("the zone was not resown with radish")
	}
}

func TestSetZoneCropWithCuttingOffLeavesTheOldPlants(t *testing.T) {
	g, _ := clearGame(t)
	old := putPlant(t, g, plant.Potato, 4, 2, 100, true)
	id := zoneAndApply(t, g, "potato", 4, 2, 4, 2)
	zcmd(g, command.SetZoneCut, id, false)
	radish, _ := CropID("radish")
	g.Enqueue(command.Command{Kind: command.SetZoneCrop, Target: id, Build: radish})
	g.Step(sowSpan)
	if !alive(g, old) || g.plantAt(4, 2) != old {
		t.Fatal("the old plant should stay with cutting off")
	}
}

func TestSetZoneCropIgnoresWhatDoesNotApply(t *testing.T) {
	g, _ := clearGame(t)
	grow := zoneAndApply(t, g, "potato", 1, 4, 2, 4)
	zoneCmd(g, command.DesignateStockpile, 8, 1, 9, 2, 0)
	stock := zoneID(t, g, 8, 1)
	radish, _ := CropID("radish")
	crop := func(id uint64, b uint8) {
		g.Enqueue(command.Command{Kind: command.SetZoneCrop, Target: id, Build: b})
		g.ApplyPending()
	}
	crop(stock, radish) // not a grow zone
	crop(99, radish)    // unknown zone
	crop(grow, uint8(plant.BerryBush))
	crop(grow, uint8(plant.Moss))
	crop(grow, 250) // not a species
	if v := zoneOwner(t, g, 1, 4); v.Crop != "potato" {
		t.Fatalf("crop changed to %q by a command that does not apply", v.Crop)
	}
	if v := zoneOwner(t, g, 8, 1); v.Kind != ZoneStockpile || v.Crop != "" {
		t.Fatalf("stockpile %+v", v)
	}
	tree, _ := ZoneSpeciesID("tree")
	crop(grow, tree)
	if v := zoneOwner(t, g, 1, 4); v.Crop != "tree" {
		t.Fatalf("a tree zone is a valid crop: %q", v.Crop)
	}
}

func TestSetZoneCropMergesWithATouchingZoneOfThatCrop(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 1, 4, 4, 4)
	a := zoneAndApply(t, g, "potato", 1, 4, 2, 4)
	b := zoneAndApply(t, g, "radish", 3, 4, 4, 4)
	if a == b {
		t.Fatal("different crops stay apart")
	}
	potato, _ := CropID("potato")
	g.Enqueue(command.Command{Kind: command.SetZoneCrop, Target: b, Build: potato})
	g.ApplyPending()
	zs := zonesOf(g.World(), ZoneGrow)
	if len(zs) != 1 || len(zs[0].Cells) != 4 || zs[0].ID != uint16(a) || zs[0].Crop != "potato" {
		t.Fatalf("zones %+v, want one potato zone under the lower id", zs)
	}
}

// ---- expand ----

func TestExpandZoneAddsAdjacentCellsKeepingSettingsAndName(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 4, 2, 8, 3)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 3)
	zcmd(g, command.SetZoneSow, id, false)
	zcmd(g, command.SetZoneHidden, id, true)
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: id, Name: "Spuds"})
	g.ApplyPending()
	zrect(g, command.ExpandZone, id, 6, 3, 8, 2) // corners given in any order
	zs := zonesOf(g.World(), ZoneGrow)
	if len(zs) != 1 {
		t.Fatalf("zones %+v", zs)
	}
	z := zs[0]
	if z.ID != uint16(id) || len(z.Cells) != 4+6 || !inZone(z, 8, 2) || z.MaxX != 8 {
		t.Fatalf("zone %+v", z)
	}
	if !z.NoSow || z.NoCut || !z.Hidden || z.Name != "Spuds" || z.Crop != "potato" {
		t.Fatalf("settings changed: %+v", z)
	}
}

func TestExpandZoneDetachedBlockBecomesANewZoneWithTheSameSettings(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 1, 1, 9, 4)
	id := zoneAndApply(t, g, "potato", 1, 1, 2, 1)
	zcmd(g, command.SetZoneCut, id, false)
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: id, Name: "Spuds"})
	g.ApplyPending()
	zrect(g, command.ExpandZone, id, 8, 3, 9, 4)
	zs := zonesOf(g.World(), ZoneGrow)
	if len(zs) != 2 || len(zs[0].Cells) != 2 || len(zs[1].Cells) != 4 || zs[1].ID == zs[0].ID {
		t.Fatalf("zones %+v", zs)
	}
	if !zs[1].NoCut || zs[1].NoSow || zs[1].Name != "Spuds" || zs[1].Crop != "potato" {
		t.Fatalf("the new zone copies the settings: %+v", zs[1])
	}
}

func TestExpandZoneSkipsInvalidCells(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 4, 2, 8, 3)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 2)
	g.setTerrain(6, 2, terrain.Rock)
	g.setTerrain(7, 2, terrain.Stone) // rough stone: not fertile
	zoneCmd(g, command.DesignateStockpile, 4, 3, 4, 3, 0)
	zrect(g, command.ExpandZone, id, 4, 3, 8, 3) // 4,3 is a stockpile
	zrect(g, command.ExpandZone, id, 6, 2, 7, 2) // rock and rough stone
	zrect(g, command.ExpandZone, id, -5, -5, -1, -1)
	zrect(g, command.ExpandZone, id, 40, 40, 50, 50)
	z := zoneOwner(t, g, 4, 2)
	if inZone(z, 6, 2) || inZone(z, 7, 2) || inZone(z, 4, 3) {
		t.Fatalf("invalid cells joined the zone: %+v", z)
	}
	if len(z.Cells) != 6 || !inZone(z, 5, 3) || !inZone(z, 8, 3) || len(zonesOf(g.World(), ZoneGrow)) != 1 {
		t.Fatalf("the valid cells of row 3 should join through 5,3: %+v", z)
	}
	if st := zoneOwner(t, g, 4, 3); st.Kind != ZoneStockpile || len(st.Cells) != 1 {
		t.Fatalf("the stockpile cell moved: %+v", st)
	}
}

func TestExpandZoneAppliesToStockpilesAndIgnoresUnknownIDs(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 4, 2, 6, 3)
	zoneCmd(g, command.DesignateStockpile, 4, 2, 5, 2, 0)
	id := zoneID(t, g, 4, 2)
	g.setTerrain(6, 2, terrain.Stone)
	zrect(g, command.ExpandZone, id, 6, 2, 6, 3) // stockpiles may sit on rough stone
	if v := zoneOwner(t, g, 4, 2); len(v.Cells) != 4 || !inZone(v, 6, 2) {
		t.Fatalf("stockpile %+v", v)
	}
	before := len(g.World().Zones)
	zrect(g, command.ExpandZone, 99, 0, 0, 3, 3)
	if len(g.World().Zones) != before || zoneOwner(t, g, 0, 0).ID != 0 {
		t.Fatal("an unknown zone id painted something")
	}
}

func TestExpandZoneMarksRipePlantsItCovers(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 4, 2, 6, 2)
	id := zoneAndApply(t, g, "potato", 4, 2, 4, 2)
	ripe := putPlant(t, g, plant.BerryBush, 5, 2, plant.DefOf(plant.BerryBush).GrowTicks, false)
	young := putPlant(t, g, plant.Corn, 6, 2, 100, true)
	zrect(g, command.ExpandZone, id, 5, 2, 6, 2)
	marked := func(pid entity.EntityID) bool { p, _ := g.plants.Get(pid); return p.Designated }
	if !marked(ripe) || marked(young) {
		t.Fatalf("ripe bush marked %v, young corn marked %v", marked(ripe), marked(young))
	}
}

// ---- remove cells ----

func TestRemoveZoneCellsTouchesOnlyThatZone(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 2, 2, 8, 3)
	grow := zoneAndApply(t, g, "potato", 2, 2, 5, 3)
	zoneCmd(g, command.DesignateStockpile, 6, 2, 8, 3, 0)
	zrect(g, command.RemoveZoneCells, grow, 5, 2, 8, 3) // spans the stockpile
	gz := zoneOwner(t, g, 2, 2)
	if len(gz.Cells) != 6 || inZone(gz, 5, 2) || inZone(gz, 5, 3) {
		t.Fatalf("grow zone %+v", gz)
	}
	if st := zoneOwner(t, g, 7, 2); st.Kind != ZoneStockpile || len(st.Cells) != 6 {
		t.Fatalf("the neighbouring stockpile lost cells: %+v", st)
	}
}

func TestRemoveZoneCellsSplitsKeepingSettingsAndTheBiggestPieceKeepsTheID(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 1, 2, 9, 3)
	id := zoneAndApply(t, g, "potato", 1, 2, 9, 3)
	zcmd(g, command.SetZoneSow, id, false)
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: id, Name: "Long"})
	g.ApplyPending()
	zrect(g, command.RemoveZoneCells, id, 3, 3, 3, 2) // a column: west 2x2, east 6x2
	zs := zonesOf(g.World(), ZoneGrow)
	if len(zs) != 2 {
		t.Fatalf("zones %+v", zs)
	}
	if zs[0].ID != uint16(id) || len(zs[0].Cells) != 12 || !inZone(zs[0], 9, 3) {
		t.Fatalf("the bigger piece keeps the id: %+v", zs[0])
	}
	for _, z := range zs {
		if !z.NoSow || z.NoCut || z.Name != "Long" || z.Crop != "potato" {
			t.Fatalf("a piece lost its settings: %+v", z)
		}
	}
	if len(zs[1].Cells) != 4 || !inZone(zs[1], 1, 2) {
		t.Fatalf("west piece %+v", zs[1])
	}
}

func TestRemoveZoneCellsDeletesAnEmptiedZone(t *testing.T) {
	g, _ := clearGame(t)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 3)
	zrect(g, command.RemoveZoneCells, id, 0, 0, 11, 5)
	if len(g.World().Zones) != 0 {
		t.Fatalf("zones %+v", g.World().Zones)
	}
	zrect(g, command.RemoveZoneCells, id, 0, 0, 11, 5) // already gone: nothing happens
	zrect(g, command.RemoveZoneCells, 77, 0, 0, 11, 5)
}

// ---- delete ----

func TestDeleteZoneRemovesItAndFreesItsCells(t *testing.T) {
	g, _ := clearGame(t)
	a := zoneAndApply(t, g, "potato", 1, 4, 2, 4)
	zoneCmd(g, command.DesignateStockpile, 8, 1, 9, 2, 0)
	zcmd(g, command.DeleteZone, a, false)
	w := g.World()
	if len(w.Zones) != 1 || w.Zones[0].Kind != ZoneStockpile || zoneOwner(t, g, 1, 4).ID != 0 {
		t.Fatalf("zones %+v", w.Zones)
	}
	if !g.CanZoneAt(ZoneGrow, 1, 4) {
		t.Fatal("the freed cells can be zoned again")
	}
	zcmd(g, command.DeleteZone, a, false) // twice and unknown ids are harmless
	zcmd(g, command.DeleteZone, 500, false)
	if len(g.World().Zones) != 1 {
		t.Fatal("a stale delete removed something")
	}
}

// ---- rename and hide ----

func TestRenameZoneTrimsAndCapsTheName(t *testing.T) {
	g, _ := clearGame(t)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 3)
	rename := func(name string) string {
		g.Enqueue(command.Command{Kind: command.RenameZone, Target: id, Name: name})
		g.ApplyPending()
		return zoneOwner(t, g, 4, 2).Name
	}
	if got := rename("  Spud patch \t"); got != "Spud patch" {
		t.Fatalf("trimmed name %q", got)
	}
	long := strings.Repeat("a", 24)
	if got := rename(long + "bcd"); got != long {
		t.Fatalf("capped name %q", got)
	}
	if got := rename(strings.Repeat("é", 30)); got != strings.Repeat("é", 24) {
		t.Fatalf("a name is capped in characters, not bytes: %q", got)
	}
	if got := rename(strings.Repeat("a", 23) + " b"); got != strings.Repeat("a", 23) {
		t.Fatalf("a cut that ends in a space is trimmed: %q", got)
	}
	if got := rename("   "); got != "" {
		t.Fatalf("blank name %q, want the default (empty)", got)
	}
	rename("Back")
	if got := rename(""); got != "" {
		t.Fatalf("empty name %q", got)
	}
}

func TestRenameAndHideApplyToEveryZoneKind(t *testing.T) {
	g, _ := clearGame(t)
	zoneCmd(g, command.DesignateStockpile, 8, 1, 9, 2, 0)
	st := zoneID(t, g, 8, 1)
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: st, Name: "Store"})
	g.ApplyPending()
	zcmd(g, command.SetZoneHidden, st, true)
	if v := zoneOwner(t, g, 8, 1); v.Name != "Store" || !v.Hidden {
		t.Fatalf("stockpile %+v", v)
	}
	zcmd(g, command.SetZoneHidden, st, false)
	if zoneOwner(t, g, 8, 1).Hidden {
		t.Fatal("not shown again")
	}
	// unknown ids are ignored
	zcmd(g, command.SetZoneHidden, 99, true)
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: 99, Name: "x"})
	g.ApplyPending()
}

func TestSowAndCutTogglesIgnoreStockpilesAndUnknownIDs(t *testing.T) {
	g, _ := clearGame(t)
	zoneCmd(g, command.DesignateStockpile, 8, 1, 9, 2, 0)
	st := zoneID(t, g, 8, 1)
	for _, k := range []command.Kind{command.SetZoneSow, command.SetZoneCut} {
		zcmd(g, k, st, false)
		zcmd(g, k, 99, false)
	}
	if v := zoneOwner(t, g, 8, 1); v.NoSow || v.NoCut {
		t.Fatalf("stockpile took a grow zone setting: %+v", v)
	}
}

// ---- painting next to a configured zone ----

// Settings and names never decide whether zones merge: painting a zone of
// the same crop against one joins it, and the tiles take the zone's
// settings (as the lowest id's do when a drag bridges two zones).
func TestPaintingNextToAConfiguredZoneJoinsItAndTakesItsSettings(t *testing.T) {
	g, _ := clearGame(t)
	soil(g, 2, 2, 9, 3)
	id := zoneAndApply(t, g, "potato", 2, 2, 3, 3)
	zcmd(g, command.SetZoneSow, id, false)
	zcmd(g, command.SetZoneHidden, id, true)
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: id, Name: "Spuds"})
	g.ApplyPending()
	other := zoneAndApply(t, g, "potato", 8, 2, 9, 3) // apart: a zone with default settings
	if other == id {
		t.Fatal("apart: two zones")
	}
	zoneAndApply(t, g, "potato", 4, 2, 7, 3) // bridges both
	zs := zonesOf(g.World(), ZoneGrow)
	if len(zs) != 1 || len(zs[0].Cells) != 16 || zs[0].ID != uint16(id) {
		t.Fatalf("zones %+v, want one under the lower id", zs)
	}
	if !zs[0].NoSow || !zs[0].Hidden || zs[0].Name != "Spuds" || zs[0].NoCut {
		t.Fatalf("the merged zone should keep the lower id's settings: %+v", zs[0])
	}
}

// ---- saves and the state hash ----

func TestZoneSettingsSurviveSaveAndLoad(t *testing.T) {
	g, _ := clearGame(t)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 3)
	zcmd(g, command.SetZoneSow, id, false)
	zcmd(g, command.SetZoneCut, id, false)
	zcmd(g, command.SetZoneHidden, id, true)
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: id, Name: "Spuds"})
	g.ApplyPending()
	zoneCmd(g, command.DesignateStockpile, 8, 1, 9, 2, 0)
	var buf bytes.Buffer
	if err := g.Save(&buf, "zones"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	v := zoneOwner(t, h, 4, 2)
	if !v.NoSow || !v.NoCut || !v.Hidden || v.Name != "Spuds" || v.Crop != "potato" || len(v.Cells) != 4 {
		t.Fatalf("loaded zone %+v", v)
	}
	if st := zoneOwner(t, h, 8, 1); st.NoSow || st.NoCut || st.Hidden || st.Name != "" {
		t.Fatalf("the other zone picked up settings: %+v", st)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after a save round trip")
	}
	// and the loaded game still honours them
	h.Step(sowSpan)
	if sownIn(h, plant.Potato, 4, 2, 5, 3) != 0 {
		t.Fatal("the loaded zone sows with sowing off")
	}
}

// A zone with every setting at its default hashes as it did before the
// settings existed: toggling one on moves the hash, off again restores it.
func TestDefaultZoneSettingsDoNotMoveTheStateHash(t *testing.T) {
	g, _ := clearGame(t)
	id := zoneAndApply(t, g, "potato", 4, 2, 5, 3)
	base := g.StateHash()
	for _, k := range []command.Kind{command.SetZoneSow, command.SetZoneCut, command.SetZoneHidden} {
		zcmd(g, k, id, k == command.SetZoneHidden)
		zcmd(g, k, id, k != command.SetZoneHidden) // back to the defaults
		if got := g.StateHash(); got != base {
			t.Fatalf("hash moved after toggling %v back to its default", k)
		}
	}
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: id, Name: "x"})
	g.ApplyPending()
	if g.StateHash() == base {
		t.Fatal("a name does not change the hash")
	}
	g.Enqueue(command.Command{Kind: command.RenameZone, Target: id, Name: ""})
	g.ApplyPending()
	if g.StateHash() != base {
		t.Fatal("clearing the name did not restore the hash")
	}
	zcmd(g, command.SetZoneSow, id, false)
	if g.StateHash() == base {
		t.Fatal("NoSow does not change the hash")
	}
}
