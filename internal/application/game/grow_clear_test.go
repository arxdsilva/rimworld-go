package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// PLAY-017 §1 and §10: growers clear a zone cell before sowing it, ripe
// plants in a zone are harvested through their mark, and "Cancel harvest"
// sticks.

// clearGame is a 12x6 map with Ada at (1,1) and nothing planted yet.
func clearGame(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	for _, pid := range append([]entity.EntityID(nil), g.plantIDs...) {
		g.removePlant(pid) // a clean field: no generated plants
	}
	return g, id
}

// putPlant places a plant (replacing anything on the cell); zoneSown makes
// it look sown by a growing zone.
func putPlant(t *testing.T, g *Game, s plant.Species, x, y, growth int, zoneSown bool) entity.EntityID {
	t.Helper()
	if old := g.plantAt(x, y); old != 0 {
		g.removePlant(old)
	}
	pid, err := g.spawnPlant(s, x, y, growth)
	if err != nil {
		t.Fatalf("spawn %v at %d,%d: %v", s, x, y, err)
	}
	if zoneSown {
		p, _ := g.plants.Get(pid)
		p.Zone = true
		g.plants.Set(pid, p)
		g.notePlantMark(pid)
	}
	return pid
}

// drawZone queues a growing zone of the named crop over x1,y1..x2,y2.
func drawZone(t *testing.T, g *Game, crop string, x1, y1, x2, y2 int) {
	t.Helper()
	c, ok := ZoneSpeciesID(crop)
	if !ok {
		t.Fatalf("%q is not zone sowable", crop)
	}
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: x1, Y: y1, X2: x2, Y2: y2, Build: c})
}

func jobOf(g *Game, id entity.EntityID) job.Record {
	j, _ := g.jobs.Get(id)
	return j
}

func alive(g *Game, pid entity.EntityID) bool {
	_, ok := g.plants.Get(pid)
	return ok
}

// TestZoneClearsWrongPlantsThenSows: whatever stands on a potato-zone cell
// and is not the zone's crop is cut by a grower, and only then is the cell
// sown with potatoes.
func TestZoneClearsWrongPlantsThenSows(t *testing.T) {
	ripe := func(s plant.Species) int { return plant.DefOf(s).GrowTicks }
	cases := []struct {
		name     string
		species  plant.Species
		growth   int
		zoneSown bool
	}{
		{"moss (ground plant)", plant.Moss, 0, false},
		{"unripe berry bush", plant.BerryBush, 0, false},
		{"wild tree", plant.Tree, ripe(plant.Tree), false},
		{"unripe crop of another species", plant.Corn, 100, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := clearGame(t)
			orig := putPlant(t, g, tc.species, 4, 2, tc.growth, tc.zoneSown)
			drawZone(t, g, "potato", 4, 2, 4, 2)

			gone := false
			for i := 0; i < 3000 && !(gone && g.plantAt(4, 2) != 0); i++ { // FOOD-2: a tree takes 1,400 to cut (wiki)
				g.Step(1)
				if !gone && !alive(g, orig) {
					gone = true
					if g.plantAt(4, 2) != 0 {
						t.Fatalf("tick %d: the cell was sown in the same instant it was cut", i)
					}
				}
			}
			if !gone {
				t.Fatalf("the %s was never cut", tc.name)
			}
			pid := g.plantAt(4, 2)
			if pid == 0 {
				t.Fatalf("the cleared cell was never sown")
			}
			if p, _ := g.plants.Get(pid); p.Species != plant.Potato || !p.Zone {
				t.Fatalf("cell holds %+v, want a zone-sown potato", p)
			}
		})
	}
}

// TestZoneClearTargetsTheNearestWrongPlant: with two wrong plants the
// grower walks to the nearer one first, with the ordinary Cut job.
func TestZoneClearTargetsTheNearestWrongPlant(t *testing.T) {
	g, id := clearGame(t)
	far := putPlant(t, g, plant.Moss, 8, 2, 0, false)
	near := putPlant(t, g, plant.Moss, 5, 2, 0, false)
	drawZone(t, g, "potato", 5, 2, 8, 2)
	g.Step(1)
	g.interrupt(id, job.Cancelled)
	if !g.assignZoneClear(id) {
		t.Fatal("assignZoneClear found nothing to clear")
	}
	j := jobOf(g, id)
	if j.Kind != job.Cut || j.Target != near || j.TX != 5 || j.TY != 2 {
		t.Fatalf("job = %+v, want Cut of the near moss (%d), not the far one (%d)", j, near, far)
	}
	if want := plant.DefOf(plant.Moss).CutWork; j.WorkLeft != want {
		t.Fatalf("WorkLeft = %d, want the plant's CutWork %d", j.WorkLeft, want)
	}
}

// TestZoneClearSkipsReservedAndPostponedPlants: a plant another pawn has
// reserved, or this pawn has put off, is not picked.
func TestZoneClearSkipsReservedAndPostponedPlants(t *testing.T) {
	g, id := clearGame(t)
	other, _ := g.SpawnNamedPawn(2, 1, "Bo")
	pid := putPlant(t, g, plant.Moss, 5, 2, 0, false)
	drawZone(t, g, "potato", 5, 2, 5, 2)
	g.Step(1)
	g.interrupt(id, job.Cancelled)
	g.interrupt(other, job.Cancelled)

	// control: free and unpostponed, the moss is cleared
	if !g.assignZoneClear(id) {
		t.Fatal("control: a free wrong plant was not cleared")
	}
	g.interrupt(id, job.Cancelled)
	g.reserve.ReleaseOwner(id)

	if !g.reserve.TryEntity(other, pid) {
		t.Fatal("could not reserve the moss for Bo")
	}
	if g.assignZoneClear(id) {
		t.Fatal("cleared a plant reserved by another pawn")
	}
	g.reserve.ReleaseOwner(other)

	g.postpone(postKey{Pawn: id, Kind: job.Cut, Target: pid}, job.Blocked)
	if g.assignZoneClear(id) {
		t.Fatal("cleared a plant this pawn had put off")
	}
}

// TestZoneNeverCutsTheRightCrop: a zone's own crop, and its own unripe
// trees, stand untouched however long the grower works.
func TestZoneNeverCutsTheRightCrop(t *testing.T) {
	for _, tc := range []struct {
		name string
		crop string
		s    plant.Species
	}{
		{"unripe potato in a potato zone", "potato", plant.Potato},
		{"unripe zone tree in a tree zone", "tree", plant.Tree},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, id := clearGame(t)
			pid := putPlant(t, g, tc.s, 4, 2, 100, true)
			drawZone(t, g, tc.crop, 4, 2, 4, 2)
			g.Step(1)
			g.interrupt(id, job.Cancelled)
			if g.assignZoneClear(id) {
				t.Fatalf("assignZoneClear started %+v on the right crop", jobOf(g, id))
			}
			for i := 0; i < 600; i++ {
				g.Step(1)
				if j := jobOf(g, id); j.Kind == job.Cut {
					t.Fatalf("tick %d: the grower started cutting: %+v", i, j)
				}
			}
			if !alive(g, pid) {
				t.Fatal("the right crop was cut")
			}
		})
	}
}

// TestZoneHarvestsRipeWrongCropBeforeClearing: a ripe plant that is wrong
// for the zone is harvested through its mark (never cut while ripe); the
// cell is cleared and sown once the harvest leaves it unripe or empty.
func TestZoneHarvestsRipeWrongCropBeforeClearing(t *testing.T) {
	t.Run("ripe corn in a potato zone", func(t *testing.T) {
		g, id := clearGame(t)
		orig := putPlant(t, g, plant.Corn, 4, 2, plant.DefOf(plant.Corn).GrowTicks, true)
		drawZone(t, g, "potato", 4, 2, 4, 2)
		g.Step(1)
		g.interrupt(id, job.Cancelled)
		if g.assignZoneClear(id) {
			t.Fatalf("assignZoneClear cut a ripe crop: %+v", jobOf(g, id))
		}
		for i := 0; i < 1500 && !(g.plantAt(4, 2) != 0 && g.plantAt(4, 2) != orig); i++ {
			g.Step(1)
			if j := jobOf(g, id); j.Kind == job.Cut && j.Target == orig {
				t.Fatalf("tick %d: the ripe corn was cut, not harvested", i)
			}
		}
		if g.World().FoodByKind["corn"] < plant.DefOf(plant.Corn).Yield {
			t.Fatalf("corn harvested = %v", g.World().FoodByKind)
		}
		if p, _ := g.plants.Get(g.plantAt(4, 2)); p.Species != plant.Potato {
			t.Fatalf("cell holds %+v, want potatoes after the harvest", p)
		}
	})
	t.Run("ripe wild berry bush in a potato zone", func(t *testing.T) {
		g, id := clearGame(t)
		orig := putPlant(t, g, plant.BerryBush, 4, 2, plant.DefOf(plant.BerryBush).GrowTicks, false)
		drawZone(t, g, "potato", 4, 2, 4, 2)
		g.Step(1)
		g.interrupt(id, job.Cancelled)
		if g.assignZoneClear(id) {
			t.Fatalf("assignZoneClear cut a ripe bush: %+v", jobOf(g, id))
		}
		harvested := false
		for i := 0; i < 2500 && !(g.plantAt(4, 2) != 0 && g.plantAt(4, 2) != orig); i++ {
			g.Step(1)
			if j := jobOf(g, id); j.Kind == job.Cut && j.Target == orig {
				if p, _ := g.plants.Get(orig); p.Harvestable() {
					t.Fatalf("tick %d: the ripe bush was cut", i)
				}
				harvested = true // cut only once the harvest has set it back
			}
		}
		if g.World().FoodByKind["berries"] < plant.DefOf(plant.BerryBush).Yield {
			t.Fatalf("berries harvested = %v", g.World().FoodByKind)
		}
		if !harvested {
			t.Fatal("the harvested bush was never cleared")
		}
		if p, _ := g.plants.Get(g.plantAt(4, 2)); p.Species != plant.Potato {
			t.Fatalf("cell holds %+v, want potatoes", p)
		}
	})
}

// TestNoClearingOutOfSeason: in a frozen winter the soil is not sowable,
// so nothing is cleared for it; in spring the same moss is.
func TestNoClearingOutOfSeason(t *testing.T) {
	g, id := clearGame(t)
	moss := putPlant(t, g, plant.Moss, 4, 2, 0, false)
	drawZone(t, g, "potato", 4, 2, 4, 2)
	g.Step(1)
	g.interrupt(id, job.Cancelled)

	// control: in spring the moss is cleared
	if !g.assignZoneClear(id) {
		t.Fatal("spring soil: the moss should be cleared")
	}
	if j := jobOf(g, id); j.Kind != job.Cut || j.Target != moss {
		t.Fatalf("job = %+v, want Cut of the moss", j)
	}
	g.interrupt(id, job.Cancelled)
	g.reserve.ReleaseOwner(id)

	jumpTo(g, 45, 12)
	if g.assignZoneClear(id) {
		t.Fatalf("cleared out of season: %+v", jobOf(g, id))
	}
	g.Step(600)
	if !alive(g, moss) {
		t.Fatal("moss was cut in winter")
	}
}

// TestZoneClearIgnoresUnsowableCells: a cell blocked for another reason (a
// planned wall, here) is not cleared: sowing could not follow.
func TestZoneClearIgnoresUnsowableCells(t *testing.T) {
	g, id := clearGame(t)
	putPlant(t, g, plant.Moss, 4, 2, 0, false)
	drawZone(t, g, "potato", 4, 2, 4, 2)
	g.Step(1)
	g.interrupt(id, job.Cancelled)
	if !g.sowableIgnoringPlant(4, 2) || !g.assignZoneClear(id) {
		t.Fatal("control: the free cell's moss should be cleared")
	}
	g.interrupt(id, job.Cancelled)
	g.reserve.ReleaseOwner(id)
	g.Enqueue(command.Command{Kind: command.DesignateWall, X: 4, Y: 2})
	g.Step(1)
	g.interrupt(id, job.Cancelled)
	if g.sowableIgnoringPlant(4, 2) {
		t.Fatal("a framed cell should not be sowable")
	}
	if g.assignZoneClear(id) {
		t.Fatalf("cleared a cell that cannot be sown: %+v", jobOf(g, id))
	}
}

// TestForcedSowUsesTheCropsOwnWork: a Prioritize "Sow potato" order takes
// the potato's sow work, not the generic constant, as the growers' own
// sowing does.
func TestForcedSowUsesTheCropsOwnWork(t *testing.T) {
	for _, tc := range []struct {
		crop string
		s    plant.Species
	}{{"potato", plant.Potato}, {"tree", plant.Tree}} {
		t.Run(tc.crop, func(t *testing.T) {
			g, id := clearGame(t)
			drawZone(t, g, tc.crop, 4, 2, 4, 2)
			g.Step(1)
			rec, why := g.forcedJob(id, command.OrderSow, 4, 2, 0)
			if why != "" {
				t.Fatalf("forced sow refused: %q", why)
			}
			want := plant.DefOf(tc.s).SowWork
			if want == SowWork {
				t.Fatalf("test needs a crop whose sow work differs from %d", SowWork)
			}
			if rec.Kind != job.Sow || rec.WorkLeft != want {
				t.Fatalf("forced sow = %+v, want Sow with WorkLeft %d", rec, want)
			}
		})
	}
}

// TestZoneDrawnOverRipePlantMarksIt: drawing a growing zone over a ripe
// harvestable plant marks it for harvest; an unripe plant, a plant outside
// the zone and a plant that already sat in a zone (its mark cancelled) are
// left alone.
func TestZoneDrawnOverRipePlantMarksIt(t *testing.T) {
	g, _ := clearGame(t)
	ripeBush := putPlant(t, g, plant.BerryBush, 4, 2, plant.DefOf(plant.BerryBush).GrowTicks, false)
	ripeCorn := putPlant(t, g, plant.Corn, 5, 2, plant.DefOf(plant.Corn).GrowTicks, true) // a wrong, ripe crop
	young := putPlant(t, g, plant.Corn, 6, 2, 100, true)
	outside := putPlant(t, g, plant.BerryBush, 9, 4, plant.DefOf(plant.BerryBush).GrowTicks, false)
	for _, pid := range []entity.EntityID{ripeBush, ripeCorn, young, outside} {
		if p, _ := g.plants.Get(pid); p.Designated {
			t.Fatalf("plant %d marked before any zone", pid)
		}
	}
	drawZone(t, g, "potato", 4, 2, 6, 2)
	g.Step(1)
	marked := func(pid entity.EntityID) bool { p, _ := g.plants.Get(pid); return p.Designated }
	if !marked(ripeBush) || !marked(ripeCorn) {
		t.Fatalf("ripe plants under the new zone: bush %v, corn %v, want both marked", marked(ripeBush), marked(ripeCorn))
	}
	if marked(young) {
		t.Fatal("an unripe plant was marked")
	}
	if marked(outside) {
		t.Fatal("a ripe plant outside the zone was marked")
	}

	// the player cancels one mark, then draws a touching zone of the same
	// crop: the cancelled plant stays unmarked (it was already in a zone)
	g.Enqueue(command.Command{Kind: command.Undesignate, Order: command.OrderHarvest, X: 4, Y: 2, X2: 4, Y2: 2})
	g.Step(1)
	if marked(ripeBush) {
		t.Fatal("Cancel harvest did not clear the mark")
	}
	drawZone(t, g, "potato", 7, 2, 8, 2)
	g.Step(1)
	if marked(ripeBush) {
		t.Fatal("drawing a neighbouring zone re-marked a plant whose harvest was cancelled")
	}
}

// TestWantsHarvestNeedsTheMark: a ripe zone crop is wanted only while
// marked; a ripe wild plant likewise.
func TestWantsHarvestNeedsTheMark(t *testing.T) {
	ripe := plant.Plant{Species: plant.Potato, Growth: plant.DefOf(plant.Potato).GrowTicks, Zone: true}
	if wantsHarvest(ripe) {
		t.Fatal("an unmarked ripe zone crop is wanted")
	}
	ripe.Designated = true
	if !wantsHarvest(ripe) {
		t.Fatal("a marked ripe crop is not wanted")
	}
	ripe.Growth = 0
	if wantsHarvest(ripe) {
		t.Fatal("an unripe marked crop is wanted")
	}
	wild := plant.Plant{Species: plant.BerryBush, Growth: plant.DefOf(plant.BerryBush).GrowTicks, Designated: true}
	if !wantsHarvest(wild) {
		t.Fatal("a marked ripe wild bush is not wanted")
	}
}

// TestCancelHarvestSticksOnRipeZoneCrop: a ripe, marked zone crop is
// harvested; after "Cancel harvest" (Undesignate harvest) a grower leaves
// it standing.
func TestCancelHarvestSticksOnRipeZoneCrop(t *testing.T) {
	setup := func(t *testing.T) (*Game, entity.EntityID, entity.EntityID) {
		g, id := clearGame(t)
		pid := putPlant(t, g, plant.Potato, 4, 2, plant.DefOf(plant.Potato).GrowTicks, true)
		drawZone(t, g, "potato", 4, 2, 4, 2)
		g.Step(1) // the zone draw marks the ripe crop
		if p, _ := g.plants.Get(pid); !p.Designated {
			t.Fatal("the ripe zone crop was not marked when the zone was drawn")
		}
		return g, id, pid
	}
	t.Run("marked crop is harvested", func(t *testing.T) {
		g, _, pid := setup(t)
		g.Step(TicksPerCell*20 + plant.DefOf(plant.Potato).HarvestWork + 60)
		if alive(g, pid) {
			t.Fatal("the marked ripe crop was never harvested")
		}
		if g.World().FoodByKind["potatoes"] < plant.DefOf(plant.Potato).Yield {
			t.Fatalf("potatoes = %v", g.World().FoodByKind)
		}
	})
	t.Run("cancelled harvest sticks", func(t *testing.T) {
		g, id, pid := setup(t)
		g.interrupt(id, job.Cancelled)
		g.Enqueue(command.Command{Kind: command.Undesignate, Order: command.OrderHarvest, X: 4, Y: 2, X2: 4, Y2: 2})
		g.Step(TicksPerCell*20 + plant.DefOf(plant.Potato).HarvestWork + 400)
		p, ok := g.plants.Get(pid)
		if !ok {
			t.Fatal("the crop was harvested or cut after Cancel harvest")
		}
		if p.Designated || !p.Harvestable() {
			t.Fatalf("plant = %+v, want ripe and unmarked", p)
		}
		if got := g.World().FoodByKind["potatoes"]; got != 0 {
			t.Fatalf("potatoes = %d, want none", got)
		}
	})
}
