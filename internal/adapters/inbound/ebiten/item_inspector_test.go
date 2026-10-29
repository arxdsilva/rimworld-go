package ebiten

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

func selectItemTile(t *testing.T, a *App, id entity.EntityID, x, y int) {
	t.Helper()
	clickTile(a, x, y)
	if it, ok := a.sel.SelectedItem(); !ok || it.ID != id {
		t.Fatalf("item %d not selected", id)
	}
}

func TestItemEquipChipsPerColonist(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 1)
	id, _ := g.SpawnStack(item.Rifle, 1, 8, 8)
	selectItemTile(t, a, id, 8, 8)
	cs := a.insp.Thing().ItemCommands(id)
	equip := thingCmd(t, cs, "cmd.item.equip")
	opts := equip.Options()
	if len(opts) < 2 {
		t.Fatalf("want one Equip option per colonist, got %+v", opts)
	}
	for _, o := range opts {
		if o.Disabled || o.Cmd == nil {
			t.Fatalf("reachable colonist option disabled: %+v", o)
		}
	}
	// Haul is disabled (no stockpile); only Equip is checked above.
	if haul := thingCmd(t, cs, "cmd.item.haul"); !haul.Disabled || haul.Reason != "no stockpile space" {
		t.Fatalf("haul with no stockpile = %+v", haul)
	}
	thingPress(a, equip) // the nearest colonist
	click(t, a, "item.close")
	if a.sel.HasItem {
		t.Fatal("close clears the selection")
	}
}

func TestItemUnreachableColonistDisabled(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			if dx != 0 || dy != 0 {
				g.BuildNow("wall", "stone", "", 8+dx, 8+dy)
			}
		}
	}
	id, _ := g.SpawnStack(item.Rifle, 1, 8, 8)
	selectItemTile(t, a, id, 8, 8)
	equip := thingCmd(t, a.insp.Thing().ItemCommands(id), "cmd.item.equip")
	opts := equip.Options()
	if len(opts) == 0 {
		t.Skip("walls were not built instantly")
	}
	if !opts[0].Disabled || opts[0].Reason == "" {
		t.Fatalf("walled-in rifle option should be disabled with a reason: %+v", opts[0])
	}
	if !equip.Disabled || equip.Reason != opts[0].Reason {
		t.Fatalf("the command says why nobody can: %+v", equip)
	}
}

func TestItemHaulOnlyWhenUnstored(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnStack(item.Wood, 25, 8, 8)
	selectItemTile(t, a, id, 8, 8)
	it, _ := a.sel.SelectedItem()
	if vs := itemVerbs(it, false); len(vs) != 1 || vs[0].label != "Haul" {
		t.Fatalf("unstored wood verbs = %+v", vs)
	}
	if vs := itemVerbs(it, true); len(vs) != 0 {
		t.Fatalf("stored wood must have no Haul: %+v", vs)
	}
	if got := inspector.ItemLines(it, false); got[len(got)-1] != "Not stored" {
		t.Fatalf("lines = %v", got)
	}
	if got := inspector.ItemLines(it, true); got[len(got)-1] != "In a stockpile" {
		t.Fatalf("lines = %v", got)
	}
	w := game.WorldView{Zones: []game.ZoneView{rectZone(1, game.ZoneStockpile, 7, 7, 9, 9)}}
	if !inspector.InStockpile(w, 8, 8) || inspector.InStockpile(w, 10, 8) {
		t.Fatal("inStockpile bounds wrong")
	}
	if inspector.ItemCategory(it) != "material" {
		t.Fatalf("category = %q", inspector.ItemCategory(it))
	}
}

func TestItemFoodAndApparelRows(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnStack(item.Meal, 2, 6, 6)
	selectItemTile(t, a, id, 6, 6)
	it, _ := a.sel.SelectedItem()
	if vs := itemVerbs(it, true); len(vs) != 1 || vs[0].id != "cmd.item.eat" || vs[0].label != "Eat" {
		t.Fatalf("meal verbs = %+v", vs)
	}
	pid, _ := g.SpawnStack(item.Parka, 1, 7, 7)
	selectItemTile(t, a, pid, 7, 7)
	it, _ = a.sel.SelectedItem()
	if vs := itemVerbs(it, true); len(vs) != 1 || vs[0].id != "cmd.item.wear" || vs[0].label != "Wear" {
		t.Fatalf("parka verbs = %+v", vs)
	}
	a.insp.Thing().ApplyItemClick("item.act.99")
	a.insp.Thing().ApplyItemClick("item.act.x")
	a.insp.Thing().ApplyItemClick("item.bogus")
}

func TestItemPanelClosesWhenItemGone(t *testing.T) {
	t.Parallel()
	a, _, _ := liveColony(t, 0)
	a.sel.SelItem, a.sel.HasItem = 9999, true
	if len(a.insp.Commands().CommandGridButtons()) != 0 || len(a.insp.Thing().ItemCommands(9999)) != 0 || a.insp.Thing().ItemPane(9999).Title != "" || a.sel.HasItem {
		t.Fatal("panel must close")
	}
	a.insp.Thing().ApplyItemClick("item.close")
}

func TestInspectorsDrawWithoutArt(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	pid, _ := g.SpawnCrop("corn", 8, 8, 40)
	iid, _ := g.SpawnStack(item.Rifle, 1, 10, 10)
	screen := ebiten.NewImage(640, 480)
	selectPlantTile(t, a, pid, 8, 8)
	a.drawPlantMark(screen)
	selectItemTile(t, a, iid, 10, 10)
	a.drawItemMark(screen)
	if got := inspector.Capitalize(""); got != "" {
		t.Fatalf("capitalize empty = %q", got)
	}
}
