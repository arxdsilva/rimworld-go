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
	b, ok := hasButton(a.itemButtons(), "item.haulmark")
	if !ok || b.label != "Mark for hauling" {
		t.Fatalf("unclaimed unmarked stack: button = %+v", b)
	}
	click(t, a, "item.haulmark")
	if c := lastCmd(t, s); c.Kind != command.SetHaulMark || c.Target != 21 || !c.Enabled {
		t.Fatalf("cmd = %+v, want SetHaulMark(21) on", c)
	}
	s.world.Items[0].HaulMarked = true
	if b, _ := hasButton(a.itemButtons(), "item.haulmark"); b.label != "Unmark" {
		t.Fatalf("marked label = %q", b.label)
	}
	click(t, a, "item.haulmark")
	if c := lastCmd(t, s); c.Kind != command.SetHaulMark || c.Target != 21 || c.Enabled {
		t.Fatalf("cmd = %+v, want SetHaulMark(21) off", c)
	}
	// the Forbid button stays beside it
	if _, ok := hasButton(a.itemButtons(), "item.forbid"); !ok {
		t.Fatal("Forbid button gone")
	}
}

func TestItemHaulMarkButtonHiddenForColonyStacks(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: foundItems()}
	a.selectCand(selCand{selItem, 23})
	if _, ok := hasButton(a.itemButtons(), "item.haulmark"); ok {
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
	if rows := a.itemRows(it, false); len(rows) != 1 || rows[0].Verb != "Haul:" {
		t.Fatalf("a found stack still offers the forced Haul chip: %+v", rows)
	}
	click(t, a, "item.act.0")
	if c := lastCmd(t, s); c.Kind != command.Prioritize || c.Order != command.OrderHaul {
		t.Fatalf("chip cmd = %+v", c)
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

// At UI scale 1.5 the item panel is laid out in logical pixels: every
// button sits inside the panel, its label fits, and the worst-case body
// (every status line) ends above the chip rows.
func TestItemInspectorAtScale1_5(t *testing.T) {
	withUIScale(t, 1.5)
	a, s := stubColony(t)
	s.world = game.WorldView{Items: []game.ItemView{
		{ID: 21, Kind: "rifle", Qty: 1, X: 2, Y: 2, Unclaimed: true, Forbidden: true, FreshDays: 12},
	}}
	s.orders = []game.OrderOption{
		{Label: "Equip", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip}},
		{Label: "Haul", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderHaul}},
	}
	s.pawns = []game.PawnView{{ID: 1, Name: "Ada", X: 5, Y: 5, Status: "alive"}, {ID: 2, Name: "Bo", X: 6, Y: 5, Status: "alive"}}
	a.selectCand(selCand{selItem, 21})
	it, _ := a.selectedItem()
	r := itemRect()
	bs := a.itemButtons()
	if _, ok := hasButton(bs, "item.haulmark"); !ok {
		t.Fatalf("no mark button: %+v", bs)
	}
	for _, b := range bs {
		if b.x < r.x || b.y < r.y || b.x+b.w > r.x+r.w || b.y+b.h > r.y+r.h {
			t.Errorf("%s (%v,%v %vx%v) leaves the panel %+v", b.id, b.x, b.y, b.w, b.h, r)
		}
		if w, _ := measure(b.label, tsBody); w > float64(b.w)-4 && !strings.HasPrefix(b.id, "item.act.") {
			t.Errorf("%s label %q (%.0fpx) does not fit %.0fpx", b.id, b.label, w, b.w)
		}
		for _, o := range bs {
			if o.id < b.id && b.x < o.x+o.w && o.x < b.x+b.w && b.y < o.y+o.h && o.y < b.y+b.h {
				t.Errorf("%s overlaps %s", b.id, o.id)
			}
		}
	}
	// body lines stay clear of the chip rows
	rows := a.itemRows(it, false)
	if len(rows) == 0 {
		t.Fatal("no chip rows")
	}
	bodyEnd := float64(r.y) + 62
	for _, l := range itemLines(it, false) {
		bodyEnd += float64(len(wrapPx(l, tsSmall, float64(r.w)-24))) * float64(lineStep(tsSmall))
	}
	chipTop := float64(r.y+r.h) - 30 - float64(len(rows)-1)*26
	if bodyEnd > chipTop {
		t.Errorf("body ends at %.0f, chips start at %.0f", bodyEnd, chipTop)
	}
	// the category word stays clear of the mark button
	mb, _ := hasButton(bs, "item.haulmark")
	if w, _ := measure(itemCategory(it), tsSmall); float64(r.x)+60+w > float64(mb.x) {
		t.Errorf("category (%.0fpx) runs under the mark button at %v", w, mb.x)
	}
	// drawing both states does not panic
	img := ebiten.NewImage(1280, 720)
	a.drawItemInspector(img)
	s.world.Items[0].HaulMarked = true
	a.drawItemInspector(img)
}

func TestGroupInspectorAtScale1_5(t *testing.T) {
	withUIScale(t, 1.5)
	a, s := stubColony(t)
	s.world = game.WorldView{Items: foundItems()}
	a.setGroup(selItem, []entity.EntityID{21, 22, 23})
	r := groupRect(3)
	for _, b := range a.groupButtons() {
		if b.x < r.x || b.y < r.y || b.x+b.w > r.x+r.w || b.y+b.h > r.y+r.h {
			t.Errorf("%s leaves the group panel", b.id)
		}
		if strings.HasPrefix(b.id, "group.pick.") || strings.HasSuffix(b.id, ".close") {
			continue
		}
		if w, _ := measure(b.label, tsBody); w > float64(b.w)-4 {
			t.Errorf("%s label %q (%.0fpx) does not fit %.0fpx", b.id, b.label, w, b.w)
		}
		for _, o := range a.groupButtons() {
			if o.id < b.id && b.x < o.x+o.w && o.x < b.x+b.w && b.y < o.y+o.h && o.y < b.y+b.h {
				t.Errorf("%s overlaps %s", b.id, o.id)
			}
		}
	}
	a.drawGroupInspector(ebiten.NewImage(1280, 720))
}

// haulOverlay draws one live-tier frame of items over a stub world with a
// fake haul-mark sprite and reports whether the mark was drawn.
func haulOverlay(t *testing.T, tile float64, items ...game.ItemView) bool {
	t.Helper()
	w := lodWorld(64, 64)
	w.Items = items
	r, _ := lodRenderer(w, 64, 64)
	sp := fakeSprite([2]int{128, 128}, pt{0, 0}, nil)
	r.art.sprites[haulMarkKey] = sp
	lodFrame(r, tile)
	return sp.img != nil
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
