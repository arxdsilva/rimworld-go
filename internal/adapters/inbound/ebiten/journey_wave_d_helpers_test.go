package ebiten

// journey_wave_d_helpers_test.go: fixtures for the wave D journeys (PRD-3
// fueled benches, BED-3 seating comfort, DEF-3 mortar, traps and holds).
// Names the parallel agents land (items, buttons, fields) are looked up
// at run time, so a missing one fails the journey with a message.

import (
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// wdBuild places a finished building trying each stuff in turn.
func wdBuild(j *journey, kind string, x, y int) {
	j.t.Helper()
	for _, st := range []string{"steel", "wood", "cloth", "leather", "stone"} {
		if j.g.BuildNow(kind, st, "south", x, y) {
			return
		}
	}
	j.t.Fatalf("cannot place %q at (%d, %d) (kind missing or tile blocked)", kind, x, y)
}

// wdBuilding is the building view at (x, y).
func wdBuilding(j *journey, x, y int) (id entity.EntityID) {
	j.t.Helper()
	return wbBuildingAt(j, x, y)
}

// wdSelect selects the building at (x, y) with a real click.
func wdSelect(j *journey, x, y int) entity.EntityID {
	j.t.Helper()
	id := wdBuilding(j, x, y)
	j.lookAt(x, y)
	j.update(1)
	j.settle()
	j.clickTile(x, y, 0, 0)
	if !j.a.sel.HasBuilding || j.a.sel.SelBuilding != id {
		j.t.Fatalf("the click at (%d, %d) did not select building %d", x, y, id)
	}
	return id
}

// wdView is the world's building view with that id.
func wdView(j *journey, id entity.EntityID) (v any) {
	j.t.Helper()
	for _, b := range j.world().Buildings {
		if b.ID == id {
			return b
		}
	}
	j.t.Fatalf("building %d vanished", id)
	return nil
}

// wdButton finds an inspector control id holding every substring.
func wdButton(j *journey, what string, subs ...string) string {
	j.t.Helper()
	if id := wbFind(j, subs...); id != "" {
		return id
	}
	var all []string
	for _, id := range wbIDs(j) {
		if strings.HasPrefix(id, "bldg") || strings.HasPrefix(id, "inspector") || strings.Contains(id, "mortar") || strings.Contains(id, "turret") || strings.Contains(id, "door") {
			all = append(all, id)
		}
	}
	j.t.Fatalf("%s: no control with ids containing %q; building controls on screen: %q", what, subs, all)
	return ""
}

// wdKindAny is the first item kind of those names that exists.
func wdKindAny(j *journey, names ...string) item.Kind {
	j.t.Helper()
	for _, n := range names {
		if k, ok := item.ParseKind(n); ok {
			return k
		}
	}
	j.t.Fatalf("none of the item kinds %q exists yet", names)
	return 0
}
