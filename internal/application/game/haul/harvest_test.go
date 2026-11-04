package haul

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// fakeField is a Harvester and SweepHost: one pawn per id at pos, the
// nearest ripe plant ripe (0: none) and the harvests started.
type fakeField struct {
	growOff  bool
	pos      [2]int
	ripe     entity.EntityID
	rx, ry   int
	refuse   bool
	asked    int
	started  []entity.EntityID
	position int
}

func (f *fakeField) GrowOn(entity.EntityID) bool { return !f.growOff }
func (f *fakeField) Position(entity.EntityID) (int, int) {
	f.position++
	return f.pos[0], f.pos[1]
}
func (f *fakeField) NearestRipe(entity.EntityID) (entity.EntityID, int, int, bool) {
	f.asked++
	return f.ripe, f.rx, f.ry, f.ripe != 0
}
func (f *fakeField) StartHarvest(_, t entity.EntityID) bool {
	if f.refuse {
		return false
	}
	f.started = append(f.started, t)
	return true
}

func TestHarvestStartsTheNearestRipePlant(t *testing.T) {
	t.Parallel()
	f := &fakeField{}
	if Harvest(f, 1) || len(f.started) != 0 {
		t.Fatal("a harvest started with no ripe plant")
	}
	f.ripe = 7
	if !Harvest(f, 1) || len(f.started) != 1 || f.started[0] != 7 {
		t.Fatalf("started %v, want [7]", f.started)
	}
	f.refuse = true
	if Harvest(f, 1) {
		t.Fatal("a harvest that could not start reported true")
	}
}

// TestWantsHarvestNeedsTheMark: a ripe zone crop is wanted only while
// marked; a ripe wild plant likewise.
func TestWantsHarvestNeedsTheMark(t *testing.T) {
	t.Parallel()
	ripe := plant.Plant{Species: plant.Potato, Growth: plant.DefOf(plant.Potato).GrowTicks, Zone: true}
	if WantsHarvest(ripe) {
		t.Fatal("an unmarked ripe zone crop is wanted")
	}
	ripe.Designated = true
	if !WantsHarvest(ripe) {
		t.Fatal("a marked ripe crop is not wanted")
	}
	ripe.Growth = 0
	if WantsHarvest(ripe) {
		t.Fatal("an unripe marked crop is wanted")
	}
	wild := plant.Plant{Species: plant.BerryBush, Growth: plant.DefOf(plant.BerryBush).GrowTicks, Designated: true}
	if !WantsHarvest(wild) {
		t.Fatal("a marked ripe wild bush is not wanted")
	}
}
