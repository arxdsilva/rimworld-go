package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/input"
)

// newKit is a menu App on a fake input with its widget hook empty; tests
// set the hook (the screen's widgets) and drive Update frames.
func newKit(t *testing.T) (*App, *fakeInput, *uikit.Kit) {
	t.Helper()
	a := testApp(&stubSim{w: 8, h: 8})
	a.finishBoot()
	f := fakeOn(a)
	f.Cur.Mx, f.Cur.My = 5, 5
	f.Frame(t, a) // settle the screen
	return a, f, a.uiSt()
}

// setHook makes fn the kit's screen hook; u is an App's kit (a.uiSt()).
func setHook(u *uikit.Kit, fn func(a *App)) {
	a := u.Host().(uiHost).a
	u.Hook = func(_, _ int) { fn(a) }
}

// press holds the left button down for one frame at (x, y).
func (f *fakeInput) press(t *testing.T, a *App, x, y int) {
	t.Helper()
	f.Cur.Mx, f.Cur.My = x, y
	f.Cur.Press[inputframe.Left], f.Cur.Down[inputframe.Left] = true, true
	f.Frame(t, a)
}

// move moves the cursor and runs a frame (the button state is unchanged).
func (f *fakeInput) move(t *testing.T, a *App, x, y int) {
	t.Helper()
	f.Cur.Mx, f.Cur.My = x, y
	f.Frame(t, a)
}

// letGo releases the left button at (x, y).
func (f *fakeInput) letGo(t *testing.T, a *App, x, y int) {
	t.Helper()
	f.Cur.Mx, f.Cur.My = x, y
	f.Cur.Down[inputframe.Left], f.Cur.Release[inputframe.Left] = false, true
	f.Frame(t, a)
}

func (f *fakeInput) clickAt(t *testing.T, a *App, x, y int) {
	t.Helper()
	f.Cur.Mx, f.Cur.My = x, y
	f.Click(t, a, inputframe.Left, 0)
}

func closeTo(a, b, eps float64) bool { return a-b < eps && b-a < eps }

func TestSliderDragAndKeys(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	val := 20.0
	released := true
	r := gfx.Rect{X: 100, Y: 100, W: 300, H: 30}
	sp := uikit.SliderSpec{Min: 0, Max: 100, Step: 5, Title: "Rain"}
	setHook(u, func(a *App) {
		a.uiSlider("s", r, &val, sp)
		released = a.in.Released(inputframe.Left)
	})
	track := uikit.SliderTrackRect(r)
	at := func(frac float32) int { return int(track.X + track.W*frac) }

	f.press(t, a, at(0.5), 110)
	if !closeTo(val, 50, 0.01) || u.Active != "s" || u.Focus != "s" {
		t.Fatalf("press on the track captures and jumps: val=%v active=%q focus=%q", val, u.Active, u.Focus)
	}
	f.move(t, a, at(1)+40, 400) // far outside the slider: the capture still follows x
	if val != 100 {
		t.Fatalf("drag past the end clamps: %v", val)
	}
	f.move(t, a, at(0)-40, 5)
	if val != 0 {
		t.Fatalf("drag past the start clamps: %v", val)
	}
	f.move(t, a, at(0.5), 110)
	f.letGo(t, a, at(0.5), 110)
	if released || u.Active != "" || !closeTo(val, 50, 0.01) {
		t.Fatalf("release ends the capture and is consumed: released=%v active=%q val=%v", released, u.Active, val)
	}
	if w, ok := u.Find("s"); !ok || w.Tip != "Rain\n50" {
		t.Fatalf("tooltip shows the value: %+v", w)
	}

	f.Key(t, a, "ArrowRight", 0)
	if !closeTo(val, 55, 0.01) {
		t.Fatalf("Right = +step: %v", val)
	}
	f.Key(t, a, "ArrowRight", input.ModShift)
	if !closeTo(val, 80, 0.01) {
		t.Fatalf("Shift+Right = +5 steps: %v", val)
	}
	f.Key(t, a, "ArrowLeft", 0)
	f.Key(t, a, "End", 0)
	if val != 100 {
		t.Fatalf("End = max: %v", val)
	}
	f.Key(t, a, "Home", 0)
	if val != 0 {
		t.Fatalf("Home = min: %v", val)
	}

	// Discrete stops snap and are labelled.
	stop := 0.0
	dr := gfx.Rect{X: 100, Y: 200, W: 300, H: 40}
	dsp := uikit.SliderSpec{Stops: []string{"Low", "Normal", "High"}}
	setHook(u, func(a *App) { a.uiSlider("d", dr, &stop, dsp) })
	dt := uikit.SliderTrackRect(dr)
	f.press(t, a, int(dt.X+dt.W*0.45), 205)
	if stop != 1 {
		t.Fatalf("0.45 along snaps to the middle stop: %v", stop)
	}
	f.move(t, a, int(dt.X+dt.W*0.9), 205)
	f.letGo(t, a, int(dt.X+dt.W*0.9), 205)
	if stop != 2 {
		t.Fatalf("0.9 along snaps to High: %v", stop)
	}
	f.Key(t, a, "ArrowLeft", 0)
	if stop != 1 {
		t.Fatalf("Left = one stop back: %v", stop)
	}
	if w, _ := u.Find("d"); w.Tip != "Normal" {
		t.Fatalf("stop tooltip = %q", w.Tip)
	}
	n := 3
	setHook(u, func(a *App) { a.uiSliderInt("i", dr, &n, uikit.SliderSpec{Min: 0, Max: 10}) })
	f.press(t, a, int(dt.X), 205)
	f.letGo(t, a, int(dt.X), 205)
	if n != 0 {
		t.Fatalf("int slider = %d", n)
	}
}

func TestTextFieldEditing(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	val := "ab"
	r := gfx.Rect{X: 100, Y: 200, W: 300, H: 32}
	var changed, committed bool
	setHook(u, func(a *App) {
		c, m := a.uiSt().TextField("t", r, &val, uikit.TextFieldSpec{})
		changed, committed = changed || c, committed || m
	})
	f.Frame(t, a)
	if a.textActive() {
		t.Fatal("an unfocused field does not own the keyboard")
	}

	f.clickAt(t, a, 380, 216) // right of the text: caret at the end
	if u.Edit.ID != "t" || u.Caret != 2 || !a.textActive() || a.contextStack()[0] != input.CtxText {
		t.Fatalf("click focuses with the caret at the end: edit=%q caret=%d", u.Edit.ID, u.Caret)
	}
	f.TypeText(t, a, "cd")
	if val != "abcd" || u.Caret != 4 {
		t.Fatalf("typing inserts at the caret: %q caret %d", val, u.Caret)
	}

	f.Key(t, a, "Home", 0)
	f.Key(t, a, "End", input.ModShift)
	if u.Caret != 4 || u.SelStart != 0 || u.SelEnd != 4 {
		t.Fatalf("Home then Shift+End selects everything: caret=%d sel=[%d,%d)", u.Caret, u.SelStart, u.SelEnd)
	}
	f.TypeText(t, a, "Z")
	if val != "Z" {
		t.Fatalf("typing replaces the selection: %q", val)
	}
	f.TypeText(t, a, "xyz")
	f.Key(t, a, "ArrowLeft", 0)
	f.Key(t, a, "ArrowLeft", 0)
	f.TypeText(t, a, "-")
	if val != "Zx-yz" {
		t.Fatalf("Left moves the caret, typing inserts there: %q", val)
	}

	// Backspace repeats: once at once, then after 24 frames, then every 3.
	f.Cur.Held, f.Cur.Pressed = []string{"Backspace"}, []string{"Backspace"}
	f.Frame(t, a)
	if len(val) != 4 {
		t.Fatalf("first Backspace deletes at once: %q", val)
	}
	want := map[int]int{2: 4, 24: 4, 25: 3, 27: 3, 28: 2}
	for n := 2; n <= 28; n++ {
		f.Frame(t, a)
		if l, ok := want[n]; ok && len(val) != l {
			t.Fatalf("frame %d of the hold: %q (len %d), want len %d", n, val, len(val), l)
		}
	}
	f.Cur.Held = nil
	f.Frame(t, a)

	f.Key(t, a, "A", input.ModCtrl)
	if u.SelStart != 0 || u.SelEnd != len([]rune(val)) {
		t.Fatalf("Ctrl+A selects all: [%d,%d)", u.SelStart, u.SelEnd)
	}
	f.TypeText(t, a, "Q")
	if val != "Q" {
		t.Fatalf("typing over Ctrl+A: %q", val)
	}

	changed = false
	f.Key(t, a, "Escape", 0)
	f.Frame(t, a)
	if val != "ab" || u.Edit.ID != "" || a.textActive() || !changed {
		t.Fatalf("Esc restores the value from focus time and ends the edit: %q edit=%q changed=%v", val, u.Edit.ID, changed)
	}

	// Click-away commits and keeps the edit.
	f.clickAt(t, a, 380, 216)
	f.TypeText(t, a, "!")
	committed = false
	f.clickAt(t, a, 700, 500)
	f.Frame(t, a)
	if val != "ab!" || u.Edit.ID != "" || !committed {
		t.Fatalf("click elsewhere commits: %q edit=%q committed=%v", val, u.Edit.ID, committed)
	}

	// Enter commits a single-line field and a caret click lands mid-text.
	val = "hello"
	f.clickAt(t, a, 100+uikit.FieldPad+1, 216)
	if u.Caret != 0 {
		t.Fatalf("a click at the left edge puts the caret first: %d", u.Caret)
	}
	f.TypeText(t, a, "X")
	committed = false
	f.Key(t, a, "Enter", 0)
	f.Frame(t, a)
	if val != "Xhello" || u.Edit.ID != "" || !committed {
		t.Fatalf("Enter commits: %q edit=%q committed=%v", val, u.Edit.ID, committed)
	}
	if a.textActive() {
		t.Fatal("global shortcuts are back after the edit")
	}

	// Delete, Normalize and Esc after a normalising commit.
	val = "x"
	spec := uikit.TextFieldSpec{Normalize: strings.ToUpper}
	setHook(u, func(a *App) { a.uiSt().TextField("t", r, &val, spec) })
	f.clickAt(t, a, 380, 216)
	f.TypeText(t, a, "yz")
	f.Key(t, a, "Home", 0)
	f.Key(t, a, "Delete", 0)
	if val != "yz" {
		t.Fatalf("Delete removes after the caret: %q", val)
	}
	f.Key(t, a, "Enter", 0)
	if val != "YZ" {
		t.Fatalf("Normalize runs on commit: %q", val)
	}
}

func TestTextFieldValidatorAndMaxLen(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	val := ""
	r := gfx.Rect{X: 100, Y: 200, W: 200, H: 32}
	setHook(u, func(a *App) {
		a.uiSt().TextField("n", r, &val, uikit.TextFieldSpec{MaxLen: 3, Valid: uikit.DigitsOnly, Placeholder: "seed"})
	})
	f.clickAt(t, a, 150, 216)
	f.TypeText(t, a, "1a2b345")
	if val != "123" {
		t.Fatalf("digits only, at most 3: %q", val)
	}
	f.Key(t, a, "Backspace", 0)
	f.TypeText(t, a, "9")
	if val != "129" {
		t.Fatalf("after Backspace there is room again: %q", val)
	}
}

func TestTextFieldMultiline(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	val := "first line\nsecond"
	r := gfx.Rect{X: 100, Y: 200, W: 240, H: 120}
	setHook(u, func(a *App) { a.uiSt().TextField("m", r, &val, uikit.TextFieldSpec{Multiline: true}) })
	f.clickAt(t, a, 330, 300) // below the text: caret at the end
	if u.Caret != len([]rune(val)) {
		t.Fatalf("caret = %d", u.Caret)
	}
	f.Key(t, a, "ArrowUp", 0)
	if u.Caret > len("first line") {
		t.Fatalf("Up moves to the first line: %d", u.Caret)
	}
	f.Key(t, a, "End", 0)
	if u.Caret != len("first line") {
		t.Fatalf("End goes to the end of the visual line: %d", u.Caret)
	}
	f.Key(t, a, "Enter", 0)
	if val != "first line\n\nsecond" || u.Edit.ID != "m" {
		t.Fatalf("Enter inserts a newline in a multi-line field: %q", val)
	}
	f.TypeText(t, a, "this extra text wraps around the narrow field")
	spans := uikit.LayoutLines([]rune(val), gfx.Body, float64(uikit.FieldInner(r).W))
	if len(spans) < 4 {
		t.Fatalf("long text wraps into visual lines: %d", len(spans))
	}
	for _, sp := range spans {
		if w, _ := gfx.Measure(val[sp.Start:sp.End], gfx.Body); w > float64(uikit.FieldInner(r).W)+0.5 {
			t.Fatalf("line %q is %v wide", val[sp.Start:sp.End], w)
		}
	}
	f.Key(t, a, "Escape", 0)
	if val != "first line\nsecond" {
		t.Fatalf("Esc restores: %q", val)
	}
}

func TestScrollViewWheelAndThumb(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	r := gfx.Rect{X: 100, Y: 100, W: 300, H: 200}
	var area uikit.ScrollArea
	wheelLeft := 0.0
	setHook(u, func(a *App) {
		area = a.uiSt().ScrollArea("sv", r, 1000, uikit.ScrollOpts{})
		wheelLeft = a.in.Raw.WheelY
	})
	f.Cur.Mx, f.Cur.My = 200, 200
	f.Cur.WheelY = -1
	f.Frame(t, a)
	if area.Off != uikit.ScrollStep || wheelLeft != 0 {
		t.Fatalf("wheel down scrolls and is consumed: off=%v left=%v", area.Off, wheelLeft)
	}
	f.Cur.WheelY = 1
	f.Frame(t, a)
	f.Cur.WheelY = 1
	f.Frame(t, a)
	if area.Off != 0 {
		t.Fatalf("wheel up clamps at the top: %v", area.Off)
	}
	f.Cur.Mx, f.Cur.My = 50, 50
	f.Cur.WheelY = -1
	f.Frame(t, a)
	if area.Off != 0 || wheelLeft != -1 {
		t.Fatalf("the wheel off the view is not taken: off=%v left=%v", area.Off, wheelLeft)
	}

	// The thumb: 200*200/1000 = 40 px tall at the top of the 8 px bar.
	bar := gfx.Rect{X: r.Right() - uikit.ScrollBarW, Y: r.Y, W: uikit.ScrollBarW, H: r.H}
	thumb := uikit.ScrollThumbRect(bar, 0, 800, 1000)
	if thumb.H != 40 || thumb.Y != 100 {
		t.Fatalf("thumb = %+v", thumb)
	}
	f.press(t, a, 396, 110)
	if u.Active != "sv.thumb" {
		t.Fatalf("press on the thumb captures: %q", u.Active)
	}
	f.move(t, a, 396, 270) // grabbed 10 px down: the thumb's top is at 260 = the bottom stop
	if area.Off != 800 {
		t.Fatalf("dragging to the bottom reaches the end: %v", area.Off)
	}
	f.move(t, a, 396, 5)
	if area.Off != 0 {
		t.Fatalf("dragging past the top: %v", area.Off)
	}
	f.letGo(t, a, 396, 5)
	if u.Active != "" {
		t.Fatalf("release frees the capture: %q", u.Active)
	}

	// A click on the track below the thumb pages down; PageDown over the view too.
	f.clickAt(t, a, 396, 290)
	if !closeTo(float64(area.Off), 180, 0.5) {
		t.Fatalf("track click pages: %v", area.Off)
	}
	f.Cur.Mx, f.Cur.My = 200, 200
	f.Key(t, a, "PageDown", 0)
	if !closeTo(float64(area.Off), 360, 0.5) {
		t.Fatalf("PageDown pages: %v", area.Off)
	}
	f.Key(t, a, "PageUp", 0)
	if !closeTo(float64(area.Off), 180, 0.5) {
		t.Fatalf("PageUp pages back: %v", area.Off)
	}
	if !a.uiSt().WheelOver(200, 200) || a.uiSt().WheelOver(50, 50) {
		t.Fatal("uiWheelOver follows the scroll view")
	}
}

func TestListSelectionAndKeys(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	r := gfx.Rect{X: 100, Y: 100, W: 300, H: 260}
	sel := -1
	setHook(u, func(a *App) {
		a.uiSt().List("l", r, 100, &sel, uikit.ListSpec{Label: func(i int) string { return "row " + uikit.Num(i) }})
	})
	f.Frame(t, a)
	f.clickAt(t, a, 150, 100+2*uikit.ListRowH+5)
	if sel != 2 || u.Focus != "l" {
		t.Fatalf("a click selects the row: sel=%d focus=%q", sel, u.Focus)
	}
	f.Key(t, a, "ArrowDown", 0)
	if sel != 3 {
		t.Fatalf("Down = next row: %d", sel)
	}
	f.Key(t, a, "End", 0)
	if sel != 99 {
		t.Fatalf("End = last row: %d", sel)
	}
	if off := u.Scroll["l"]; off != 100*uikit.ListRowH-r.H {
		t.Fatalf("the selection is kept visible: off=%v", off)
	}
	f.Key(t, a, "Home", 0)
	if sel != 0 || u.Scroll["l"] != 0 {
		t.Fatalf("Home = first row, scrolled to the top: sel=%d off=%v", sel, u.Scroll["l"])
	}
	first, last := uikit.ListVisible(1000, 260, 26, 100)
	if first != 38 || last != 48 {
		t.Fatalf("only visible rows are laid out: %d..%d", first, last)
	}
	// virtualised: far fewer widgets than rows
	if len(u.Cur) > 14 {
		t.Fatalf("%d widgets for 100 rows", len(u.Cur))
	}
}

func TestTooltipDelayAndClamp(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	setHook(u, func(a *App) {
		a.uiSt().Tip("tip", gfx.Rect{X: 200, Y: 200, W: 80, H: 30}, "Landing site\nTemperate forest, 14 C a year.")
	})
	f.Cur.Mx, f.Cur.My = 210, 210
	for i := 1; i <= 23; i++ {
		f.Frame(t, a)
	}
	if u.TipVisible() {
		t.Fatalf("no tooltip before the delay (%d frames)", u.TipFrames)
	}
	for i := 0; i < 5; i++ {
		f.Frame(t, a)
	}
	if !u.TipVisible() {
		t.Fatalf("the tooltip shows after the delay (%d frames)", u.TipFrames)
	}
	f.move(t, a, 600, 400)
	f.Frame(t, a)
	if u.TipVisible() {
		t.Fatal("moving off hides it")
	}
	f.move(t, a, 210, 210)
	if u.TipVisible() {
		t.Fatal("coming back restarts the delay")
	}
	f.Cur.Press[inputframe.Left], f.Cur.Down[inputframe.Left] = true, true
	for i := 0; i < 30; i++ {
		f.Frame(t, a)
		f.Cur.Press[inputframe.Left] = false
	}
	if u.TipVisible() {
		t.Fatal("no tooltip while a button is down")
	}
	f.Cur.Down[inputframe.Left] = false

	long := "Title that is rather long and will need wrapping onto several lines to fit\n" + strings.Repeat("A body sentence that keeps going. ", 12)
	c := uikit.TipLayout(long, 1280, 720, 1270, 710)
	if c.Box.W > uikit.TipMaxW || c.Box.X < uikit.TipMargin || c.Box.Y < uikit.TipMargin || c.Box.Right() > 1280-uikit.TipMargin || c.Box.Bottom() > 720-uikit.TipMargin {
		t.Fatalf("card must stay on screen and within 360 px: %+v", c.Box)
	}
	if len(c.Title) < 2 || len(c.Body) < 3 {
		t.Fatalf("title and body wrap: %d/%d lines", len(c.Title), len(c.Body))
	}
	if c.Box.Y+c.Box.H > 710 {
		t.Fatalf("near the bottom the card flips above the cursor: %+v", c.Box)
	}
	short := uikit.TipLayout("Draft [R]", 1280, 720, 100, 100)
	if short.Box.X != 114 || short.Box.Y != 120 || len(short.Body) != 0 {
		t.Fatalf("a short tip sits down-right of the cursor: %+v", short)
	}
	// paint draws it without panicking
	f.move(t, a, 210, 210)
	for i := 0; i < 30; i++ {
		f.Frame(t, a)
	}
	a.uiSt().Paint(ebiten.NewImage(1280, 720))
}

func TestModalEscClosesTopOnly(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	a.apply(intent{kind: intentPush, screen: ports.ScreenOptions})
	f.Frame(t, a)
	closed := []string{}
	for _, id := range []string{"one", "two"} {
		id := id
		a.uiPushModal(modal{ID: id, Title: id, W: 400, H: 200, OnClose: func() { closed = append(closed, id) }})
	}
	f.Frame(t, a)
	if a.uiModalTop() != "two" || a.contextStack()[0] != input.CtxModal {
		t.Fatalf("the modal is on top of the input stack: %v", a.contextStack())
	}
	for _, h := range a.layoutHits(a.size()) {
		if h.ID == "options.back" {
			t.Fatal("a modal hides the screen's buttons from the hit list")
		}
	}

	f.Key(t, a, "Escape", 0)
	if len(u.Modals) != 1 || a.uiModalTop() != "one" || strings.Join(closed, ",") != "two" {
		t.Fatalf("Esc closes only the top modal: %v closed=%v", u.Modals, closed)
	}
	if a.nav.screen() != ports.ScreenOptions {
		t.Fatalf("and does not go Back: %s", a.nav.screen())
	}

	// A click on a button behind the modal does nothing.
	f.clickAt(t, a, 50, int(footerY(a.sh))+10)
	if a.nav.screen() != ports.ScreenOptions || len(u.Modals) != 1 {
		t.Fatalf("clicks outside are ignored: %s, %d modals", a.nav.screen(), len(u.Modals))
	}

	// The X closes the top one too.
	_, tr, _, _ := uikit.ModalGeometry(a.sw, a.sh, u.Modals[0])
	f.clickAt(t, a, int(tr.Right())-uikit.ModalCloseW/2-8, int(tr.Y+tr.H/2))
	if len(u.Modals) != 0 || strings.Join(closed, ",") != "two,one" {
		t.Fatalf("the X closes: %v closed=%v", u.Modals, closed)
	}
	if a.nav.screen() != ports.ScreenOptions {
		t.Fatal("still on Options")
	}
	f.Key(t, a, "Escape", 0)
	if a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("with no modal Esc goes Back: %s", a.nav.screen())
	}
}

func TestModalConfirmAndButtons(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	ok := 0
	a.uiSt().Confirm("c", "Discard world?", "The generated world will be lost.", "Discard", true, func() { ok++ })
	f.Frame(t, a)
	if len(u.Modals) != 1 {
		t.Fatal("confirm opens")
	}
	w, found := u.Find("modal.c.ok")
	if !found {
		t.Fatalf("OK button is registered: %+v", u.Cur)
	}
	cx, cy := w.R.Center()
	f.clickAt(t, a, int(cx), int(cy))
	if ok != 1 || len(u.Modals) != 0 {
		t.Fatalf("OK runs the action and closes: ok=%d modals=%d", ok, len(u.Modals))
	}
	a.uiSt().Confirm("c", "Discard world?", "The generated world will be lost.", "Discard", false, func() { ok++ })
	f.Frame(t, a)
	w, _ = u.Find("modal.c.cancel")
	cx, cy = w.R.Center()
	f.clickAt(t, a, int(cx), int(cy))
	if ok != 1 || len(u.Modals) != 0 {
		t.Fatalf("Cancel closes without acting: ok=%d", ok)
	}
	a.uiPushModal(modal{ID: "x", Title: "X", W: 300, H: 200})
	a.uiPushModal(modal{ID: "x", Title: "X", W: 300, H: 200})
	if len(u.Modals) != 1 {
		t.Fatal("a modal id opens once")
	}
	a.win.CloseModal("x")
	if len(u.Modals) != 0 {
		t.Fatal("closed by id")
	}
}

func TestTableSort(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	names := []string{"cara", "adam", "bea"}
	scores := []int{5, 9, 1}
	sel := -1
	r := gfx.Rect{X: 100, Y: 100, W: 400, H: 200}
	cols := []uikit.TableCol{
		{ID: "name", Title: "Name", Cell: func(i int) string { return names[i] }, Less: func(x, y int) bool { return names[x] < names[y] }},
		{ID: "score", Title: "Score", Cell: func(i int) string { return uikit.Num(scores[i]) }, Less: func(x, y int) bool { return scores[x] < scores[y] },
			Tip: func(i int) string { return "score of " + names[i] }},
		{ID: "note", Title: "Note", Cell: func(i int) string { return "-" }},
	}
	setHook(u, func(a *App) { a.uiSt().Table("tb", r, cols, 3, &sel, 0) })
	f.Frame(t, a)
	order := func() []int { return a.uiSt().TableOrder("tb", cols, 3) }
	eq := func(got []int, want ...int) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	if !eq(order(), 0, 1, 2) {
		t.Fatalf("unsorted = %v", order())
	}
	colW := (r.W - uikit.ScrollBarW) / 3
	nameX, scoreX, noteX := int(r.X+colW/2), int(r.X+colW*1.5), int(r.X+colW*2.5)
	f.clickAt(t, a, nameX, 110)
	if !eq(order(), 1, 2, 0) {
		t.Fatalf("name ascending = %v", order())
	}
	f.clickAt(t, a, nameX, 110)
	if col, desc := a.uiSt().TableSort("tb"); col != 0 || !desc || !eq(order(), 0, 2, 1) {
		t.Fatalf("again reverses: col=%d desc=%v %v", col, desc, order())
	}
	f.clickAt(t, a, scoreX, 110)
	if col, desc := a.uiSt().TableSort("tb"); col != 1 || desc || !eq(order(), 2, 0, 1) {
		t.Fatalf("another column sorts ascending: col=%d desc=%v %v", col, desc, order())
	}
	f.clickAt(t, a, noteX, 110)
	if col, _ := a.uiSt().TableSort("tb"); col != 1 {
		t.Fatalf("an unsortable header does nothing: %d", col)
	}

	// Rows: display row 0 is data row 2 ("bea", the lowest score).
	f.clickAt(t, a, nameX, 100+uikit.TableHeaderH+uikit.ListRowH/2)
	if sel != 2 {
		t.Fatalf("selection is a data row: %d", sel)
	}
	f.clickAt(t, a, nameX, 100+uikit.TableHeaderH+uikit.ListRowH+5)
	if sel != 0 {
		t.Fatalf("second display row is data row 0: %d", sel)
	}
	// cell tooltip
	f.move(t, a, scoreX, 100+uikit.TableHeaderH+5)
	if u.Hot == "" || !strings.Contains(u.Hot, ".cell.0.score") {
		t.Fatalf("hover is the score cell: %q", u.Hot)
	}
	if w, _ := u.Find(u.Hot); w.Tip != "score of bea" {
		t.Fatalf("cell tip = %q", w.Tip)
	}
	a.uiSt().TableSetSort("tb", -1, false)
	if col, _ := a.uiSt().TableSort("tb"); col != -1 {
		t.Fatal("sort cleared")
	}
	a.uiSt().Paint(ebiten.NewImage(1280, 720))
}

func TestTabFocusOrder(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	on, text, tab := false, "name", 0
	v := 3.0
	setHook(u, func(a *App) {
		a.uiSt().Check("a", gfx.Rect{X: 100, Y: 100, W: 200, H: 24}, "Check", &on, uikit.CheckOpts{})
		a.uiSlider("b", gfx.Rect{X: 100, Y: 140, W: 200, H: 30}, &v, uikit.SliderSpec{Min: 0, Max: 10})
		a.uiSt().TextField("c", gfx.Rect{X: 100, Y: 190, W: 200, H: 30}, &text, uikit.TextFieldSpec{})
		a.uiSt().Tabs("d", gfx.Rect{X: 100, Y: 240, W: 300, H: 28}, []string{"One", "Two", "Three"}, &tab, uikit.TabsOpts{})
		a.uiSt().Button("e", gfx.Rect{X: 100, Y: 290, W: 100, H: 30}, "Go", uikit.BtnOpts{Disabled: true, Reason: "not yet"})
	})
	f.Frame(t, a)
	step := func(mods input.Mod) string {
		f.Key(t, a, "Tab", mods)
		return u.Focus
	}
	for _, want := range []string{"a", "b", "c", "d", "a"} {
		if got := step(0); got != want {
			t.Fatalf("Tab -> %q, want %q", got, want)
		}
		if want == "c" && u.Edit.ID != "c" {
			t.Fatal("Tab onto a text field starts editing it")
		}
	}
	if got := step(input.ModShift); got != "d" {
		t.Fatalf("Shift+Tab wraps back to %q", got)
	}
	if u.Edit.ID != "" {
		t.Fatal("moving off the field ended its edit")
	}
	if got := step(input.ModShift); got != "c" || u.Edit.ID != "c" {
		t.Fatalf("Shift+Tab -> %q (edit %q)", got, u.Edit.ID)
	}
	if got := step(input.ModShift); got != "b" {
		t.Fatalf("Shift+Tab out of a field -> %q", got)
	}
	// The disabled button is skipped; Tab never reaches the Architect binding.
	f.Key(t, a, "Tab", 0) // c
	f.Key(t, a, "Tab", 0) // d
	f.Key(t, a, "Tab", 0)
	if u.Focus != "a" {
		t.Fatalf("e is disabled, so after d comes a: %q", u.Focus)
	}
	f.Key(t, a, "Space", 0)
	if !on {
		t.Fatal("Space toggles the focused checkbox")
	}
	f.Key(t, a, "Space", 0)
	if on {
		t.Fatal("and back")
	}
	// Arrow keys inside the tab strip.
	u.Focus = "d"
	f.Key(t, a, "ArrowRight", 0)
	f.Key(t, a, "ArrowRight", 0)
	f.Key(t, a, "ArrowRight", 0)
	if tab != 2 {
		t.Fatalf("Right moves to the next tab and stops at the end: %d", tab)
	}
	f.Key(t, a, "ArrowLeft", 0)
	if tab != 1 {
		t.Fatalf("Left = previous tab: %d", tab)
	}
	a.uiSt().Paint(ebiten.NewImage(1280, 720)) // the focus ring paints
}

func TestCheckRadioAndButtons(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	on, pick, tab := false, 0, 0
	clicks := 0
	setHook(u, func(a *App) {
		a.uiSt().Check("chk", gfx.Rect{X: 100, Y: 100, W: 200, H: 24}, "Ruins", &on, uikit.CheckOpts{})
		a.uiSt().Radio("rad", gfx.Rect{X: 100, Y: 140, W: 200, H: 72}, []string{"One", "Two", "Locked"}, &pick, uikit.RadioOpts{Disabled: []string{"", "", "needs a map"}})
		a.uiSt().Tabs("tabs", gfx.Rect{X: 100, Y: 240, W: 400, H: 28}, []string{"A", "B", "C"}, &tab, uikit.TabsOpts{Disabled: []string{"", "", "later"}})
		if a.uiSt().Button("btn", gfx.Rect{X: 100, Y: 300, W: 100, H: 30}, "Go", uikit.BtnOpts{}) {
			clicks++
		}
		if a.uiSt().Button("dis", gfx.Rect{X: 220, Y: 300, W: 100, H: 30}, "No", uikit.BtnOpts{Disabled: true, Reason: "pick a tile first"}) {
			clicks += 100
		}
	})
	f.Frame(t, a)
	f.clickAt(t, a, 120, 110)
	if !on {
		t.Fatal("click toggles the checkbox")
	}
	f.clickAt(t, a, 120, 110)
	if on {
		t.Fatal("click again clears it")
	}
	f.clickAt(t, a, 120, 140+24+5)
	if pick != 1 {
		t.Fatalf("radio pick = %d", pick)
	}
	f.clickAt(t, a, 120, 140+48+5)
	if pick != 1 || a.hint != "needs a map" {
		t.Fatalf("a disabled radio option says why: pick=%d hint=%q", pick, a.hint)
	}
	f.Key(t, a, "ArrowUp", 0)
	if pick != 0 {
		t.Fatalf("Up = previous option: %d", pick)
	}
	f.Key(t, a, "ArrowDown", 0)
	f.Key(t, a, "ArrowDown", 0)
	if pick != 1 {
		t.Fatalf("Down skips the disabled option: %d", pick)
	}
	rects := uikit.TabRects(gfx.Rect{X: 100, Y: 240, W: 400, H: 28}, []string{"A", "B", "C"})
	f.clickAt(t, a, int(rects[1].X)+5, 250)
	f.clickAt(t, a, int(rects[2].X)+5, 250)
	if tab != 1 || a.hint != "later" {
		t.Fatalf("tabs select; a disabled one says why: tab=%d hint=%q", tab, a.hint)
	}
	f.clickAt(t, a, 110, 310)
	f.clickAt(t, a, 230, 310)
	if clicks != 1 || a.hint != "pick a tile first" {
		t.Fatalf("buttons: enabled fires once, disabled denies: clicks=%d hint=%q", clicks, a.hint)
	}
	// The click path reaches widgets by id (uiClick), and layoutHits lists them.
	found := false
	for _, b := range a.layoutHits(a.size()) {
		if b.ID == "btn" {
			found = true
		}
	}
	ok := a.uiSt().Click("btn")
	f.Frame(t, a) // the widget reports the click on its next call
	if !found || !ok || clicks != 2 || a.uiSt().Click("nope") {
		t.Fatalf("widgets are in the hit list and clickable by id: found=%v clicks=%d", found, clicks)
	}
	a.uiSt().Paint(ebiten.NewImage(1280, 720))
}

func TestWheelRoutingMenuVsCamera(t *testing.T) {
	t.Parallel()
	// In a menu the scroll view takes the wheel from the frame.
	a, f, u := newKit(t)
	r := gfx.Rect{X: 100, Y: 100, W: 300, H: 200}
	left := 0.0
	setHook(u, func(a *App) {
		a.uiSt().ScrollArea("menu", r, 900, uikit.ScrollOpts{})
		left = a.in.Raw.WheelY
	})
	f.Cur.Mx, f.Cur.My = 200, 200
	f.Cur.WheelY = -2
	f.Frame(t, a)
	if left != 0 || u.Scroll["menu"] != 2*uikit.ScrollStep {
		t.Fatalf("a hovered menu consumes the wheel: left=%v off=%v", left, u.Scroll["menu"])
	}

	// Over the colony, a modal's list scrolls and the camera stays put; off
	// the list the wheel zooms the map as before.
	ca, _, cs := inputColony(t)
	ca.renderer.smooth.Instant = true
	lr := gfx.Rect{X: 300, Y: 200, W: 300, H: 200}
	ca.uiPushModal(modal{ID: "list", Title: "List", W: 400, H: 300, Draw: func(a *App, body gfx.Rect) {
		a.uiSt().ScrollArea("modal.list", lr, 900, uikit.ScrollOpts{})
	}})
	cs.Frame(t, ca)
	z := ca.renderer.cam.Zoom
	cs.Cur.Mx, cs.Cur.My = 400, 300
	cs.Cur.WheelY = 1
	cs.Frame(t, ca)
	if ca.renderer.cam.Zoom != z {
		t.Fatal("the wheel over a scroll view does not zoom the camera")
	}
	if ca.uiSt().Scroll["modal.list"] != 0 {
		t.Fatal("scrolling up at the top stays at the top")
	}
	cs.Cur.WheelY = -1
	cs.Frame(t, ca)
	if ca.uiSt().Scroll["modal.list"] != uikit.ScrollStep || ca.renderer.cam.Zoom != z {
		t.Fatalf("it scrolls the list instead: off=%v zoom %v -> %v", ca.uiSt().Scroll["modal.list"], z, ca.renderer.cam.Zoom)
	}
	cs.Cur.Mx, cs.Cur.My = 5, 5
	cs.Cur.WheelY = 1
	cs.Frame(t, ca)
	if ca.renderer.cam.Zoom <= z {
		t.Fatal("the wheel away from every scroll view zooms the map")
	}
	// PageUp over a hovered scroll view does not zoom either.
	z = ca.renderer.cam.Zoom
	cs.Cur.Mx, cs.Cur.My = 400, 300
	cs.Key(t, ca, "PageUp", 0)
	if ca.renderer.cam.Zoom != z {
		t.Fatal("PageUp belongs to the scroll view under the cursor")
	}
}

func TestSetupMinWindow(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		w, h  int
		small bool
	}{{1279, 720, true}, {1280, 719, true}, {800, 600, true}, {1280, 720, false}, {1920, 1080, false}} {
		if uikit.TooSmall(c.w, c.h) != c.small {
			t.Fatalf("tooSmall(%d,%d) != %v", c.w, c.h, c.small)
		}
	}
	a, f, u := newKit(t)
	setHook(u, func(a *App) {
		sw, sh := a.size()
		if a.uiSt().SetupTooSmall(sw, sh) {
			return
		}
		a.uiSt().Button("btn", gfx.Rect{X: 100, Y: 100, W: 100, H: 30}, "Go", uikit.BtnOpts{})
	})
	hasBtn := func() bool {
		for _, b := range a.layoutHits(a.size()) {
			if b.ID == "btn" {
				return true
			}
		}
		return false
	}
	a.sw, a.sh = 1100, 700
	f.Frame(t, a)
	if hasBtn() {
		t.Fatal("a window below 1280x720 shows no widgets (they would overlap)")
	}
	img := ebiten.NewImage(1100, 700)
	uikit.DrawTooSmall(img, 1100, 700)
	a.uiSt().Paint(img)
	a.sw, a.sh = 1280, 720
	f.Frame(t, a)
	if !hasBtn() {
		t.Fatal("at 1280x720 the widgets are back")
	}
}

func TestKeyRepeatSchedule(t *testing.T) {
	t.Parallel()
	var fired []int
	for n := 1; n <= 34; n++ {
		if inputframe.RepeatFire(n) {
			fired = append(fired, n)
		}
	}
	want := []int{1, 25, 28, 31, 34}
	if len(fired) != len(want) {
		t.Fatalf("fires on %v, want %v", fired, want)
	}
	for i := range want {
		if fired[i] != want[i] {
			t.Fatalf("fires on %v, want %v", fired, want)
		}
	}
	a, f, _ := newKit(t)
	f.Cur.Held = []string{"ArrowDown"}
	f.Frame(t, a)
	if !a.keyRepeat("ArrowDown") || a.keyRepeat("ArrowUp") {
		t.Fatal("keyRepeat follows the held keys")
	}
	f.Cur.Held = nil
	f.Frame(t, a)
	if a.keyRepeat("ArrowDown") {
		t.Fatal("released keys do not repeat")
	}
}

// testScreen is a screen id nothing else uses, for the registry path.
const testScreen ports.Screen = 99

var testScreenDrawn int

// registerTestScreen adds the test screen to a's table, as NewApp does
// for the real ones.
func registerTestScreen(a *App) {
	a.shell.screens[testScreen] = func(a *App, sw, sh int) {
		testScreenDrawn++
		a.uiSt().Panel(gfx.Rect{X: 10, Y: 10, W: 100, H: 100})
		a.uiSt().Label(20, 20, "registered", gfx.Body)
	}
}

func TestRegisteredScreenUIAndPaintSmoke(t *testing.T) {
	a, f, u := newKit(t)
	registerTestScreen(a)
	before := testScreenDrawn
	a.nav.push(frame{screen: testScreen})
	f.Frame(t, a)
	f.Frame(t, a)
	if testScreenDrawn < before+2 {
		t.Fatal("the registered screen hook runs once per Update")
	}
	if a.uiSt().IsHot("x") || a.uiSt().IsFocused("x") {
		t.Fatal("nothing hot or focused")
	}
	img := ebiten.NewImage(1280, 720)
	a.uiSt().Paint(img)
	a.nav.pop()

	// Every widget painted: stops slider, a selected multi-line field with
	// Reduced Motion and without, the default list row, a table with an
	// icon-less custom cell.
	v, sv := 1.0, 0.3
	text, multi := "some text here", "alpha beta\ngamma delta epsilon zeta eta theta"
	sel := 1
	on := true
	pick := 1
	setHook(u, func(a *App) {
		a.uiSlider("st", gfx.Rect{X: 50, Y: 50, W: 300, H: 40}, &v, uikit.SliderSpec{Stops: []string{"Low", "Normal", "High"}})
		a.uiSlider("ct", gfx.Rect{X: 50, Y: 100, W: 300, H: 30}, &sv, uikit.SliderSpec{Min: 0, Max: 1, Disabled: true, Reason: "locked", Format: func(f float64) string { return "x" }})
		a.uiSt().TextField("tf", gfx.Rect{X: 50, Y: 150, W: 200, H: 30}, &text, uikit.TextFieldSpec{})
		a.uiSt().TextField("mf", gfx.Rect{X: 50, Y: 200, W: 160, H: 80}, &multi, uikit.TextFieldSpec{Multiline: true})
		a.uiSt().TextField("ph", gfx.Rect{X: 300, Y: 150, W: 200, H: 30}, new(string), uikit.TextFieldSpec{Placeholder: "seed"})
		a.uiSt().TextField("dis", gfx.Rect{X: 300, Y: 200, W: 200, H: 30}, &text, uikit.TextFieldSpec{Disabled: true, Reason: "locked"})
		a.uiSt().List("ls", gfx.Rect{X: 50, Y: 300, W: 200, H: 100}, 8, &sel, uikit.ListSpec{Label: func(i int) string { return "item " + uikit.Num(i) }, Disabled: func(i int) string {
			if i == 5 {
				return "gone"
			}
			return ""
		}})
		a.uiSt().Check("ck", gfx.Rect{X: 300, Y: 300, W: 150, H: 24}, "Check", &on, uikit.CheckOpts{Disabled: true, Reason: "r"})
		a.uiSt().Radio("rd", gfx.Rect{X: 300, Y: 340, W: 150, H: 48}, []string{"a", "b"}, &pick, uikit.RadioOpts{})
		a.uiSt().Button("b1", gfx.Rect{X: 300, Y: 400, W: 90, H: 30}, "Primary", uikit.BtnOpts{Style: uikit.BtnPrimary})
		a.uiSt().Button("b2", gfx.Rect{X: 400, Y: 400, W: 90, H: 30}, "Danger", uikit.BtnOpts{Style: uikit.BtnDanger, Tip: "Careful\nIt is final."})
	})
	f.clickAt(t, a, 100, 165) // edit the single-line field
	f.Key(t, a, "ArrowLeft", input.ModShift)
	f.Key(t, a, "ArrowLeft", input.ModShift)
	a.uiSt().Paint(img)
	f.Cur.Held = []string{"Meta"}
	f.Key(t, a, "A", 0)
	f.Cur.Held = nil
	if u.SelStart != 0 || u.SelEnd != len(text) {
		t.Fatalf("Cmd+A selects all: [%d,%d)", u.SelStart, u.SelEnd)
	}
	f.Key(t, a, "ArrowLeft", 0)
	if u.Caret != 0 || u.SelStart != u.SelEnd {
		t.Fatalf("Left collapses a selection to its start: caret=%d", u.Caret)
	}
	f.Key(t, a, "ArrowRight", input.ModShift)
	f.Key(t, a, "ArrowRight", 0)
	if u.Caret != 1 {
		t.Fatalf("Right collapses to its end: %d", u.Caret)
	}
	a.settings.Accessibility.ReducedMotion = true
	a.uiSt().Paint(img)
	a.settings.Accessibility.ReducedMotion = false
	for i := 0; i < 70; i++ { // the caret blinks through both phases
		a.updates++
		a.uiSt().Paint(img)
	}
	f.clickAt(t, a, 100, 240) // the multi-line field takes over editing
	f.Key(t, a, "ArrowDown", input.ModShift)
	f.Key(t, a, "ArrowUp", 0)
	a.uiSt().Paint(img)
	f.clickAt(t, a, 700, 600)
	f.move(t, a, 70, 320)
	a.uiSt().Paint(img)
}
