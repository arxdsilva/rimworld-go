package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017 §5: the "Haul things" Architect tool, the item panel's mark
// button and status line, the group panel's "Mark for hauling" button and
// the world overlay on marked stacks.

const haulFoundLine = "Found here. Colonists won't haul it until you mark it."

// foundItems is a world with a found stack, a marked one and a colony one.
func foundItems() []game.ItemView {
	return []game.ItemView{
		{ID: 21, Kind: "wood", Qty: 5, X: 2, Y: 2, Unclaimed: true},
		{ID: 22, Kind: "steel", Qty: 3, X: 3, Y: 2, Unclaimed: true, HaulMarked: true},
		{ID: 23, Kind: "wood", Qty: 9, X: 4, Y: 2},
	}
}

func TestHaulToolEmitsDesignateHaul(t *testing.T) {
	t.Parallel()
	tool, ok := toolByID("haul")
	if !ok || tool.category != "orders" || tool.icon != "order_haul" || tool.mode != placeArea ||
		tool.cmd != command.DesignateHaul || !tool.enable || tool.label != "Haul things" {
		t.Fatalf("haul tool = %+v", tool)
	}
	if want := "Mark things found on the map for hauling. Colonists haul the colony's own goods by themselves; anything else waits for this mark."; tool.desc != want {
		t.Fatalf("desc = %q", tool.desc)
	}
	a, s := stubColony(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.orders")
	click(t, a, "arch.tool.haul")
	a.archX0, a.archY0, a.archX1, a.archY1 = 1, 2, 3, 4
	a.archDrag = true
	a.submitArchitect()
	c := lastCmd(t, s)
	if c.Kind != command.DesignateHaul || !c.Enabled || c.X != 1 || c.Y != 2 || c.X2 != 3 || c.Y2 != 4 {
		t.Fatalf("haul cmd = %+v, want DesignateHaul(1,2)-(3,4) marking", c)
	}
}

func TestItemHaulMarkButtonEmitsAndFlips(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: foundItems()}
	a.selectCand(selCand{selItem, 21})
	b, ok := thingFind(a.itemCommands(21), "item.haulmark")
	if !ok || b.Label != "Mark for hauling" {
		t.Fatalf("unclaimed unmarked stack: command = %+v", b)
	}
	click(t, a, "item.haulmark")
	if c := lastCmd(t, s); c.Kind != command.SetHaulMark || c.Target != 21 || !c.Enabled {
		t.Fatalf("cmd = %+v, want SetHaulMark(21) on", c)
	}
	s.world.Items[0].HaulMarked = true
	if b, _ := thingFind(a.itemCommands(21), "item.haulmark"); b.Label != "Unmark" {
		t.Fatalf("marked label = %q", b.Label)
	}
	click(t, a, "item.haulmark")
	if c := lastCmd(t, s); c.Kind != command.SetHaulMark || c.Target != 21 || c.Enabled {
		t.Fatalf("cmd = %+v, want SetHaulMark(21) off", c)
	}
	// the Forbid command stays beside it
	if _, ok := thingFind(a.itemCommands(21), "item.forbid"); !ok {
		t.Fatal("Forbid command gone")
	}
}

func TestItemHaulMarkButtonHiddenForColonyStacks(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: foundItems()}
	a.selectCand(selCand{selItem, 23})
	if _, ok := thingFind(a.itemCommands(23), "item.haulmark"); ok {
		t.Fatal("a claimed stack needs no mark button")
	}
	// a stale click does nothing
	a.applyItemClick("item.haulmark")
	if len(s.cmds) != 0 {
		t.Fatalf("claimed stack emitted %+v", s.cmds)
	}
}

func TestItemFoundLineOnlyForUnclaimed(t *testing.T) {
	t.Parallel()
	found, marked, colony := foundItems()[0], foundItems()[1], foundItems()[2]
	if ls := itemLines(found, false); !contains(ls, haulFoundLine) {
		t.Fatalf("unclaimed lines = %q", ls)
	}
	if ls := itemLines(marked, false); contains(ls, haulFoundLine) || !contains(ls, "Found here. Marked: colonists will haul it.") {
		t.Fatalf("marked lines = %q", ls)
	}
	ls := itemLines(colony, false)
	for _, l := range ls {
		if strings.Contains(l, "Found here") {
			t.Fatalf("colony stack shows %q", l)
		}
	}
	if ls[len(ls)-1] != "Not stored" {
		t.Fatalf("storage stays last: %q", ls)
	}
}

func TestItemInspectorKeepsForcedHaulChip(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: foundItems()}
	s.pawns = []game.PawnView{{ID: 1, Name: "Ada", X: 5, Y: 5, Status: "alive"}}
	s.orders = []game.OrderOption{{Label: "Haul", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderHaul, Pawn: 1}}}
	a.selectCand(selCand{selItem, 21})
	it, _ := a.selectedItem()
	if vs := itemVerbs(it, false); len(vs) != 1 || vs[0].id != "cmd.item.haul" {
		t.Fatalf("a found stack still offers the forced Haul command: %+v", vs)
	}
	click(t, a, "cmd.item.haul")
	if c := lastCmd(t, s); c.Kind != command.Prioritize || c.Order != command.OrderHaul {
		t.Fatalf("haul option cmd = %+v", c)
	}
}

func TestGroupMarkForHaulingButton(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: append(foundItems(),
		game.ItemView{ID: 24, Kind: "wood", Qty: 2, X: 5, Y: 2, Unclaimed: true})}
	a.setGroup(selItem, []entity.EntityID{21, 22, 23, 24})
	if !a.grouped() {
		t.Fatal("no group")
	}
	b, ok := hasButton(a.groupButtons(), "group.haulmark")
	if !ok || b.label != "Mark for hauling" {
		t.Fatalf("button = %+v", b)
	}
	click(t, a, "group.haulmark")
	got := cmdsOf(s, command.SetHaulMark)
	if len(got) != 2 || got[0].Target != 21 || got[1].Target != 24 || !got[0].Enabled || !got[1].Enabled {
		t.Fatalf("marks only the found, unmarked stacks: %+v", s.cmds)
	}
	// once every found stack is marked the button goes away
	s.world.Items[0].HaulMarked = true
	s.world.Items[3].HaulMarked = true
	if _, ok := hasButton(a.groupButtons(), "group.haulmark"); ok {
		t.Fatal("nothing left to mark")
	}
	// the Forbid/Allow pair is unchanged
	for _, id := range []string{"group.forbid", "group.allow"} {
		if _, ok := hasButton(a.groupButtons(), id); !ok {
			t.Fatalf("%s gone", id)
		}
	}
}

func TestGroupMarkButtonHiddenWithoutFoundStacks(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: []game.ItemView{
		{ID: 31, Kind: "wood", Qty: 1, X: 2, Y: 2},
		{ID: 32, Kind: "wood", Qty: 1, X: 3, Y: 2},
	}}
	a.setGroup(selItem, []entity.EntityID{31, 32})
	if !a.grouped() {
		t.Fatal("no group")
	}
	if _, ok := hasButton(a.groupButtons(), "group.haulmark"); ok {
		t.Fatal("a colony-only group has nothing to mark")
	}
}

// At UI scale 1.5 the item pane and its commands are laid out in logical
// pixels: every command has a grid cell right of the pane that overlaps no
// other, every label word fits its cell, the worst-case body (every status
// line) is in the pane, and drawing both states does not panic.
func TestItemInspectorAtScale1_5(t *testing.T) {
	withUIScale(t, 1.5)
	a, s := stubColony(t)
	s.world = game.WorldView{Items: []game.ItemView{
		{ID: 21, Kind: "rifle", Qty: 1, X: 2, Y: 2, Unclaimed: true, Forbidden: true, FreshDays: 12, RotRate: 500},
	}}
	s.orders = []game.OrderOption{
		{Label: "Equip", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip}},
		{Label: "Haul", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderHaul}},
	}
	s.pawns = []game.PawnView{{ID: 1, Name: "Ada", X: 5, Y: 5, Status: "alive"}, {ID: 2, Name: "Bo", X: 6, Y: 5, Status: "alive"}}
	a.selectCand(selCand{selItem, 21})
	cs := a.itemCommands(21)
	if _, ok := thingFind(cs, "item.haulmark"); !ok {
		t.Fatalf("no mark command: %v", thingIDs(cs))
	}
	sw, sh := a.size()
	l := computeHUDLayout(sw, sh, hudState{PaneShown: true, Commands: len(cs)})
	if len(l.GridCells) != len(cs) {
		t.Fatalf("%d commands, %d cells", len(cs), len(l.GridCells))
	}
	for i, c := range cs {
		cell := l.GridCells[i]
		if cell.X < l.Pane.Right() || cell.Right() > l.Global.X || cell.Bottom() > l.Tabs.Y {
			t.Errorf("%s: cell %+v leaves the grid band", c.ID, cell)
		}
		for _, w := range strings.Fields(c.Label) {
			if tw := textW(w, tsSmall); tw > cell.W-8 {
				t.Errorf("%s label word %q (%.0fpx) does not fit its %.0fpx cell", c.ID, w, tw, cell.W)
			}
		}
		for j, o := range l.GridCells[:i] {
			if !cell.Intersect(o).Empty() {
				t.Errorf("%s overlaps command %d", c.ID, j)
			}
		}
	}
	// the worst-case body: every status line is on the pane, storage last
	sp := a.itemPane(21)
	for _, want := range []string{"Forbidden: colonists leave it alone", "Found here. Colonists won't haul it until you mark it.", "Not stored"} {
		if !contains(sp.Lines, want) {
			t.Errorf("pane lines %q lack %q", sp.Lines, want)
		}
	}
	if sp.Lines[len(sp.Lines)-1] != "Not stored" || sp.Subtitle != "weapon" {
		t.Errorf("pane = %+v", sp)
	}
	// drawing both states does not panic
	img := ebiten.NewImage(1280, 720)
	a.drawItemMark(img)
	s.world.Items[0].HaulMarked = true
	a.drawItemMark(img)
}

// At UI scale 1.5 a group of stacks is laid out in logical pixels: its
// members are rows of the pane, its commands sit in grid cells right of
// the pane (inside the grid band, overlapping nothing) and every label word
// fits its cell.
func TestGroupInspectorAtScale1_5(t *testing.T) {
	withUIScale(t, 1.5)
	a, s := stubColony(t)
	s.world = game.WorldView{Items: foundItems()}
	a.setGroup(selItem, []entity.EntityID{21, 22, 23})
	l := a.hud()
	if l.Pane.Empty() {
		t.Fatal("a group has a pane")
	}
	if spec := a.groupPane(); len(spec.Rows) != 3 {
		t.Fatalf("the pane lists the members: %+v", spec.Rows)
	}
	bs := a.groupButtons()
	if len(bs) == 0 {
		t.Fatal("no group commands")
	}
	for i, b := range bs {
		r := rect{X: b.x, Y: b.y, W: b.w, H: b.h}
		if r.X < l.Pane.Right() || r.Right() > l.Global.X || r.Bottom() > l.Tabs.Y {
			t.Errorf("%s (%+v) leaves the grid band", b.id, r)
		}
		for _, word := range strings.Fields(b.label) {
			if w := textW(word, tsSmall); w > b.w-8 {
				t.Errorf("%s label word %q (%.0fpx) does not fit %.0fpx", b.id, word, w, b.w)
			}
		}
		for _, o := range bs[:i] {
			if b.x < o.x+o.w && o.x < b.x+b.w && b.y < o.y+o.h && o.y < b.y+b.h {
				t.Errorf("%s overlaps %s", b.id, o.id)
			}
		}
	}
	img := ebiten.NewImage(1280, 720)
	a.drawPane(img) // the group's pane and merged command buttons draw
	a.drawCommandGrid(img)
}

// haulOverlay draws one live-tier frame of items over a stub world with a
// fake haul-mark sprite and reports whether the mark was drawn.
func haulOverlay(t *testing.T, tile float64, items ...game.ItemView) bool {
	t.Helper()
	w := lodWorld(64, 64)
	w.Items = items
	r, _ := lodRenderer(w, 64, 64)
	sp := fakeSprite([2]int{128, 128}, pt{0, 0}, nil)
	r.art.Sprites[haulMarkKey] = sp
	lodFrame(r, tile)
	return sp.Img != nil
}

func TestHaulOverlayDrawnForMarkedStackOnly(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		tile  float64
		items []game.ItemView
		want  bool
	}{
		{"marked stack", 64, []game.ItemView{{ID: 1, Kind: "wood", Qty: 4, X: 3, Y: 3, Unclaimed: true, HaulMarked: true}}, true},
		{"found, unmarked", 64, []game.ItemView{{ID: 1, Kind: "wood", Qty: 4, X: 3, Y: 3, Unclaimed: true}}, false},
		{"colony stack", 64, []game.ItemView{{ID: 1, Kind: "wood", Qty: 4, X: 3, Y: 3}}, false},
		{"marked but carried", 64, []game.ItemView{{ID: 1, Kind: "wood", Qty: 4, X: 3, Y: 3, Unclaimed: true, HaulMarked: true, Carried: true}}, false},
		{"marked off screen", 64, []game.ItemView{{ID: 1, Kind: "wood", Qty: 4, X: 60, Y: 60, Unclaimed: true, HaulMarked: true}}, false},
		{"marked among others", 64, []game.ItemView{
			{ID: 1, Kind: "wood", X: 3, Y: 3}, {ID: 2, Kind: "wood", X: 4, Y: 3, Unclaimed: true},
			{ID: 3, Kind: "wood", X: 5, Y: 3, Unclaimed: true, HaulMarked: true}}, true},
		{"map zoom has no item sprites, so no marks", 8, []game.ItemView{{ID: 1, Kind: "wood", Qty: 4, X: 3, Y: 3, Unclaimed: true, HaulMarked: true}}, false},
	} {
		if got := haulOverlay(t, c.tile, c.items...); got != c.want {
			t.Errorf("%s: mark drawn = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHaulOverlayWithoutArtDoesNotPanic(t *testing.T) {
	t.Parallel()
	w := lodWorld(64, 64)
	w.Items = []game.ItemView{{ID: 1, Kind: "wood", Qty: 4, X: 3, Y: 3, Unclaimed: true, HaulMarked: true}}
	r, _ := lodRenderer(w, 64, 64) // art without the sprite: nothing is drawn
	lodFrame(r, 64)
}
