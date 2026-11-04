package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// buildAll builds kind on each tile and returns the ids in tile order.
func buildAll(t *testing.T, j *journey, kind string, tiles ...[2]int) []entity.EntityID {
	t.Helper()
	var ids []entity.EntityID
	for _, c := range tiles {
		if !j.g.BuildNow(kind, "wood", "south", c[0], c[1]) {
			t.Fatalf("no %s at %v", kind, c)
		}
		for _, b := range j.world().Buildings {
			if b.X == c[0] && b.Y == c[1] && b.Kind == kind {
				ids = append(ids, b.ID)
			}
		}
	}
	j.update(1)
	return ids
}

func doubleClick(j *journey, x, y int) {
	j.t.Helper()
	j.settle()
	j.clickTile(x, y, inputframe.Left, 0)
	j.clickTile(x, y, inputframe.Left, 0)
}

// sameIDs compares regardless of order.
func sameIDs(got, want []entity.EntityID) bool {
	got, want = slices.Clone(got), slices.Clone(want)
	slices.Sort(got)
	slices.Sort(want)
	return slices.Equal(got, want)
}

func TestDoubleClickDoorsTakesThoseInView(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(200, 24), jPawns([2]int{18, 8}))
	doors := buildAll(t, j, "door", [2]int{10, 2}, [2]int{12, 2}, [2]int{150, 2})
	doubleClick(j, 10, 2)
	if got := j.a.sel.Group; got.Kind() != selectionpkg.Building || !sameIDs(got.IDs(), doors[:2]) {
		t.Fatalf("group %v %v, want the doors in view %v", got.Kind(), got.IDs(), doors[:2])
	}
	j.draw(2) // the marks
}

func TestDoubleClickWallIsAPlainClick(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	walls := buildAll(t, j, "wall", [2]int{10, 2}, [2]int{12, 2})
	doubleClick(j, 10, 2)
	if j.a.sel.Grouped() || !j.a.sel.HasBuilding || j.a.sel.SelBuilding != walls[0] {
		t.Fatalf("group %v, building %v, want the one wall", j.a.sel.Group.IDs(), j.a.sel.SelBuilding)
	}
}

func TestDoubleClickWallBlueprintsGroup(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	for _, x := range []int{10, 12} {
		j.g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Wall), Stuff: uint8(item.Wood), X: x, Y: 2})
	}
	j.tick(1)
	if n := len(j.world().Frames); n != 2 {
		t.Fatalf("%d blueprints", n)
	}
	doubleClick(j, 10, 2)
	if got := j.a.sel.Group; got.Kind() != selectionpkg.Frame || len(got.IDs()) != 2 {
		t.Fatalf("group %v %v, want both blueprints", got.Kind(), got.IDs())
	}
	j.draw(2)
}

func TestDoubleClickZoneAndItsPlantTakeTheZones(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	a := j.layZone("zones", "grow", 8, 1, 9, 2)
	b := j.layZone("zones", "grow", 12, 1, 13, 2)
	want := []entity.EntityID{entity.EntityID(a.ID), entity.EntityID(b.ID)}
	doubleClick(j, 8, 1)
	if got := j.a.sel.Group; got.Kind() != selectionpkg.Zone || !sameIDs(got.IDs(), want) {
		t.Fatalf("a zone's double click: %v %v, want %v", got.Kind(), got.IDs(), want)
	}
	j.draw(2)

	j.a.sel.Clear(selectionpkg.None)
	if _, err := j.g.SpawnPlantSpecies(j.world().Zones[0].Crop, 9, 2, 50); err != nil {
		t.Fatal(err)
	}
	j.update(1)
	doubleClick(j, 9, 2)
	if got := j.a.sel.Group; got.Kind() != selectionpkg.Zone || !sameIDs(got.IDs(), want) {
		t.Fatalf("a zone plant's double click: %v %v, want %v", got.Kind(), got.IDs(), want)
	}
}

func TestDoubleClickWildPlantIsAPlainClick(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	if _, err := j.g.SpawnPlant(10, 2); err != nil {
		t.Fatal(err)
	}
	j.update(1)
	doubleClick(j, 10, 2)
	if j.a.sel.Grouped() || !j.a.sel.HasPlant {
		t.Fatalf("group %v plant %v, want the plant alone", j.a.sel.Group.IDs(), j.a.sel.HasPlant)
	}
}

func TestBoxTakesFurnitureBlueprintsThenZones(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	doors := buildAll(t, j, "door", [2]int{8, 6}, [2]int{10, 6})
	buildAll(t, j, "wall", [2]int{9, 6})
	j.dragTiles(inputframe.Left, 7, 5, 11, 7, 0)
	if got := j.a.sel.Group; got.Kind() != selectionpkg.Building || !sameIDs(got.IDs(), doors) {
		t.Fatalf("furniture box: %v %v, want %v", got.Kind(), got.IDs(), doors)
	}

	j.g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Wall), Stuff: uint8(item.Wood), X: 8, Y: 2})
	j.g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Wall), Stuff: uint8(item.Wood), X: 10, Y: 2})
	j.tick(1)
	j.settle()
	j.dragTiles(inputframe.Left, 7, 1, 11, 3, 0)
	if got := j.a.sel.Group; got.Kind() != selectionpkg.Frame || len(got.IDs()) != 2 {
		t.Fatalf("blueprint box: %v %v", got.Kind(), got.IDs())
	}

	j.a.sel.Clear(selectionpkg.None)
	a := j.layZone("zones", "grow", 14, 2, 15, 3)
	b := j.layZone("zones", "stockpile", 14, 5, 15, 6)
	j.settle()
	j.dragTiles(inputframe.Left, 13, 1, 16, 7, 0)
	if got := j.a.sel.Group; got.Kind() != selectionpkg.Zone || !sameIDs(got.IDs(), []entity.EntityID{entity.EntityID(a.ID), entity.EntityID(b.ID)}) {
		t.Fatalf("zone box: %v %v", got.Kind(), got.IDs())
	}
	j.draw(2)

	j.settle()
	j.dragTiles(inputframe.Left, 1, 1, 4, 4, 0)
	if j.a.sel.Grouped() {
		t.Fatalf("a box over bare ground kept %v", j.a.sel.Group.IDs())
	}
}

func TestShiftMixesKindsOnTheMap(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	door := buildAll(t, j, "door", [2]int{10, 2})[0]
	stack, err := j.g.SpawnStack(item.Wood, 10, 12, 2)
	if err != nil {
		t.Fatal(err)
	}
	j.update(1)
	j.clickTile(10, 2, inputframe.Left, 0)
	j.settle()
	j.clickTile(12, 2, inputframe.Left, input.ModShift)
	want := []selectionpkg.Cand{{Kind: selectionpkg.Building, ID: door}, {Kind: selectionpkg.Item, ID: stack}}
	if got := j.a.sel.Group; got.Kind() != selectionpkg.Mixed || !slices.Equal(got.Members, want) {
		t.Fatalf("group %v %v, want %v", got.Kind(), got.Members, want)
	}
	j.draw(2)
	j.settle()
	j.clickTile(10, 2, inputframe.Left, input.ModShift)
	if j.a.sel.Grouped() || !j.a.sel.HasItem || j.a.sel.SelItem != stack {
		t.Fatalf("taking the door out leaves group %v item %v", j.a.sel.Group.IDs(), j.a.sel.SelItem)
	}
}
