package uikit

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
)

// traceStep runs one frame and returns what it drew (gfx's trace is
// global, so the callers are not parallel).
func traceStep(k *Kit, h *fakeHost, raw inputframe.Raw, declare func()) []gfx.TraceEvent {
	var ev []gfx.TraceEvent
	gfx.SetDrawTrace(func(e gfx.TraceEvent) { ev = append(ev, e) })
	defer gfx.SetDrawTrace(nil)
	step(k, h, raw, declare)
	return ev
}

// dims reports whether a frame filled the whole 1280 x 720 screen (the
// modal scrim).
func dims(ev []gfx.TraceEvent) bool {
	for _, e := range ev {
		if e.Kind == "fill" && e.X <= 0 && e.Y <= 0 && e.W >= 1280 && e.H >= 720 {
			return true
		}
	}
	return false
}

// textInside fails when a text run drawn inside one of rs leaves it.
func textInside(t *testing.T, ev []gfx.TraceEvent, rs []gfx.Rect) {
	t.Helper()
	for _, e := range ev {
		if e.Kind != "text" {
			continue
		}
		cx, cy := e.X+e.W/2, e.Y+e.H/2
		for _, r := range rs {
			if !r.Hit(int(cx), int(cy)) {
				continue
			}
			if e.X < r.X-0.5 || e.X+e.W > r.Right()+0.5 || e.Y < r.Y-0.5 || e.Y+e.H > r.Bottom()+0.5 {
				t.Errorf("text %q %v,%v %vx%v leaves its row %+v", e.Text, e.X, e.Y, e.W, e.H, r)
			}
		}
	}
}

// UIS-073: a frameless modal keeps its scrim but has no title bar and no X.
func TestFramelessModalHasNoTitleOrX(t *testing.T) {
	k, h := newTestKit()
	k.Push(Modal{ID: "f", Title: "Hidden", W: 620, H: 480, Frameless: true})
	ev := traceStep(k, h, at(0, 0), nil)
	if _, ok := k.Find("modal.f.close"); ok {
		t.Error("a frameless modal declares no X")
	}
	for _, e := range ev {
		if e.Kind == "text" && e.Text == "Hidden" {
			t.Error("a frameless modal draws no title")
		}
	}
	if !dims(ev) {
		t.Error("a frameless modal keeps its scrim (UIS-079)")
	}
	step(k, h, keys(0, 0), nil)
	if k.Top() != "f" {
		t.Fatal("it stays open with no input")
	}
}

// A popup does not dim; a click outside closes it (running OnClose) and
// does not reach the screen; a click inside keeps it.
func TestPopupClosesOnOutsideClickAndDoesNotDim(t *testing.T) {
	k, h := newTestKit()
	closed := 0
	anchor := gfx.Rect{X: 100, Y: 100, W: 200, H: 60}
	k.Push(Modal{ID: "p", W: 200, H: 60, Popup: true, Anchor: func() gfx.Rect { return anchor }, OnClose: func() { closed++ }})
	pressed := false
	screen := func() { pressed = k.Button("under", gfx.Rect{X: 600, Y: 600, W: 50, H: 50}, "U", BtnOpts{}) || pressed }
	ev := traceStep(k, h, at(0, 0), screen)
	if dims(ev) {
		t.Error("a popup draws no scrim")
	}
	if _, ok := k.Find("modal.p.close"); ok {
		t.Error("a popup has no X")
	}
	step(k, h, click(150, 120), screen)
	if k.Top() != "p" || closed != 0 {
		t.Fatalf("a click inside keeps it open: top %q closed %d", k.Top(), closed)
	}
	step(k, h, click(620, 620), screen)
	if k.Top() != "" || closed != 1 {
		t.Fatalf("a click outside closes it: top %q closed %d", k.Top(), closed)
	}
	if pressed {
		t.Error("the click that closed the popup reached the screen")
	}
}

// UIS-073: RowActions lays the buttons out as full-width rows MenuRowH
// apart, top to bottom in declaration order, keeping their ids; their
// text stays in its row.
func TestRowActionsStackAtMenuPitch(t *testing.T) {
	k, h := newTestKit()
	ran := ""
	m := Modal{ID: "l", W: 620, H: 480, Frameless: true, RowActions: true, Buttons: []ModalButton{
		{ID: "accept", Label: "Accept", OnClick: func() { ran += "a" }, Closes: true},
		{ID: "decline", Label: "Decline", OnClick: func() { ran += "d" }, Closes: true},
		{ID: "close", Label: "Close", Closes: true},
	}}
	k.Push(m)
	ev := traceStep(k, h, at(0, 0), nil)
	var rows []gfx.Rect
	for i, id := range []string{"accept", "decline", "close"} {
		w, ok := k.Find("modal.l." + id)
		if !ok {
			t.Fatalf("row %s is declared", id)
		}
		if i > 0 && w.R.Y-rows[i-1].Y != hudlayout.MenuRowH {
			t.Errorf("row %s is %v under the last, want %d", id, w.R.Y-rows[i-1].Y, hudlayout.MenuRowH)
		}
		if w.R.H != hudlayout.MenuRowH {
			t.Errorf("row %s is %v tall", id, w.R.H)
		}
		rows = append(rows, w.R)
	}
	frame, _, body, footer := ModalGeometry(1280, 720, m)
	if rows[0].W != frame.W-2*hudlayout.WindowPad || rows[0].Y != footer.Y {
		t.Errorf("rows run the window's width from the footer's top: %+v in %+v", rows[0], frame)
	}
	if rows[2].Bottom()+hudlayout.WindowPad != frame.Bottom() || body.Bottom() > footer.Y {
		t.Errorf("the footer fits its rows and the margin: rows end %v, frame %v, body %+v", rows[2].Bottom(), frame.Bottom(), body)
	}
	textInside(t, ev, rows)
	cx, cy := rows[1].Center()
	step(k, h, click(int(cx), int(cy)), nil)
	if ran != "d" || k.Top() != "" {
		t.Fatalf("a row runs its action and closes: ran %q top %q", ran, k.Top())
	}
}

// UIS-008: a drop list opens under its anchor, at least as wide as it and
// ScaleListW, rows ScaleListRowH apart, stays on the screen, greys a
// disabled entry, and a pick closes it.
func TestDropListAnchorsUnderItsRowAndFits(t *testing.T) {
	k, h := newTestKit()
	opts := []DropOpt{{Label: "Auto"}, {Label: "1×"}, {Label: "4×", Disabled: true, Tip: "Too big"}}
	anchor := gfx.Rect{X: 400, Y: 200, W: 40, H: 28}
	got := -1
	DropList(k, "pick", anchor, opts, 0, func(i int) { got = i })
	ev := traceStep(k, h, at(0, 0), nil)
	var rows []gfx.Rect
	for i := range opts {
		w, ok := k.Find(OptID("pick", i))
		if !ok {
			t.Fatalf("row %d is declared", i)
		}
		rows = append(rows, w.R)
	}
	if rows[0].Y < anchor.Bottom() || rows[0].X < anchor.X || rows[0].W < hudlayout.ScaleListW-2 {
		t.Errorf("the list sits under its anchor, at least %d wide: %+v", hudlayout.ScaleListW, rows[0])
	}
	if rows[1].Y-rows[0].Y != hudlayout.ScaleListRowH {
		t.Errorf("row pitch %v, want %d", rows[1].Y-rows[0].Y, hudlayout.ScaleListRowH)
	}
	if dims(ev) {
		t.Error("a drop list does not dim the screen")
	}
	textInside(t, ev, rows)
	cx, cy := rows[2].Center()
	step(k, h, click(int(cx), int(cy)), nil)
	if got != -1 || k.Top() != "pick" {
		t.Fatalf("a disabled entry is not picked: got %d top %q", got, k.Top())
	}
	cx, cy = rows[1].Center()
	step(k, h, click(int(cx), int(cy)), nil)
	if got != 1 || k.Top() != "" {
		t.Fatalf("a pick closes the list: got %d top %q", got, k.Top())
	}

	low := gfx.Rect{X: 1260, Y: 700, W: 20, H: 20}
	r := DropListRect(low, 100, 90, 1280, 720)
	if r.Right() > 1280 || r.Bottom() > 720 || r.Bottom() > low.Y {
		t.Errorf("near the corner the list opens up and left, on the screen: %+v", r)
	}
}
