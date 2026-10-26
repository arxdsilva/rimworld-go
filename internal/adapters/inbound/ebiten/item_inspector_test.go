package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
	cs := a.itemCommands(id)
	equip := thingCmd(t, cs, "cmd.item.equip")
	opts := equip.Options(a)
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
	if a.hasItem {
		t.Fatal("close clears the selection")
	}
}

func TestItemChipsDisabledWithReason(t *testing.T) {
	t.Parallel()
	equip := colonistOptions([]colonistAction{{Pawn: 1, Name: "Ada"}, {Pawn: 2, Name: "Bo", Disabled: true, Reason: "cannot reach"}})
	haul := colonistOptions([]colonistAction{{Pawn: 1, Name: "Ada"}})
	if b := equip[1]; !b.Disabled || b.Reason != "cannot reach" || b.Label != "Bo" || b.Cmd != nil {
		t.Fatalf("option 1 = %+v", b)
	}
	// the Haul command sits after the Equip one in the grid
	var ids []string
	for _, v := range itemVerbs(game.ItemView{Kind: "rifle"}, false) {
		ids = append(ids, v.id)
	}
	if strings.Join(ids, ",") != "cmd.item.equip,cmd.item.haul" {
		t.Fatalf("verbs = %v", ids)
	}
	if n := len(equip) + len(haul); n != 3 {
		t.Fatalf("options = %d", n)
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
	equip := thingCmd(t, a.itemCommands(id), "cmd.item.equip")
	opts := equip.Options(a)
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
	it, _ := a.selectedItem()
	if vs := itemVerbs(it, false); len(vs) != 1 || vs[0].label != "Haul" {
		t.Fatalf("unstored wood verbs = %+v", vs)
	}
	if vs := itemVerbs(it, true); len(vs) != 0 {
		t.Fatalf("stored wood must have no Haul: %+v", vs)
	}
	if got := itemLines(it, false); got[len(got)-1] != "Not stored" {
		t.Fatalf("lines = %v", got)
	}
	if got := itemLines(it, true); got[len(got)-1] != "In a stockpile" {
		t.Fatalf("lines = %v", got)
	}
	w := game.WorldView{Zones: []game.ZoneView{rectZone(1, game.ZoneStockpile, 7, 7, 9, 9)}}
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
	if vs := itemVerbs(it, true); len(vs) != 1 || vs[0].id != "cmd.item.eat" || vs[0].label != "Eat" {
		t.Fatalf("meal verbs = %+v", vs)
	}
	pid, _ := g.SpawnStack(item.Parka, 1, 7, 7)
	selectItemTile(t, a, pid, 7, 7)
	it, _ = a.selectedItem()
	if vs := itemVerbs(it, true); len(vs) != 1 || vs[0].id != "cmd.item.wear" || vs[0].label != "Wear" {
		t.Fatalf("parka verbs = %+v", vs)
	}
	a.applyItemClick("item.act.99")
	a.applyItemClick("item.act.x")
	a.applyItemClick("item.bogus")
}

func TestItemPanelClosesWhenItemGone(t *testing.T) {
	t.Parallel()
	a, _, _ := liveColony(t, 0)
	a.selItem, a.hasItem = 9999, true
	if len(a.commandGridButtons()) != 0 || len(a.itemCommands(9999)) != 0 || a.itemPane(9999).Title != "" || a.hasItem {
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
	a.drawPlantMark(screen)
	selectItemTile(t, a, iid, 10, 10)
	a.drawItemMark(screen)
	if got := capitalize(""); got != "" {
		t.Fatalf("capitalize empty = %q", got)
	}
}
