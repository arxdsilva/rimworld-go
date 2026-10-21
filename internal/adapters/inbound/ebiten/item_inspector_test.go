package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

func selectItemTile(t *testing.T, a *App, id entity.EntityID, x, y int) {
	t.Helper()
	clickTile(a, x, y)
	if it, ok := a.selectedItem(); !ok || it.ID != id {
		t.Fatalf("item %d not selected", id)
	}
}

func TestItemEquipChipsPerColonist(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 1)
	id, _ := g.SpawnStack(item.Rifle, 1, 8, 8)
	selectItemTile(t, a, id, 8, 8)
	chips := 0
	for _, b := range a.itemButtons() {
		// Haul chips are disabled (no stockpile); only Equip chips are checked.
		if strings.HasPrefix(b.id, "item.act.") && b.reason != "no stockpile space" {
			chips++
			if b.disabled {
				t.Fatalf("reachable colonist chip disabled: %+v", b)
			}
		}
	}
	if chips < 2 {
		t.Fatalf("want one Equip chip per colonist, got %d", chips)
	}
	click(t, a, "item.act.0")
	click(t, a, "item.close")
	if a.hasItem {
		t.Fatal("close clears the selection")
	}
}

func TestItemChipsDisabledWithReason(t *testing.T) {
	t.Parallel()
	rows := []itemChipRow{
		{"Equip:", []colonistAction{{Pawn: 1, Name: "Ada"}, {Pawn: 2, Name: "Bo", Disabled: true, Reason: "cannot reach"}}},
		{"Haul:", []colonistAction{{Pawn: 1, Name: "Ada"}}},
	}
	bs := itemPanelButtons(itemRect(), rows)
	b, ok := btnByID(bs, "item.act.1")
	if !ok || !b.disabled || b.reason != "cannot reach" || b.label != "Bo ×" {
		t.Fatalf("chip 1 = %+v", b)
	}
	if h, _ := btnByID(bs, "item.act.2"); h.y <= b.y {
		t.Fatalf("haul row should sit below equip row: %+v vs %+v", h, b)
	}
	if n := len(flattenChips(rows)); n != 3 {
		t.Fatalf("flatten = %d", n)
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
	for _, b := range a.itemButtons() {
		if strings.HasPrefix(b.id, "item.act.") {
			if !b.disabled || b.reason == "" {
				t.Fatalf("walled-in rifle chip should be disabled with a reason: %+v", b)
			}
			return
		}
	}
	t.Skip("walls were not built instantly")
}

func TestItemHaulOnlyWhenUnstored(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnStack(item.Wood, 25, 8, 8)
	selectItemTile(t, a, id, 8, 8)
	it, _ := a.selectedItem()
	if rows := a.itemRows(it, false); len(rows) != 1 || rows[0].Verb != "Haul:" {
		t.Fatalf("unstored wood rows = %+v", rows)
	}
	if rows := a.itemRows(it, true); len(rows) != 0 {
		t.Fatalf("stored wood must have no Haul row: %+v", rows)
	}
	if got := itemLines(it, false); got[len(got)-1] != "Not stored" {
		t.Fatalf("lines = %v", got)
	}
	if got := itemLines(it, true); got[len(got)-1] != "In a stockpile" {
		t.Fatalf("lines = %v", got)
	}
	w := game.WorldView{Stockpiles: []game.StockpileView{{MinX: 7, MinY: 7, MaxX: 9, MaxY: 9}}}
	if !inStockpile(w, 8, 8) || inStockpile(w, 10, 8) {
		t.Fatal("inStockpile bounds wrong")
	}
	if itemCategory(it) != "material" {
		t.Fatalf("category = %q", itemCategory(it))
	}
}

func TestItemCorpseTitle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		it    game.ItemView
		title string
		cat   string
	}{
		{game.ItemView{Kind: "human corpse", OfName: "Ada", Qty: 1}, "Body of Ada", "body"},
		{game.ItemView{Kind: "bones", OfName: "Ada", Qty: 1}, "Bones of Ada", "body"},
		{game.ItemView{Kind: "human corpse"}, "Body", "body"},
		{game.ItemView{Kind: "bones"}, "Bones", "body"},
		{game.ItemView{Kind: "rifle", Qty: 1}, "Rifle", "weapon"},
		{game.ItemView{Kind: "wood", Qty: 25}, "Wood ×25", "material"},
		{game.ItemView{Kind: "parka", Qty: 1}, "Parka", "apparel"},
		{game.ItemView{Kind: "meal", Qty: 2}, "Meal ×2", "food"},
		{game.ItemView{Kind: "dustbuck corpse", Qty: 1}, "Dustbuck corpse", "body"},
	}
	for _, c := range cases {
		if got := itemTitle(c.it); got != c.title {
			t.Errorf("title(%+v) = %q, want %q", c.it, got, c.title)
		}
		if got := itemCategory(c.it); got != c.cat {
			t.Errorf("category(%s) = %q, want %q", c.it.Kind, got, c.cat)
		}
	}
	for _, c := range []struct {
		it   game.ItemView
		want string
	}{
		{game.ItemView{Rotten: true}, "Rotten"},
		{game.ItemView{Dried: true}, "Dried"},
		{game.ItemView{FreshDays: 4}, "Fresh for 4 days"},
	} {
		if got := itemLines(c.it, true)[0]; got != c.want {
			t.Errorf("condition = %q, want %q", got, c.want)
		}
	}
	if got := itemLines(game.ItemView{FreshDays: -1}, true); len(got) != 1 {
		t.Fatalf("non-spoiling item lines = %v", got)
	}
}

func TestItemFoodAndApparelRows(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnStack(item.Meal, 2, 6, 6)
	selectItemTile(t, a, id, 6, 6)
	it, _ := a.selectedItem()
	if rows := a.itemRows(it, true); len(rows) != 1 || rows[0].Verb != "Eat:" {
		t.Fatalf("meal rows = %+v", rows)
	}
	pid, _ := g.SpawnStack(item.Parka, 1, 7, 7)
	selectItemTile(t, a, pid, 7, 7)
	it, _ = a.selectedItem()
	if rows := a.itemRows(it, true); len(rows) != 1 || rows[0].Verb != "Wear:" {
		t.Fatalf("parka rows = %+v", rows)
	}
	a.applyItemClick("item.act.99")
	a.applyItemClick("item.act.x")
	a.applyItemClick("item.bogus")
}

func TestItemPanelClosesWhenItemGone(t *testing.T) {
	t.Parallel()
	a, _, _ := liveColony(t, 0)
	a.selItem, a.hasItem = 9999, true
	if len(a.itemButtons()) != 0 || a.hasItem {
		t.Fatal("panel must close")
	}
	a.applyItemClick("item.close")
}

func TestInspectorsDrawWithoutArt(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	pid, _ := g.SpawnCrop("corn", 8, 8, 40)
	iid, _ := g.SpawnStack(item.Rifle, 1, 10, 10)
	screen := ebiten.NewImage(640, 480)
	selectPlantTile(t, a, pid, 8, 8)
	a.drawPlantInspector(screen)
	selectItemTile(t, a, iid, 10, 10)
	a.drawItemInspector(screen)
	if got := capitalize(""); got != "" {
		t.Fatalf("capitalize empty = %q", got)
	}
}
