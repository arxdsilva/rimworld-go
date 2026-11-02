package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// journey_allow_test.go (STK-AL, .cursor/ALLOW_GAP_ANALYSIS.md): the allow
// system through the player's input path. A journey colony (game.New) has
// no Home; alHome paints rows 6..23, so rows 0..5 lie outside it.

// alHome paints the Home area over rows 6..23.
func alHome(j *journey) {
	j.g.Enqueue(command.Command{Kind: command.DesignateHome, X: 0, Y: 6, X2: 23, Y2: 23, Enabled: true})
	j.g.ApplyPending()
}

// alItem is stack id's read model (ok false: gone or merged away).
func alItem(j *journey, id entity.EntityID) (game.ItemView, bool) {
	for _, it := range j.world().Items {
		if it.ID == id {
			return it, true
		}
	}
	return game.ItemView{}, false
}

// alItemAt is the ground stack on (x, y) whose kind ends in suffix.
func alItemAt(j *journey, suffix string, x, y int) (game.ItemView, bool) {
	for _, it := range j.world().Items {
		if !it.Carried && strings.HasSuffix(it.Kind, suffix) && it.X == x && it.Y == y {
			return it, true
		}
	}
	return game.ItemView{}, false
}

// alForbidStack selects stack id on (x, y) with a click and forbids it
// with F.
func alForbidStack(j *journey, id entity.EntityID, x, y int) {
	j.t.Helper()
	j.clickTile(x, y, inputframe.Left, 0)
	j.key("F", 0)
	j.update(1)
	if it, ok := alItem(j, id); !ok || !it.Forbidden {
		j.t.Fatalf("F did not forbid stack %d: %+v %v", id, it, ok)
	}
	j.settle()
}

// TestJourneyAllowDoorKeyAndMarker: F on a selected door forbids it; the
// pane then says so, its toggle reads Allow, the frame draws the marker,
// and F again allows it.
func TestJourneyAllowDoorKeyAndMarker(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.BuildNow("door", "wood", "south", 12, 3) {
		t.Fatal("no door")
	}
	id := wbBuildingAt(j, 12, 3)
	j.lookAt(12, 3)
	j.update(1)
	j.settle()
	j.clickTile(12, 3, inputframe.Left, 0)
	if !j.a.sel.HasBuilding || j.a.sel.SelBuilding != id {
		t.Fatalf("the click did not select the door %d", id)
	}
	j.key("F", 0)
	c, ok := j.lastCmd(command.SetForbidden)
	if !ok || c.Target != uint64(id) || !c.Enabled {
		t.Fatalf("F on the door sent %+v (%v), want SetForbidden on %d", c, ok, id)
	}
	j.update(1)
	if !strings.Contains(wbPaneLines(j, id), "forbidden") {
		t.Fatalf("the door pane does not say forbidden: %q", wbPaneLines(j, id))
	}
	for _, b := range j.a.insp.Building().BuildingCommands(id) {
		if b.ID == "bldg.forbid" && (b.Label != "Allow" || !b.On) {
			t.Fatalf("the forbidden door's toggle = %+v, want Allow, on", b)
		}
	}
	j.draw(1) // the forbidden marker over the door (render/world objects.go)
	j.reset()
	j.key("F", 0)
	if c, ok := j.lastCmd(command.SetForbidden); !ok || c.Enabled || c.Target != uint64(id) {
		t.Fatalf("the second F sent %+v (%v), want an allow of the door", c, ok)
	}
}

// TestJourneyAllowHaulThingsAllows: the Architect's Haul things tool
// dragged over a forbidden stack allows it.
func TestJourneyAllowHaulThingsAllows(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	id, _ := j.g.SpawnStack(item.Wood, 10, 10, 3)
	alForbidStack(j, id, 10, 3)
	j.openArch("orders", "haul")
	j.dragTiles(inputframe.Left, 9, 2, 11, 4, 0)
	if _, ok := j.lastCmd(command.DesignateHaul); !ok {
		t.Fatal("the Haul tool sent no DesignateHaul")
	}
	j.update(1)
	if it, ok := alItem(j, id); ok && it.Forbidden {
		t.Fatalf("the hauled-over stack is still forbidden: %+v", it)
	}
}

// TestJourneyAllowOrderedEquipAllows: ordering a colonist to equip a
// forbidden knife from the right-click menu allows it.
func TestJourneyAllowOrderedEquipAllows(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id, _ := j.g.SpawnStack(item.Knife, 1, 12, 4)
	alForbidStack(j, id, 12, 4)
	ordPick(j, 0)
	j.clickTile(12, 4, inputframe.Right, 0)
	j.pick("Equip")
	if c, ok := j.lastCmd(command.Prioritize); !ok || c.Order != command.OrderEquip {
		t.Fatalf("Equip sent %+v (%v)", c, ok)
	}
	j.update(1)
	if it, ok := alItem(j, id); ok && it.Forbidden {
		t.Fatalf("the ordered knife is still forbidden: %+v", it)
	}
}

// TestJourneyAllowDeathOutsideHome: an animal killed outside the home area
// leaves a forbidden carcass, one killed inside does not; Mark for hauling
// on the forbidden one allows and marks it in one click.
func TestJourneyAllowDeathOutsideHome(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	alHome(j)
	out, err := j.g.SpawnAnimal("dustbuck", 14, 2)
	if err != nil {
		t.Fatal(err)
	}
	in, err := j.g.SpawnAnimal("dustbuck", 14, 7)
	if err != nil {
		t.Fatal(err)
	}
	var ox, oy, ix, iy int
	for _, an := range j.world().Animals {
		switch an.ID {
		case out:
			ox, oy = an.X, an.Y
		case in:
			ix, iy = an.X, an.Y
		}
	}
	j.g.KillAnimal(out)
	j.g.KillAnimal(in)
	j.update(1)
	far, ok := alItemAt(j, "corpse", ox, oy)
	if !ok || !far.Forbidden || !far.Unclaimed {
		t.Fatalf("the carcass outside Home at (%d, %d) = %+v (%v), want forbidden and found", ox, oy, far, ok)
	}
	if near, ok := alItemAt(j, "corpse", ix, iy); !ok || near.Forbidden {
		t.Fatalf("the carcass inside Home at (%d, %d) = %+v (%v), want allowed", ix, iy, near, ok)
	}
	j.lookAt(ox, oy)
	j.update(1)
	j.settle()
	j.clickTile(ox, oy, inputframe.Left, 0)
	j.draw(1) // the item's forbidden marker
	if !j.clickID("item.haulmark") {
		t.Fatal("Mark for hauling is disabled on a forbidden carcass")
	}
	if c, ok := j.lastCmd(command.SetHaulMark); !ok || !c.Enabled || c.Target != uint64(far.ID) {
		t.Fatalf("the mark sent %+v (%v)", c, ok)
	}
	j.update(1)
	if it, ok := alItem(j, far.ID); !ok || it.Forbidden || !it.HaulMarked {
		t.Fatalf("the marked carcass = %+v (%v), want allowed and marked", it, ok)
	}
}
