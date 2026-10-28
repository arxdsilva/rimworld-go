package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// clickTile left-clicks the centre of tile (tx, ty) through clickSelect.
func clickTile(a *App, tx, ty int) {
	sx, sy := a.renderer.worldToScreenF(float64(tx)+0.5, float64(ty)+0.5)
	a.cursor = &[2]int{int(sx), int(sy)}
	a.clickSelect()
}

func TestClickPrecedenceAndCycle(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if _, err := g.SpawnCrop("potato", 8, 8, 50); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Rifle, 1, 8, 8); err != nil {
		t.Fatal(err)
	}
	clickTile(a, 8, 8)
	if _, ok := a.selectedItem(); !ok {
		t.Fatal("an item comes before the plant under it")
	}
	clickTile(a, 8, 8)
	if _, ok := a.selectedPlant(); !ok {
		t.Fatal("a second click cycles to the plant")
	}
	if _, ok := a.selectedItem(); ok {
		t.Fatal("selecting the plant clears the item")
	}
	clickTile(a, 8, 8)
	if _, ok := a.selectedItem(); !ok {
		t.Fatal("the cycle wraps back to the item")
	}
}

func TestSelectingPlantClearsOthers(t *testing.T) {
	t.Parallel()
	a, g, id := liveColony(t, 0)
	if _, err := g.SpawnCrop("corn", 10, 10, 30); err != nil {
		t.Fatal(err)
	}
	a.renderer.Select(id)
	a.selectBuildingAt(0, 0) // nothing there
	clickTile(a, 10, 10)
	if _, ok := a.selectedPlant(); !ok {
		t.Fatal("clicking a crop selects it")
	}
	if a.renderer.HasSelect() || a.sel.HasAnimal || a.sel.HasBuilding || a.sel.HasItem {
		t.Fatal("selecting a plant clears every other selection")
	}
	clickTile(a, 14, 14) // empty ground
	if a.sel.HasPlant {
		t.Fatal("clicking empty ground clears the selection")
	}
}

func TestColonistActionsFromOrderMenu(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 2)
	if _, err := g.SpawnStack(item.Rifle, 1, 6, 6); err != nil {
		t.Fatal(err)
	}
	acts := a.colonistActions(6, 6, orderIs(command.OrderEquip))
	if len(acts) != 3 {
		t.Fatalf("one Equip action per colonist, got %d", len(acts))
	}
	for i, ac := range acts {
		if ac.Cmd.Kind != command.Prioritize || ac.Cmd.Order != command.OrderEquip || ac.Label == "" {
			t.Fatalf("action %d: %+v", i, ac)
		}
		if i > 0 && !acts[i-1].Disabled && !ac.Disabled && acts[i-1].Dist > ac.Dist {
			t.Fatal("nearest colonists first")
		}
	}
}
