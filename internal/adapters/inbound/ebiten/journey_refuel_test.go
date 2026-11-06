package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// refuelJourney is a journey with an unfuelled campfire at (13, 5) and a
// colonist selected.
func refuelJourney(t *testing.T, wood bool) *journey {
	t.Helper()
	j := newJourney(t)
	if !j.g.BuildNow("campfire", "wood", "south", 13, 5) {
		t.Fatal("no campfire")
	}
	if wood {
		if _, err := j.g.SpawnStack(item.Wood, 20, 11, 5); err != nil {
			t.Fatal(err)
		}
	}
	j.update(1)
	ordPick(j, 0)
	return j
}

// TestJourneyOrdersPrioritizeRefuel: right-clicking an unfuelled burner with
// a colonist selected offers Prioritize refueling, and picking it sends the
// refuel order for that colonist and tile.
func TestJourneyOrdersPrioritizeRefuel(t *testing.T) {
	t.Parallel()
	j := refuelJourney(t, true)
	id := ordColonists(j)[0]
	j.clickTile(13, 5, inputframe.Right, 0)
	j.pick("Prioritize refueling")
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderRefuel || c.Pawn != id || c.X != 13 || c.Y != 5 {
		t.Fatalf("sent %+v (%v), want a Prioritize OrderRefuel for colonist %d at (13, 5)", c, ok, id)
	}
}

// TestJourneyBuildingAutoRefuelToggle: the inspector's Auto refuel toggle on
// a selected burner (on by default) sends SetAutoRefuel off for it.
func TestJourneyBuildingAutoRefuelToggle(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.BuildNow("campfire", "wood", "south", 13, 5) {
		t.Fatal("no campfire")
	}
	id := wbBuildingAt(j, 13, 5)
	j.lookAt(13, 5)
	j.update(1)
	j.settle()
	j.clickTile(13, 5, inputframe.Left, 0)
	if !j.a.sel.HasBuilding || j.a.sel.SelBuilding != id {
		t.Fatalf("the click did not select the campfire %d", id)
	}
	if !j.clickID(inspector.AutoRefuelID) {
		t.Fatal("the Auto refuel toggle is disabled")
	}
	c, ok := j.lastCmd(command.SetAutoRefuel)
	if !ok || c.Target != uint64(id) || c.Enabled {
		t.Fatalf("sent %+v (%v), want SetAutoRefuel off on %d", c, ok, id)
	}
}

// TestJourneyOrdersRefuelDisabledShowsReason: with no wood on the map the
// refuel row is disabled; picking it shows why and orders nothing.
func TestJourneyOrdersRefuelDisabledShowsReason(t *testing.T) {
	t.Parallel()
	j := refuelJourney(t, false)
	j.clickTile(13, 5, inputframe.Right, 0)
	j.pick("Prioritize refueling")
	if _, ok := j.lastCmd(command.Prioritize); ok {
		t.Fatalf("a disabled row sent %+v", j.cmds)
	}
	if j.a.hint == "" {
		t.Fatal("a disabled row gave no reason")
	}
}
