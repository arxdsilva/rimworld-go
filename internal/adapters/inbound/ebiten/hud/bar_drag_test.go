package hud

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// dispShell is the fake shell that also takes intents.
type dispShell struct {
	*fakeShell
	intents []ports.Intent
}

func (d *dispShell) Do(i ports.Intent) { d.intents = append(d.intents, i) }

func dragBar(pawns ...game.PawnView) (*Chrome, *dispShell) {
	f := &fakeShell{env: Env{Game: fakeGame{pawns: pawns, colonists: pawns}, ViewCached: true, Colony: true, Updates: 1}, pawns: pawns}
	f.set.Gameplay.ShowColonistBar = true
	d := &dispShell{fakeShell: f}
	c := New(d)
	lay(c, f)
	return c, d
}

func rightFrame(x, y int, press, down, release bool) *inputframe.Frame {
	var raw inputframe.Raw
	raw.Press[inputframe.Right], raw.Down[inputframe.Right], raw.Release[inputframe.Right] = press, down, release
	f := inputframe.NewFrame(raw)
	f.Ui = inputframe.UIPoint{Mx: x, My: y}
	return &f
}

func centre(r gfx.Rect) (int, int) { return int(r.X + r.W/2), int(r.Y + r.H/2) }

func alive(id entity.EntityID, m game.MapID, order int) game.PawnView {
	return game.PawnView{ID: id, Map: m, Status: "alive", BarOrder: order}
}

// drag presses on slot a, holds, and releases at (x, y).
func drag(c *Chrome, d *dispShell, a int, x, y int) bool {
	ax, ay := centre(d.lay.BarSlots[a])
	c.Bar().HandleRight(rightFrame(ax, ay, true, true, false))
	c.Bar().HandleRight(rightFrame(x, y, false, true, false))
	return c.Bar().HandleRight(rightFrame(x, y, false, false, true))
}

func TestBarSortsByBarOrderStable(t *testing.T) {
	c, _ := dragBar(alive(1, 0, 2), alive(2, 0, 0), alive(3, 0, 2), alive(4, 0, 1))
	var got []entity.EntityID
	for _, p := range c.Bar().Pawns() {
		got = append(got, p.ID)
	}
	want := []entity.EntityID{2, 4, 1, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order %v want %v", got, want)
		}
	}
}

func TestBarSortsEachMapGroup(t *testing.T) {
	c, _ := twoMaps(0, alive(1, 0, 5), alive(2, 1, 9), alive(3, 0, 1), alive(4, 1, 2))
	ps := c.Bar().Pawns()
	want := []entity.EntityID{3, 1, 4, 2}
	for i := range want {
		if ps[i].ID != want[i] {
			t.Fatalf("order %v", ps)
		}
	}
}

func TestRightDragOverAnotherReorders(t *testing.T) {
	c, d := dragBar(alive(1, 0, 0), alive(2, 0, 1), alive(3, 0, 2))
	x, y := centre(d.lay.BarSlots[2])
	if !drag(c, d, 0, x, y) {
		t.Fatal("the bar did not own the release")
	}
	if len(d.cmds) != 1 || d.cmds[0].Kind != command.SetColonistOrder || d.cmds[0].Pawn != 1 || d.cmds[0].Count != 2 {
		t.Fatalf("cmds %+v", d.cmds)
	}
	if len(d.intents) != 0 {
		t.Fatalf("intents %v", d.intents)
	}
}

func TestRightClickOnItselfOpensTheWorld(t *testing.T) {
	c, d := dragBar(alive(1, 0, 0), alive(2, 0, 1))
	x, y := centre(d.lay.BarSlots[1])
	drag(c, d, 1, x+4, y) // 4 px: still a click
	if len(d.cmds) != 0 || len(d.intents) != 1 {
		t.Fatalf("cmds %v intents %v", d.cmds, d.intents)
	}
	if ow, ok := d.intents[0].(ports.OpenWorld); !ok || ow.Select != "map:0" {
		t.Fatalf("intent %#v", d.intents[0])
	}
}

func TestRightDragOnItselfFarCancels(t *testing.T) {
	c, d := dragBar(alive(1, 0, 0), alive(2, 0, 1))
	ax, ay := centre(d.lay.BarSlots[0])
	c.Bar().HandleRight(rightFrame(ax, ay, true, true, false))
	c.Bar().HandleRight(rightFrame(ax+9, ay, false, true, false)) // wandered off ...
	c.Bar().HandleRight(rightFrame(ax, ay, false, false, true))   // ... and back
	if len(d.cmds) != 0 || len(d.intents) != 0 {
		t.Fatalf("cmds %v intents %v", d.cmds, d.intents)
	}
}

func TestRightDragElsewhereCancels(t *testing.T) {
	c, d := dragBar(alive(1, 0, 0), alive(2, 0, 1))
	if !drag(c, d, 0, 5, 700) {
		t.Fatal("the gesture was not owned to its end")
	}
	if len(d.cmds) != 0 || len(d.intents) != 0 {
		t.Fatalf("cmds %v intents %v", d.cmds, d.intents)
	}
	if c.Bar().HandleRight(rightFrame(5, 700, false, false, false)) {
		t.Fatal("still dragging after the release")
	}
}

func TestRightDragToAnotherMapHints(t *testing.T) {
	_, f := twoMaps(0, alive(1, 0, 0), alive(2, 1, 0))
	d := &dispShell{fakeShell: f}
	c := New(d)
	lay(c, f)
	x, y := centre(f.lay.BarSlots[1])
	drag(c, d, 0, x, y)
	if len(f.cmds) != 0 || len(d.intents) != 0 {
		t.Fatalf("cmds %v intents %v", f.cmds, d.intents)
	}
	if len(c.LiveToasts()) != 1 {
		t.Fatalf("toasts %v", c.LiveToasts())
	}
}

func TestMoreSlotTakesNoPartInTheDrag(t *testing.T) {
	var ps []game.PawnView
	for i := 1; i <= 40; i++ {
		ps = append(ps, alive(entity.EntityID(i), 0, i))
	}
	c, d := dragBar(ps...)
	if d.lay.BarMore == 0 {
		t.Skip("the bar does not overflow at this size")
	}
	more := d.lay.BarSlots[len(d.lay.BarSlots)-1]
	mx, my := centre(more)
	if c.Bar().HandleRight(rightFrame(mx, my, true, true, false)) {
		t.Fatal("a press on +N started a drag")
	}
	x, y := centre(d.lay.BarSlots[0])
	drag(c, d, 0, mx, my)
	_ = x
	_ = y
	if len(d.cmds) != 0 {
		t.Fatalf("cmds %v", d.cmds)
	}
}

func TestPressOffTheBarIsNotTaken(t *testing.T) {
	c, _ := dragBar(alive(1, 0, 0))
	if c.Bar().HandleRight(rightFrame(5, 700, true, true, false)) {
		t.Fatal("took a press that was not on a portrait")
	}
}

// The bar lists each map's colonists together while the order is one rank
// across the maps: a drag sends the target's rank, not its place in the list.
func TestRightDragInALaterGroupSendsTheRank(t *testing.T) {
	pawns := []game.PawnView{alive(1, 0, 0), alive(2, 1, 1), alive(3, 0, 2), alive(4, 1, 3)}
	c, d := dragBar(pawns...)
	d.env.Game = fakeGame{pawns: pawns, colonists: pawns, maps: []game.MapView{{ID: 0, Label: "Colony"}, {ID: 1, Label: "Mine site"}}}
	d.env.Updates++ // the bar memoizes its list per Update
	lay(c, d.fakeShell)
	if got := c.Bar().Pawns(); got[2].ID != 2 || got[3].ID != 4 {
		t.Fatalf("list %d, %d", got[2].ID, got[3].ID)
	}
	x, y := centre(d.lay.BarSlots[2])
	drag(c, d, 3, x, y)
	if len(d.cmds) != 1 || d.cmds[0].Pawn != 4 || d.cmds[0].Count != 1 {
		t.Fatalf("cmds %+v", d.cmds)
	}
}
