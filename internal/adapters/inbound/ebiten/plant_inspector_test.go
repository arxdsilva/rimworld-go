package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func btnByID(bs []button, id string) (button, bool) {
	for _, b := range bs {
		if b.id == id {
			return b, true
		}
	}
	return button{}, false
}

func plantByID(a *App, id entity.EntityID) game.PlantView {
	for _, p := range a.renderer.world().Plants {
		if p.ID == id {
			return p
		}
	}
	return game.PlantView{}
}

func selectPlantTile(t *testing.T, a *App, id entity.EntityID, x, y int) {
	t.Helper()
	clickTile(a, x, y)
	if p, ok := a.selectedPlant(); !ok || p.ID != id {
		t.Fatalf("plant %d not selected", id)
	}
}

func TestPlantPanelHarvestStates(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	ripe, _ := g.SpawnCrop("potato", 8, 8, 100)
	young, _ := g.SpawnCrop("corn", 10, 10, 30)

	selectPlantTile(t, a, ripe, 8, 8)
	b, ok := btnByID(a.plantButtons(), "plant.harvest")
	if !ok || b.disabled || b.label != "Harvest" {
		t.Fatalf("ripe crop harvest button = %+v", b)
	}
	click(t, a, "plant.harvest")
	g.Step(1)
	if !plantByID(a, ripe).Designated {
		t.Fatal("Harvest marks the plant")
	}
	if b, _ := btnByID(a.plantButtons(), "plant.harvest"); b.label != "Cancel harvest" || !b.selected {
		t.Fatalf("marked plant should offer Cancel harvest: %+v", b)
	}
	click(t, a, "plant.harvest")
	g.Step(1)
	if plantByID(a, ripe).Designated {
		t.Fatal("Cancel harvest clears the mark")
	}

	selectPlantTile(t, a, young, 10, 10)
	b, _ = btnByID(a.plantButtons(), "plant.harvest")
	if !b.disabled || b.reason != "not ready" {
		t.Fatalf("unripe crop harvest must be disabled 'not ready': %+v", b)
	}
	lines := strings.Join(plantLines(plantByID(a, young)), "|")
	if !strings.Contains(lines, "Not ready (30%)") {
		t.Fatalf("lines = %q", lines)
	}
	if got := plantLines(plantByID(a, ripe)); got[0] != "Ready to harvest" {
		t.Fatalf("ripe lines = %v", got)
	}
}

func TestPlantCutUsesUndesignate(t *testing.T) {
	t.Parallel()
	p := game.PlantView{X: 4, Y: 5, Species: "corn", Cut: true}
	c := plantMarkCmd(p, false, true)
	if c.Kind != command.Undesignate || c.Order != command.OrderCut || c.X != 4 || c.X2 != 4 || c.Y2 != 5 {
		t.Fatalf("cancel cut = %+v, want 1-tile Undesignate OrderCut", c)
	}
	if c := plantMarkCmd(p, true, true); c.Kind != command.Undesignate || c.Order != command.OrderHarvest {
		t.Fatalf("cancel harvest = %+v", c)
	}
	if c := plantMarkCmd(p, false, false); c.Kind != command.DesignateCut {
		t.Fatalf("cut = %+v", c)
	}

	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnCrop("corn", 8, 8, 30)
	selectPlantTile(t, a, id, 8, 8)
	click(t, a, "plant.cut")
	g.Step(1)
	if !plantByID(a, id).Cut {
		t.Fatal("Cut marks the plant")
	}
	if b, _ := btnByID(a.plantButtons(), "plant.cut"); b.label != "Cancel cut" {
		t.Fatalf("label = %q", b.label)
	}
	click(t, a, "plant.cut")
	g.Step(1)
	if plantByID(a, id).Cut {
		t.Fatal("Cancel cut clears the mark")
	}
}

func TestTreeSaysChop(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnCrop("tree", 8, 8, 100)
	selectPlantTile(t, a, id, 8, 8)
	bs := a.plantButtons()
	if b, ok := btnByID(bs, "plant.cut"); !ok || b.label != "Chop" {
		t.Fatalf("tree cut button = %+v", b)
	}
	if _, ok := btnByID(bs, "plant.harvest"); ok {
		t.Fatal("trees have no Harvest button")
	}
	p := plantByID(a, id)
	if title := plantTitle(p); !strings.HasSuffix(title, " tree") || title == "Tree" {
		t.Fatalf("title = %q", title)
	}
	if plantCategory(p) != "Tree" {
		t.Fatalf("category = %q", plantCategory(p))
	}
}

func TestPlantZoneLine(t *testing.T) {
	t.Parallel()
	got := plantLines(game.PlantView{Species: "potato", Growth: 10, Zone: "potato", Wilted: true, Cut: true})
	want := []string{"Not ready (10%)", "Wilting: too cold", "In a growing zone (Potato)", "Marked to cut"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	tree := plantLines(game.PlantView{Species: "tree", Zone: "tree"})
	if len(tree) != 1 || tree[0] != "In a growing zone (Tree)" {
		t.Fatalf("tree lines = %v", tree)
	}
	if plantCategory(game.PlantView{Species: "berry_bush"}) != "Wild plant" ||
		plantTitle(game.PlantView{Species: "berry_bush"}) != "Berry bush" ||
		plantCategory(game.PlantView{Species: "potato"}) != "Crop" {
		t.Fatal("categories/titles wrong")
	}
	if plantStageWord(game.PlantView{Stage: 1}) != "sprouting" ||
		plantStageWord(game.PlantView{Species: "corn", Harvestable: true, Stage: 3}) != "ready to harvest" {
		t.Fatal("stage words wrong")
	}
	if got := plantLines(game.PlantView{Species: "corn", Harvestable: true, Designated: true}); got[1] != "Marked to harvest" {
		t.Fatalf("lines = %v", got)
	}
}

func TestPlantNowChips(t *testing.T) {
	t.Parallel()
	a, g, pawn := liveColony(t, 0)
	id, _ := g.SpawnCrop("potato", 5, 5, 100)
	selectPlantTile(t, a, id, 5, 5)
	var chip button
	for _, b := range a.plantButtons() {
		if strings.HasPrefix(b.id, "plant.now.") {
			chip = b
		}
	}
	if chip.id == "" || chip.disabled {
		t.Fatalf("expected an enabled now chip, got %+v", chip)
	}
	if !strings.HasSuffix(chip.id, "."+strconv.Itoa(int(pawn))) {
		t.Fatalf("chip id %q should name pawn %d", chip.id, pawn)
	}
	click(t, a, chip.id)
	a.applyPlantClick("plant.now.notanumber")
	a.applyPlantClick("plant.close")
	if a.hasPlant {
		t.Fatal("close clears the selection")
	}
	a.applyPlantClick("plant.close") // nothing selected: no-op
}

func TestPlantPanelButtonsDisabledChip(t *testing.T) {
	t.Parallel()
	bs := plantPanelButtons(plantRect(), game.PlantView{Species: "corn"},
		[]colonistAction{{Pawn: 7, Name: "Ada", Disabled: true, Reason: "cannot reach"}})
	b, ok := btnByID(bs, "plant.now.7")
	if !ok || !b.disabled || b.reason != "cannot reach" || b.label != "Ada ×" {
		t.Fatalf("chip = %+v", b)
	}
}

func TestPlantPanelClosesWhenPlantGone(t *testing.T) {
	t.Parallel()
	a, _, _ := liveColony(t, 0)
	a.selPlant, a.hasPlant = 9999, true
	if _, ok := a.selectedPlant(); ok {
		t.Fatal("a vanished plant is not selected")
	}
	if a.hasPlant || len(a.plantButtons()) != 0 {
		t.Fatal("panel must close with no buttons")
	}
	a.applyPlantClick("plant.harvest") // no-op, no panic
}
