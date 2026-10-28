package selection

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

type fakeShell struct {
	w      game.WorldView
	pawns  []game.PawnView
	pawn   entity.EntityID
	has    bool
	cursor [2]int
	sent   []command.Command
}

func (f *fakeShell) World() game.WorldView             { return f.w }
func (f *fakeShell) Pawns() []game.PawnView            { return f.pawns }
func (f *fakeShell) MapSize() (int, int)               { return 20, 20 }
func (f *fakeShell) Tick() clock.Tick                  { return 0 }
func (f *fakeShell) Date() clock.Date                  { return clock.Date{} }
func (f *fakeShell) Select(id entity.EntityID)         { f.pawn, f.has = id, true }
func (f *fakeShell) Selected() (entity.EntityID, bool) { return f.pawn, f.has }
func (f *fakeShell) ClearSelect()                      { f.has = false }
func (f *fakeShell) TileAt(mx, my int) (int, int)      { return mx / 10, my / 10 }
func (f *fakeShell) TilePx() float64                   { return 10 }
func (f *fakeShell) WorldToScreen(x, y int) (float32, float32) {
	return float32(x * 10), float32(y * 10)
}
func (f *fakeShell) WorldToScreenF(x, y float64) (float32, float32) {
	return float32(x * 10), float32(y * 10)
}
func (f *fakeShell) CenterOn(int, int)         {}
func (f *fakeShell) Present() uint64           { return 0 }
func (f *fakeShell) Size() (int, int)          { return 200, 200 }
func (f *fakeShell) Cursor() (int, int)        { return f.cursor[0], f.cursor[1] }
func (f *fakeShell) MapCursor() (int, int)     { return f.cursor[0], f.cursor[1] }
func (f *fakeShell) MapScale() float64         { return 1 }
func (f *fakeShell) Enqueue(c command.Command) { f.sent = append(f.sent, c) }

type fakeHost struct{ zone uint16 }

func (fakeHost) AnimalsCovering(game.WorldView, int, int) []Cand         { return nil }
func (fakeHost) PawnUnderCursor(int, int) (entity.EntityID, bool)        { return 0, false }
func (fakeHost) SelectZoneLabel(game.WorldView, int, int, int, int) bool { return false }
func (fakeHost) NoteCell([2]int)                                         {}
func (h *fakeHost) SetZone(id uint16, on bool)                           { h.zone = id }
func (fakeHost) PowerShown() bool                                        { return false }
func (fakeHost) HandleOrders()                                           {}
func (fakeHost) Frame() (uint64, float64)                                { return 1, 1 }

func newModel() (*Model, *fakeShell) {
	sh := &fakeShell{w: game.WorldView{
		Items:  []game.ItemView{{ID: 5, X: 2, Y: 2, Kind: "steel"}, {ID: 6, X: 3, Y: 3}},
		Plants: []game.PlantView{{ID: 7, X: 2, Y: 2}},
	}}
	return New(sh, &fakeHost{}), sh
}

func TestClickCyclesAndClearsOthers(t *testing.T) {
	t.Parallel()
	m, sh := newModel()
	sh.cursor = [2]int{25, 25}
	m.ClickSelect()
	if _, ok := m.SelectedItem(); !ok {
		t.Fatal("an item comes before the plant under it")
	}
	m.ClickSelect()
	if _, ok := m.SelectedPlant(); !ok || m.HasItem {
		t.Fatal("a second click cycles to the plant and clears the item")
	}
	m.ClickSelect()
	if _, ok := m.SelectedItem(); !ok {
		t.Fatal("the cycle wraps")
	}
	sh.cursor = [2]int{155, 155}
	m.ClickSelect()
	if _, ok := m.Selected(); ok {
		t.Fatal("empty ground clears")
	}
}

func TestGroupSetToggleAndClicks(t *testing.T) {
	t.Parallel()
	m, sh := newModel()
	m.SetGroup(Item, []entity.EntityID{5, 6, 5})
	if !m.Grouped() || !m.InGroup(Item, 6) || len(m.SelectedItems()) != 2 {
		t.Fatalf("group %+v", m.Group)
	}
	m.ToggleInSet(Cand{Kind: Item, ID: 6})
	if m.Grouped() || !m.HasItem || m.SelItem != 5 {
		t.Fatal("a group of one is the single selection")
	}
	m.ApplyBox(Item, []entity.EntityID{6}, true)
	m.ApplyGroupClick("group.forbid")
	if len(sh.sent) != 2 {
		t.Fatalf("forbid reaches every member, sent %d", len(sh.sent))
	}
	m.ApplyGroupClick("group.pick.6")
	if m.Grouped() || m.SelItem != 6 {
		t.Fatal("pick selects the one")
	}
	m.SetGroup(Item, []entity.EntityID{5, 6})
	m.ApplyGroupClick("group.close")
	if m.Grouped() {
		t.Fatal("close drops the group")
	}
	if k, _ := m.SelectionSet(); k != None {
		t.Fatal("nothing selected")
	}
}
