package hud

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// A toast lives ToastFrames Updates; only the newest three are kept.
func TestToastsExpireAndKeepTheNewestThree(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	for _, s := range []string{"a", "b", "c", "d"} {
		c.Toast(s)
	}
	ts := c.LiveToasts()
	if len(ts) != 3 || ts[0].Text != "b" || ts[2].Text != "d" {
		t.Fatalf("live toasts = %+v", ts)
	}
	f.env.Updates = ToastFrames - 1
	if len(c.LiveToasts()) != 3 {
		t.Fatal("a toast left before its time")
	}
	f.env.Updates = ToastFrames
	if ts := c.LiveToasts(); len(ts) != 0 {
		t.Fatalf("expired toasts still live: %+v", ts)
	}
}

// A category header folds and unfolds its group; a row click does nothing.
func TestResourceHeadersFold(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	f.set.Gameplay.CategorizedResources = true
	f.w.Stored = []game.StoredResourceView{
		{Kind: "rice", Label: "Rice", Category: "Food", Qty: 20},
		{Kind: "wood", Label: "Wood", Category: "Materials", Qty: 50},
	}
	if n := len(c.Res().Rows()); n != 4 {
		t.Fatalf("%d rows, want two headers and two items", n)
	}
	c.Res().Click("res.row.rice")
	c.Res().Click(resCatPrefix + "Food")
	rows := c.Res().Rows()
	if len(rows) != 3 || !rows[0].Header || rows[0].Qty != 20 || !c.Res().Collapsed()["Food"] {
		t.Fatalf("folded Food: %+v", rows)
	}
	c.Res().Click(resCatPrefix + "Food")
	if n := len(c.Res().Rows()); n != 4 {
		t.Fatalf("unfolded: %d rows", n)
	}
}

// The alerts are built once per view: the same read models give the same
// list; a new presentation frame builds it again.
func TestCurrentAlertsAreMemoisedPerView(t *testing.T) {
	t.Parallel()
	pawns, w := rightWorld()
	c, f := newFake(pawns...)
	f.w = w
	a := c.Alerts().Current()
	if len(a) == 0 || &c.Alerts().Current()[0] != &a[0] {
		t.Fatalf("not memoised: %+v", a)
	}
	f.present++
	if b := c.Alerts().Current(); &b[0] == &a[0] || len(b) != len(a) {
		t.Fatal("a new frame reuses the old list")
	}
	f.env.Game = nil
	if c.Alerts().Current() != nil || c.Alerts().Count() != 0 {
		t.Fatal("alerts off the colony")
	}
}

// An alert click centres its targets in turn; a right-click goes back.
func TestAlertFocusCyclesTargets(t *testing.T) {
	t.Parallel()
	pawns, w := rightWorld()
	c, f := newFake(pawns...)
	f.w = w
	c.Alerts().Focus("alert.raid")
	c.Alerts().Focus("alert.raid")
	c.Alerts().Focus("alert.raid")
	c.Alerts().cycle("raid", -1)
	want := [][2]float64{{30.5, 30.5}, {50.5, 40.5}, {30.5, 30.5}, {50.5, 40.5}}
	if len(f.centred) != len(want) {
		t.Fatalf("centred %v", f.centred)
	}
	for i := range want {
		if f.centred[i] != want[i] {
			t.Fatalf("centred %v, want %v", f.centred, want)
		}
	}
}

// Two clicks on the same colonist within the window make a double click,
// which is spent.
func TestBarDoubleClick(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	f.env.Updates = 10 // Updates 0 marks "no click yet"
	if c.Bar().DoubleClick(7) {
		t.Fatal("one click is a double click")
	}
	f.env.Updates = 15
	if !c.Bar().DoubleClick(7) || c.Bar().DoubleClick(7) {
		t.Fatal("double click not seen once")
	}
	f.env.Updates = 16
	if c.Bar().DoubleClick(8) {
		t.Fatal("another colonist makes a double click")
	}
}

// Letters stack in their rect, the newest at the bottom; a right-click on
// one asks the game to dismiss it.
func TestLetterRowsAndDismiss(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	f.w.Letters = []game.LetterView{{ID: 1, Title: "Old"}, {ID: 2, Title: "New", Kind: "threat"}}
	f.lay.Letters = gfx.Rect{X: 900, Y: 400, W: 200, H: 200}
	f.lay.LetterRows = 4
	rows := c.Right().LetterRows(f.lay)
	if len(rows) != 2 || rows[1].Letter.ID != 2 || rows[1].R.Bottom() != f.lay.Letters.Bottom() {
		t.Fatalf("rows = %+v", rows)
	}
	r := rows[0].R
	if !c.Right().RightClick(int(r.X)+5, int(r.Y)+5) || len(f.cmds) != 1 ||
		f.cmds[0].Kind != command.DismissLetter || f.cmds[0].Target != 1 {
		t.Fatalf("dismiss: %+v", f.cmds)
	}
	if LetterColor("threat") != LetterKinds[0].Clr || LetterKind("nonsense") != 2 {
		t.Fatal("letter kinds")
	}
}
