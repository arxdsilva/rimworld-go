package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/input"
)

// newKit is a menu App on a fake input with its widget hook empty; tests
// set the hook (the screen's widgets) and drive Update frames.
func newKit(t *testing.T) (*App, *fakeInput, *uiState) {
	t.Helper()
	a := testApp(&stubSim{w: 8, h: 8})
	a.finishBoot()
	f := fakeOn(a)
	f.cur.mx, f.cur.my = 5, 5
	f.frame(t, a) // settle the screen
	return a, f, a.uiSt()
}

func setHook(u *uiState, fn func(a *App)) { u.hook = func(a *App, _, _ int) { fn(a) } }

// press holds the left button down for one frame at (x, y).
func (f *fakeInput) press(t *testing.T, a *App, x, y int) {
	t.Helper()
	f.cur.mx, f.cur.my = x, y
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	f.frame(t, a)
}

// move moves the cursor and runs a frame (the button state is unchanged).
func (f *fakeInput) move(t *testing.T, a *App, x, y int) {
	t.Helper()
	f.cur.mx, f.cur.my = x, y
	f.frame(t, a)
}

// letGo releases the left button at (x, y).
func (f *fakeInput) letGo(t *testing.T, a *App, x, y int) {
	t.Helper()
	f.cur.mx, f.cur.my = x, y
	f.cur.down[mbLeft], f.cur.release[mbLeft] = false, true
	f.frame(t, a)
}

func (f *fakeInput) clickAt(t *testing.T, a *App, x, y int) {
	t.Helper()
	f.cur.mx, f.cur.my = x, y
	f.click(t, a, mbLeft, 0)
}

func closeTo(a, b, eps float64) bool { return a-b < eps && b-a < eps }

func TestSliderDragAndKeys(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	val := 20.0
	released := true
	r := rect{X: 100, Y: 100, W: 300, H: 30}
	sp := sliderSpec{Min: 0, Max: 100, Step: 5, Title: "Rain"}
	setHook(u, func(a *App) {
		a.uiSlider("s", r, &val, sp)
		released = a.in.released(mbLeft)
	})
	track := sliderTrack(r)
	at := func(frac float32) int { return int(track.X + track.W*frac) }

	f.press(t, a, at(0.5), 110)
	if !closeTo(val, 50, 0.01) || u.active != "s" || u.focus != "s" {
		t.Fatalf("press on the track captures and jumps: val=%v active=%q focus=%q", val, u.active, u.focus)
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
	if released || u.active != "" || !closeTo(val, 50, 0.01) {
		t.Fatalf("release ends the capture and is consumed: released=%v active=%q val=%v", released, u.active, val)
	}
	if w, ok := u.find("s"); !ok || w.tip != "Rain\n50" {
		t.Fatalf("tooltip shows the value: %+v", w)
	}

	f.key(t, a, "ArrowRight", 0)
	if !closeTo(val, 55, 0.01) {
		t.Fatalf("Right = +step: %v", val)
	}
	f.key(t, a, "ArrowRight", input.ModShift)
	if !closeTo(val, 80, 0.01) {
		t.Fatalf("Shift+Right = +5 steps: %v", val)
	}
	f.key(t, a, "ArrowLeft", 0)
	f.key(t, a, "End", 0)
	if val != 100 {
		t.Fatalf("End = max: %v", val)
	}
	f.key(t, a, "Home", 0)
	if val != 0 {
		t.Fatalf("Home = min: %v", val)
	}

	// Discrete stops snap and are labelled.
	stop := 0.0
	dr := rect{X: 100, Y: 200, W: 300, H: 40}
	dsp := sliderSpec{Stops: []string{"Low", "Normal", "High"}}
	setHook(u, func(a *App) { a.uiSlider("d", dr, &stop, dsp) })
	dt := sliderTrack(dr)
	f.press(t, a, int(dt.X+dt.W*0.45), 205)
	if stop != 1 {
		t.Fatalf("0.45 along snaps to the middle stop: %v", stop)
	}
	f.move(t, a, int(dt.X+dt.W*0.9), 205)
	f.letGo(t, a, int(dt.X+dt.W*0.9), 205)
	if stop != 2 {
		t.Fatalf("0.9 along snaps to High: %v", stop)
	}
	f.key(t, a, "ArrowLeft", 0)
	if stop != 1 {
		t.Fatalf("Left = one stop back: %v", stop)
	}
	if w, _ := u.find("d"); w.tip != "Normal" {
		t.Fatalf("stop tooltip = %q", w.tip)
	}
	n := 3
	setHook(u, func(a *App) { a.uiSliderInt("i", dr, &n, sliderSpec{Min: 0, Max: 10}) })
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
	r := rect{X: 100, Y: 200, W: 300, H: 32}
	var changed, committed bool
	setHook(u, func(a *App) {
		c, m := a.uiTextField("t", r, &val, textFieldSpec{})
		changed, committed = changed || c, committed || m
	})
	f.frame(t, a)
	if a.textActive() {
		t.Fatal("an unfocused field does not own the keyboard")
	}

	f.clickAt(t, a, 380, 216) // right of the text: caret at the end
	if u.edit.id != "t" || u.caret != 2 || !a.textActive() || a.contextStack()[0] != input.CtxText {
		t.Fatalf("click focuses with the caret at the end: edit=%q caret=%d", u.edit.id, u.caret)
	}
	f.typeText(t, a, "cd")
	if val != "abcd" || u.caret != 4 {
		t.Fatalf("typing inserts at the caret: %q caret %d", val, u.caret)
	}

	f.key(t, a, "Home", 0)
	f.key(t, a, "End", input.ModShift)
	if u.caret != 4 || u.selStart != 0 || u.selEnd != 4 {
		t.Fatalf("Home then Shift+End selects everything: caret=%d sel=[%d,%d)", u.caret, u.selStart, u.selEnd)
	}
	f.typeText(t, a, "Z")
	if val != "Z" {
		t.Fatalf("typing replaces the selection: %q", val)
	}
	f.typeText(t, a, "xyz")
	f.key(t, a, "ArrowLeft", 0)
	f.key(t, a, "ArrowLeft", 0)
	f.typeText(t, a, "-")
	if val != "Zx-yz" {
		t.Fatalf("Left moves the caret, typing inserts there: %q", val)
	}

	// Backspace repeats: once at once, then after 24 frames, then every 3.
	f.cur.held, f.cur.pressed = []string{"Backspace"}, []string{"Backspace"}
	f.frame(t, a)
	if len(val) != 4 {
		t.Fatalf("first Backspace deletes at once: %q", val)
	}
	want := map[int]int{2: 4, 24: 4, 25: 3, 27: 3, 28: 2}
	for n := 2; n <= 28; n++ {
		f.frame(t, a)
		if l, ok := want[n]; ok && len(val) != l {
			t.Fatalf("frame %d of the hold: %q (len %d), want len %d", n, val, len(val), l)
		}
	}
	f.cur.held = nil
	f.frame(t, a)

	f.key(t, a, "A", input.ModCtrl)
	if u.selStart != 0 || u.selEnd != len([]rune(val)) {
		t.Fatalf("Ctrl+A selects all: [%d,%d)", u.selStart, u.selEnd)
	}
	f.typeText(t, a, "Q")
	if val != "Q" {
		t.Fatalf("typing over Ctrl+A: %q", val)
	}

	changed = false
	f.key(t, a, "Escape", 0)
	f.frame(t, a)
	if val != "ab" || u.edit.id != "" || a.textActive() || !changed {
		t.Fatalf("Esc restores the value from focus time and ends the edit: %q edit=%q changed=%v", val, u.edit.id, changed)
	}

	// Click-away commits and keeps the edit.
	f.clickAt(t, a, 380, 216)
	f.typeText(t, a, "!")
	committed = false
	f.clickAt(t, a, 700, 500)
	f.frame(t, a)
	if val != "ab!" || u.edit.id != "" || !committed {
		t.Fatalf("click elsewhere commits: %q edit=%q committed=%v", val, u.edit.id, committed)
	}

	// Enter commits a single-line field and a caret click lands mid-text.
	val = "hello"
	f.clickAt(t, a, 100+fieldPad+1, 216)
	if u.caret != 0 {
		t.Fatalf("a click at the left edge puts the caret first: %d", u.caret)
	}
	f.typeText(t, a, "X")
	committed = false
	f.key(t, a, "Enter", 0)
	f.frame(t, a)
	if val != "Xhello" || u.edit.id != "" || !committed {
		t.Fatalf("Enter commits: %q edit=%q committed=%v", val, u.edit.id, committed)
	}
	if a.textActive() {
		t.Fatal("global shortcuts are back after the edit")
	}

	// Delete, Normalize and Esc after a normalising commit.
	val = "x"
	spec := textFieldSpec{Normalize: strings.ToUpper}
	setHook(u, func(a *App) { a.uiTextField("t", r, &val, spec) })
	f.clickAt(t, a, 380, 216)
	f.typeText(t, a, "yz")
	f.key(t, a, "Home", 0)
	f.key(t, a, "Delete", 0)
	if val != "yz" {
		t.Fatalf("Delete removes after the caret: %q", val)
	}
	f.key(t, a, "Enter", 0)
	if val != "YZ" {
		t.Fatalf("Normalize runs on commit: %q", val)
	}
}

func TestTextFieldValidatorAndMaxLen(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	val := ""
	r := rect{X: 100, Y: 200, W: 200, H: 32}
	setHook(u, func(a *App) {
		a.uiTextField("n", r, &val, textFieldSpec{MaxLen: 3, Valid: digitsOnly, Placeholder: "seed"})
	})
	f.clickAt(t, a, 150, 216)
	f.typeText(t, a, "1a2b345")
	if val != "123" {
		t.Fatalf("digits only, at most 3: %q", val)
	}
	f.key(t, a, "Backspace", 0)
	f.typeText(t, a, "9")
	if val != "129" {
		t.Fatalf("after Backspace there is room again: %q", val)
	}
}

func TestTextFieldMultiline(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	val := "first line\nsecond"
	r := rect{X: 100, Y: 200, W: 240, H: 120}
	setHook(u, func(a *App) { a.uiTextField("m", r, &val, textFieldSpec{Multiline: true}) })
	f.clickAt(t, a, 330, 300) // below the text: caret at the end
	if u.caret != len([]rune(val)) {
		t.Fatalf("caret = %d", u.caret)
	}
	f.key(t, a, "ArrowUp", 0)
	if u.caret > len("first line") {
		t.Fatalf("Up moves to the first line: %d", u.caret)
	}
	f.key(t, a, "End", 0)
	if u.caret != len("first line") {
		t.Fatalf("End goes to the end of the visual line: %d", u.caret)
	}
	f.key(t, a, "Enter", 0)
	if val != "first line\n\nsecond" || u.edit.id != "m" {
		t.Fatalf("Enter inserts a newline in a multi-line field: %q", val)
	}
	f.typeText(t, a, "this extra text wraps around the narrow field")
	spans := layoutLines([]rune(val), tsBody, float64(fieldInner(r).W))
	if len(spans) < 4 {
		t.Fatalf("long text wraps into visual lines: %d", len(spans))
	}
	for _, sp := range spans {
		if w, _ := measure(val[sp.start:sp.end], tsBody); w > float64(fieldInner(r).W)+0.5 {
			t.Fatalf("line %q is %v wide", val[sp.start:sp.end], w)
		}
	}
	f.key(t, a, "Escape", 0)
	if val != "first line\nsecond" {
		t.Fatalf("Esc restores: %q", val)
	}
}

func TestTextBufEdits(t *testing.T) {
	t.Parallel()
	b := textBuf{rs: []rune("héllo")}
	b.move(2, false)
	b.insert([]rune("XY"), 0)
	if string(b.rs) != "héXYllo" || b.caret != 4 {
		t.Fatalf("%q %d", string(b.rs), b.caret)
	}
	b.move(0, true) // select to the start
	if lo, hi := b.sel(); lo != 0 || hi != 4 {
		t.Fatalf("sel [%d,%d)", lo, hi)
	}
	if !b.backspace() || string(b.rs) != "llo" {
		t.Fatalf("backspace deletes the selection: %q", string(b.rs))
	}
	if b.backspace() {
		t.Fatal("nothing before the caret")
	}
	b.move(3, false)
	if b.del() {
		t.Fatal("nothing after the caret")
	}
	if b.insert([]rune("abcdef"), 5); string(b.rs) != "llo"+"ab" {
		t.Fatalf("max length cuts the insert: %q", string(b.rs))
	}
	if b.insert(nil, 0) {
		t.Fatal("inserting nothing changes nothing")
	}
	if got := nearestIndex([]rune("abc"), 0); got != 0 {
		t.Fatalf("nearest at 0 = %d", got)
	}
	if got := nearestIndex([]rune("abc"), 1e6); got != 3 {
		t.Fatalf("nearest far right = %d", got)
	}
}

func TestScrollViewWheelAndThumb(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	r := rect{X: 100, Y: 100, W: 300, H: 200}
	var area scrollArea
	wheelLeft := 0.0
	setHook(u, func(a *App) {
		area = a.uiScrollArea("sv", r, 1000, scrollOpts{})
		wheelLeft = a.in.raw.wheelY
	})
	f.cur.mx, f.cur.my = 200, 200
	f.cur.wheelY = -1
	f.frame(t, a)
	if area.Off != scrollStep || wheelLeft != 0 {
		t.Fatalf("wheel down scrolls and is consumed: off=%v left=%v", area.Off, wheelLeft)
	}
	f.cur.wheelY = 1
	f.frame(t, a)
	f.cur.wheelY = 1
	f.frame(t, a)
	if area.Off != 0 {
		t.Fatalf("wheel up clamps at the top: %v", area.Off)
	}
	f.cur.mx, f.cur.my = 50, 50
	f.cur.wheelY = -1
	f.frame(t, a)
	if area.Off != 0 || wheelLeft != -1 {
		t.Fatalf("the wheel off the view is not taken: off=%v left=%v", area.Off, wheelLeft)
	}

	// The thumb: 200*200/1000 = 40 px tall at the top of the 8 px bar.
	bar := rect{X: r.Right() - scrollBarW, Y: r.Y, W: scrollBarW, H: r.H}
	thumb := scrollThumb(bar, 0, 800, 1000)
	if thumb.H != 40 || thumb.Y != 100 {
		t.Fatalf("thumb = %+v", thumb)
	}
	f.press(t, a, 396, 110)
	if u.active != "sv.thumb" {
		t.Fatalf("press on the thumb captures: %q", u.active)
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
	if u.active != "" {
		t.Fatalf("release frees the capture: %q", u.active)
	}

	// A click on the track below the thumb pages down; PageDown over the view too.
	f.clickAt(t, a, 396, 290)
	if !closeTo(float64(area.Off), 180, 0.5) {
		t.Fatalf("track click pages: %v", area.Off)
	}
	f.cur.mx, f.cur.my = 200, 200
	f.key(t, a, "PageDown", 0)
	if !closeTo(float64(area.Off), 360, 0.5) {
		t.Fatalf("PageDown pages: %v", area.Off)
	}
	f.key(t, a, "PageUp", 0)
	if !closeTo(float64(area.Off), 180, 0.5) {
		t.Fatalf("PageUp pages back: %v", area.Off)
	}
	if !a.uiWheelOver(200, 200) || a.uiWheelOver(50, 50) {
		t.Fatal("uiWheelOver follows the scroll view")
	}
}

func TestListSelectionAndKeys(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	r := rect{X: 100, Y: 100, W: 300, H: 260}
	sel := -1
	setHook(u, func(a *App) {
		a.uiList("l", r, 100, &sel, listSpec{Label: func(i int) string { return "row " + uiNum(i) }})
	})
	f.frame(t, a)
	f.clickAt(t, a, 150, 100+2*listRowH+5)
	if sel != 2 || u.focus != "l" {
		t.Fatalf("a click selects the row: sel=%d focus=%q", sel, u.focus)
	}
	f.key(t, a, "ArrowDown", 0)
	if sel != 3 {
		t.Fatalf("Down = next row: %d", sel)
	}
	f.key(t, a, "End", 0)
	if sel != 99 {
		t.Fatalf("End = last row: %d", sel)
	}
	if off := u.scroll["l"]; off != 100*listRowH-r.H {
		t.Fatalf("the selection is kept visible: off=%v", off)
	}
	f.key(t, a, "Home", 0)
	if sel != 0 || u.scroll["l"] != 0 {
		t.Fatalf("Home = first row, scrolled to the top: sel=%d off=%v", sel, u.scroll["l"])
	}
	first, last := listVisible(1000, 260, 26, 100)
	if first != 38 || last != 48 {
		t.Fatalf("only visible rows are laid out: %d..%d", first, last)
	}
	// virtualised: far fewer widgets than rows
	if len(u.cur) > 14 {
		t.Fatalf("%d widgets for 100 rows", len(u.cur))
	}
}

func TestTooltipDelayAndClamp(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	setHook(u, func(a *App) {
		a.uiTip("tip", rect{X: 200, Y: 200, W: 80, H: 30}, "Landing site\nTemperate forest, 14 C a year.")
	})
	f.cur.mx, f.cur.my = 210, 210
	for i := 1; i <= 23; i++ {
		f.frame(t, a)
	}
	if u.tipVisible() {
		t.Fatalf("no tooltip before the delay (%d frames)", u.tipFrames)
	}
	for i := 0; i < 5; i++ {
		f.frame(t, a)
	}
	if !u.tipVisible() {
		t.Fatalf("the tooltip shows after the delay (%d frames)", u.tipFrames)
	}
	f.move(t, a, 600, 400)
	f.frame(t, a)
	if u.tipVisible() {
		t.Fatal("moving off hides it")
	}
	f.move(t, a, 210, 210)
	if u.tipVisible() {
		t.Fatal("coming back restarts the delay")
	}
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	for i := 0; i < 30; i++ {
		f.frame(t, a)
		f.cur.press[mbLeft] = false
	}
	if u.tipVisible() {
		t.Fatal("no tooltip while a button is down")
	}
	f.cur.down[mbLeft] = false

	long := "Title that is rather long and will need wrapping onto several lines to fit\n" + strings.Repeat("A body sentence that keeps going. ", 12)
	c := tipLayout(long, 1280, 720, 1270, 710)
	if c.Box.W > tipMaxW || c.Box.X < tipMargin || c.Box.Y < tipMargin || c.Box.Right() > 1280-tipMargin || c.Box.Bottom() > 720-tipMargin {
		t.Fatalf("card must stay on screen and within 360 px: %+v", c.Box)
	}
	if len(c.Title) < 2 || len(c.Body) < 3 {
		t.Fatalf("title and body wrap: %d/%d lines", len(c.Title), len(c.Body))
	}
	if c.Box.Y+c.Box.H > 710 {
		t.Fatalf("near the bottom the card flips above the cursor: %+v", c.Box)
	}
	short := tipLayout("Draft [R]", 1280, 720, 100, 100)
	if short.Box.X != 114 || short.Box.Y != 120 || len(short.Body) != 0 {
		t.Fatalf("a short tip sits down-right of the cursor: %+v", short)
	}
	// paint draws it without panicking
	f.move(t, a, 210, 210)
	for i := 0; i < 30; i++ {
		f.frame(t, a)
	}
	a.uiPaint(ebiten.NewImage(1280, 720))
}

func TestModalEscClosesTopOnly(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	a.apply(intent{kind: intentPush, screen: screenOptions})
	f.frame(t, a)
	closed := []string{}
	for _, id := range []string{"one", "two"} {
		id := id
		a.uiPushModal(modal{id: id, title: id, w: 400, h: 200, onClose: func() { closed = append(closed, id) }})
	}
	f.frame(t, a)
	if a.uiModalTop() != "two" || a.contextStack()[0] != input.CtxModal {
		t.Fatalf("the modal is on top of the input stack: %v", a.contextStack())
	}
	for _, h := range a.layoutHits(a.size()) {
		if h.id == "options.back" {
			t.Fatal("a modal hides the screen's buttons from the hit list")
		}
	}

	f.key(t, a, "Escape", 0)
	if len(u.modals) != 1 || a.uiModalTop() != "one" || strings.Join(closed, ",") != "two" {
		t.Fatalf("Esc closes only the top modal: %v closed=%v", u.modals, closed)
	}
	if a.nav.screen() != screenOptions {
		t.Fatalf("and does not go Back: %s", a.nav.screen())
	}

	// A click on a button behind the modal does nothing.
	f.clickAt(t, a, 50, int(footerY(a.sh))+10)
	if a.nav.screen() != screenOptions || len(u.modals) != 1 {
		t.Fatalf("clicks outside are ignored: %s, %d modals", a.nav.screen(), len(u.modals))
	}

	// The X closes the top one too.
	_, tr, _, _ := modalGeometry(a.sw, a.sh, u.modals[0])
	f.clickAt(t, a, int(tr.Right())-modalCloseW/2-8, int(tr.Y+tr.H/2))
	if len(u.modals) != 0 || strings.Join(closed, ",") != "two,one" {
		t.Fatalf("the X closes: %v closed=%v", u.modals, closed)
	}
	if a.nav.screen() != screenOptions {
		t.Fatal("still on Options")
	}
	f.key(t, a, "Escape", 0)
	if a.nav.screen() != screenMainMenu {
		t.Fatalf("with no modal Esc goes Back: %s", a.nav.screen())
	}
}

func TestModalConfirmAndButtons(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	ok := 0
	a.uiConfirm("c", "Discard world?", "The generated world will be lost.", "Discard", true, func() { ok++ })
	f.frame(t, a)
	if len(u.modals) != 1 {
		t.Fatal("confirm opens")
	}
	w, found := u.find("modal.c.ok")
	if !found {
		t.Fatalf("OK button is registered: %+v", u.cur)
	}
	cx, cy := w.r.Center()
	f.clickAt(t, a, int(cx), int(cy))
	if ok != 1 || len(u.modals) != 0 {
		t.Fatalf("OK runs the action and closes: ok=%d modals=%d", ok, len(u.modals))
	}
	a.uiConfirm("c", "Discard world?", "The generated world will be lost.", "Discard", false, func() { ok++ })
	f.frame(t, a)
	w, _ = u.find("modal.c.cancel")
	cx, cy = w.r.Center()
	f.clickAt(t, a, int(cx), int(cy))
	if ok != 1 || len(u.modals) != 0 {
		t.Fatalf("Cancel closes without acting: ok=%d", ok)
	}
	a.uiPushModal(modal{id: "x", title: "X", w: 300, h: 200})
	a.uiPushModal(modal{id: "x", title: "X", w: 300, h: 200})
	if len(u.modals) != 1 {
		t.Fatal("a modal id opens once")
	}
	a.uiCloseModal("x")
	if len(u.modals) != 0 {
		t.Fatal("closed by id")
	}
}

func TestTableSort(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	names := []string{"cara", "adam", "bea"}
	scores := []int{5, 9, 1}
	sel := -1
	r := rect{X: 100, Y: 100, W: 400, H: 200}
	cols := []tableCol{
		{ID: "name", Title: "Name", Cell: func(i int) string { return names[i] }, Less: func(x, y int) bool { return names[x] < names[y] }},
		{ID: "score", Title: "Score", Cell: func(i int) string { return uiNum(scores[i]) }, Less: func(x, y int) bool { return scores[x] < scores[y] },
			Tip: func(i int) string { return "score of " + names[i] }},
		{ID: "note", Title: "Note", Cell: func(i int) string { return "-" }},
	}
	setHook(u, func(a *App) { a.uiTable("tb", r, cols, 3, &sel, 0) })
	f.frame(t, a)
	order := func() []int { return a.tableOrder("tb", cols, 3) }
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
	colW := (r.W - scrollBarW) / 3
	nameX, scoreX, noteX := int(r.X+colW/2), int(r.X+colW*1.5), int(r.X+colW*2.5)
	f.clickAt(t, a, nameX, 110)
	if !eq(order(), 1, 2, 0) {
		t.Fatalf("name ascending = %v", order())
	}
	f.clickAt(t, a, nameX, 110)
	if col, desc := a.uiTableSort("tb"); col != 0 || !desc || !eq(order(), 0, 2, 1) {
		t.Fatalf("again reverses: col=%d desc=%v %v", col, desc, order())
	}
	f.clickAt(t, a, scoreX, 110)
	if col, desc := a.uiTableSort("tb"); col != 1 || desc || !eq(order(), 2, 0, 1) {
		t.Fatalf("another column sorts ascending: col=%d desc=%v %v", col, desc, order())
	}
	f.clickAt(t, a, noteX, 110)
	if col, _ := a.uiTableSort("tb"); col != 1 {
		t.Fatalf("an unsortable header does nothing: %d", col)
	}

	// Rows: display row 0 is data row 2 ("bea", the lowest score).
	f.clickAt(t, a, nameX, 100+tableHeaderH+listRowH/2)
	if sel != 2 {
		t.Fatalf("selection is a data row: %d", sel)
	}
	f.clickAt(t, a, nameX, 100+tableHeaderH+listRowH+5)
	if sel != 0 {
		t.Fatalf("second display row is data row 0: %d", sel)
	}
	// cell tooltip
	f.move(t, a, scoreX, 100+tableHeaderH+5)
	if u.hot == "" || !strings.Contains(u.hot, ".cell.0.score") {
		t.Fatalf("hover is the score cell: %q", u.hot)
	}
	if w, _ := u.find(u.hot); w.tip != "score of bea" {
		t.Fatalf("cell tip = %q", w.tip)
	}
	a.uiTableSetSort("tb", -1, false)
	if col, _ := a.uiTableSort("tb"); col != -1 {
		t.Fatal("sort cleared")
	}
	a.uiPaint(ebiten.NewImage(1280, 720))
}

func TestTabFocusOrder(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	on, text, tab := false, "name", 0
	v := 3.0
	setHook(u, func(a *App) {
		a.uiCheck("a", rect{X: 100, Y: 100, W: 200, H: 24}, "Check", &on, checkOpts{})
		a.uiSlider("b", rect{X: 100, Y: 140, W: 200, H: 30}, &v, sliderSpec{Min: 0, Max: 10})
		a.uiTextField("c", rect{X: 100, Y: 190, W: 200, H: 30}, &text, textFieldSpec{})
		a.uiTabs("d", rect{X: 100, Y: 240, W: 300, H: 28}, []string{"One", "Two", "Three"}, &tab, tabsOpts{})
		a.uiButton("e", rect{X: 100, Y: 290, W: 100, H: 30}, "Go", btnOpts{Disabled: true, Reason: "not yet"})
	})
	f.frame(t, a)
	step := func(mods input.Mod) string {
		f.key(t, a, "Tab", mods)
		return u.focus
	}
	for _, want := range []string{"a", "b", "c", "d", "a"} {
		if got := step(0); got != want {
			t.Fatalf("Tab -> %q, want %q", got, want)
		}
		if want == "c" && u.edit.id != "c" {
			t.Fatal("Tab onto a text field starts editing it")
		}
	}
	if got := step(input.ModShift); got != "d" {
		t.Fatalf("Shift+Tab wraps back to %q", got)
	}
	if u.edit.id != "" {
		t.Fatal("moving off the field ended its edit")
	}
	if got := step(input.ModShift); got != "c" || u.edit.id != "c" {
		t.Fatalf("Shift+Tab -> %q (edit %q)", got, u.edit.id)
	}
	if got := step(input.ModShift); got != "b" {
		t.Fatalf("Shift+Tab out of a field -> %q", got)
	}
	// The disabled button is skipped; Tab never reaches the Architect binding.
	f.key(t, a, "Tab", 0) // c
	f.key(t, a, "Tab", 0) // d
	f.key(t, a, "Tab", 0)
	if u.focus != "a" {
		t.Fatalf("e is disabled, so after d comes a: %q", u.focus)
	}
	f.key(t, a, "Space", 0)
	if !on {
		t.Fatal("Space toggles the focused checkbox")
	}
	f.key(t, a, "Space", 0)
	if on {
		t.Fatal("and back")
	}
	// Arrow keys inside the tab strip.
	u.focus = "d"
	f.key(t, a, "ArrowRight", 0)
	f.key(t, a, "ArrowRight", 0)
	f.key(t, a, "ArrowRight", 0)
	if tab != 2 {
		t.Fatalf("Right moves to the next tab and stops at the end: %d", tab)
	}
	f.key(t, a, "ArrowLeft", 0)
	if tab != 1 {
		t.Fatalf("Left = previous tab: %d", tab)
	}
	a.uiPaint(ebiten.NewImage(1280, 720)) // the focus ring paints
}

func TestCheckRadioAndButtons(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	on, pick, tab := false, 0, 0
	clicks := 0
	setHook(u, func(a *App) {
		a.uiCheck("chk", rect{X: 100, Y: 100, W: 200, H: 24}, "Ruins", &on, checkOpts{})
		a.uiRadio("rad", rect{X: 100, Y: 140, W: 200, H: 72}, []string{"One", "Two", "Locked"}, &pick, radioOpts{Disabled: []string{"", "", "needs a map"}})
		a.uiTabs("tabs", rect{X: 100, Y: 240, W: 400, H: 28}, []string{"A", "B", "C"}, &tab, tabsOpts{Disabled: []string{"", "", "later"}})
		if a.uiButton("btn", rect{X: 100, Y: 300, W: 100, H: 30}, "Go", btnOpts{}) {
			clicks++
		}
		if a.uiButton("dis", rect{X: 220, Y: 300, W: 100, H: 30}, "No", btnOpts{Disabled: true, Reason: "pick a tile first"}) {
			clicks += 100
		}
	})
	f.frame(t, a)
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
	f.key(t, a, "ArrowUp", 0)
	if pick != 0 {
		t.Fatalf("Up = previous option: %d", pick)
	}
	f.key(t, a, "ArrowDown", 0)
	f.key(t, a, "ArrowDown", 0)
	if pick != 1 {
		t.Fatalf("Down skips the disabled option: %d", pick)
	}
	rects := tabRects(rect{X: 100, Y: 240, W: 400, H: 28}, []string{"A", "B", "C"})
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
		if b.id == "btn" {
			found = true
		}
	}
	ok := a.uiClick("btn")
	f.frame(t, a) // the widget reports the click on its next call
	if !found || !ok || clicks != 2 || a.uiClick("nope") {
		t.Fatalf("widgets are in the hit list and clickable by id: found=%v clicks=%d", found, clicks)
	}
	a.uiPaint(ebiten.NewImage(1280, 720))
}

func TestWheelRoutingMenuVsCamera(t *testing.T) {
	t.Parallel()
	// In a menu the scroll view takes the wheel from the frame.
	a, f, u := newKit(t)
	r := rect{X: 100, Y: 100, W: 300, H: 200}
	left := 0.0
	setHook(u, func(a *App) {
		a.uiScrollArea("menu", r, 900, scrollOpts{})
		left = a.in.raw.wheelY
	})
	f.cur.mx, f.cur.my = 200, 200
	f.cur.wheelY = -2
	f.frame(t, a)
	if left != 0 || u.scroll["menu"] != 2*scrollStep {
		t.Fatalf("a hovered menu consumes the wheel: left=%v off=%v", left, u.scroll["menu"])
	}

	// Over the colony, a modal's list scrolls and the camera stays put; off
	// the list the wheel zooms the map as before.
	ca, _, cs := inputColony(t)
	ca.renderer.smooth.instant = true
	lr := rect{X: 300, Y: 200, W: 300, H: 200}
	ca.uiPushModal(modal{id: "list", title: "List", w: 400, h: 300, draw: func(a *App, body rect) {
		a.uiScrollArea("modal.list", lr, 900, scrollOpts{})
	}})
	cs.frame(t, ca)
	z := ca.renderer.cam.zoom
	cs.cur.mx, cs.cur.my = 400, 300
	cs.cur.wheelY = 1
	cs.frame(t, ca)
	if ca.renderer.cam.zoom != z {
		t.Fatal("the wheel over a scroll view does not zoom the camera")
	}
	if ca.uiSt().scroll["modal.list"] != 0 {
		t.Fatal("scrolling up at the top stays at the top")
	}
	cs.cur.wheelY = -1
	cs.frame(t, ca)
	if ca.uiSt().scroll["modal.list"] != scrollStep || ca.renderer.cam.zoom != z {
		t.Fatalf("it scrolls the list instead: off=%v zoom %v -> %v", ca.uiSt().scroll["modal.list"], z, ca.renderer.cam.zoom)
	}
	cs.cur.mx, cs.cur.my = 5, 5
	cs.cur.wheelY = 1
	cs.frame(t, ca)
	if ca.renderer.cam.zoom <= z {
		t.Fatal("the wheel away from every scroll view zooms the map")
	}
	// PageUp over a hovered scroll view does not zoom either.
	z = ca.renderer.cam.zoom
	cs.cur.mx, cs.cur.my = 400, 300
	cs.key(t, ca, "PageUp", 0)
	if ca.renderer.cam.zoom != z {
		t.Fatal("PageUp belongs to the scroll view under the cursor")
	}
}

func TestSetupMinWindow(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		w, h  int
		small bool
	}{{1279, 720, true}, {1280, 719, true}, {800, 600, true}, {1280, 720, false}, {1920, 1080, false}} {
		if tooSmall(c.w, c.h) != c.small {
			t.Fatalf("tooSmall(%d,%d) != %v", c.w, c.h, c.small)
		}
	}
	a, f, u := newKit(t)
	setHook(u, func(a *App) {
		sw, sh := a.size()
		if a.setupTooSmall(sw, sh) {
			return
		}
		a.uiButton("btn", rect{X: 100, Y: 100, W: 100, H: 30}, "Go", btnOpts{})
	})
	hasBtn := func() bool {
		for _, b := range a.layoutHits(a.size()) {
			if b.id == "btn" {
				return true
			}
		}
		return false
	}
	a.sw, a.sh = 1100, 700
	f.frame(t, a)
	if hasBtn() {
		t.Fatal("a window below 1280x720 shows no widgets (they would overlap)")
	}
	img := ebiten.NewImage(1100, 700)
	drawTooSmall(img, 1100, 700)
	a.uiPaint(img)
	a.sw, a.sh = 1280, 720
	f.frame(t, a)
	if !hasBtn() {
		t.Fatal("at 1280x720 the widgets are back")
	}
}

func TestDialogFrameLayout(t *testing.T) {
	t.Parallel()
	frame, title, content, footer := dialogFrame(1280, 720)
	if frame != (rect{X: 40, Y: 40, W: 1200, H: 640}) {
		t.Fatalf("frame at 1280x720 = %+v", frame)
	}
	if title != (rect{X: 40, Y: 40, W: 1200, H: 44}) || footer != (rect{X: 40, Y: 624, W: 1200, H: 56}) {
		t.Fatalf("title %+v footer %+v", title, footer)
	}
	if content != (rect{X: 56, Y: 100, W: 1168, H: 508}) {
		t.Fatalf("content = %+v (16 px padding between the bars)", content)
	}
	frame, _, _, _ = dialogFrame(1920, 1080)
	if frame != (rect{X: 340, Y: 160, W: 1240, H: 760}) {
		t.Fatalf("frame at 1920x1080 is capped and centred: %+v", frame)
	}
	back, next := footerBack(footer), footerNext(footer)
	if back != (rect{X: 56, Y: 634, W: 140, H: 36}) || next != (rect{X: 1064, Y: 634, W: 160, H: 36}) {
		t.Fatalf("back %+v next %+v", back, next)
	}

	r := rect{X: 10, Y: 20, W: 100, H: 60}
	l, rt := r.SplitH(30)
	if l != (rect{X: 10, Y: 20, W: 30, H: 60}) || rt != (rect{X: 40, Y: 20, W: 70, H: 60}) {
		t.Fatalf("splitH %+v %+v", l, rt)
	}
	l, rt = r.SplitH(-30)
	if l.W != 70 || rt != (rect{X: 80, Y: 20, W: 30, H: 60}) {
		t.Fatalf("splitH(-30) cuts from the right: %+v %+v", l, rt)
	}
	tp, bt := r.SplitV(-20)
	if tp.H != 40 || bt != (rect{X: 10, Y: 60, W: 100, H: 20}) {
		t.Fatalf("splitV(-20) %+v %+v", tp, bt)
	}
	cols := rect{X: 0, Y: 0, W: 100, H: 10}.Columns(3, 5)
	if len(cols) != 3 || cols[0].W != 30 || cols[1].X != 35 || cols[2].Right() != 100 {
		t.Fatalf("columns = %+v", cols)
	}
	rows := rect{X: 0, Y: 0, W: 50, H: 100}.Rows(20, 5)
	if len(rows) != 4 || rows[3].Y != 75 {
		t.Fatalf("rows fit the height: %+v", rows)
	}
	if got := r.RowsN(5, 20, 0); len(got) != 5 || got[4].Y != 100 {
		t.Fatalf("rowsN overflows on purpose: %+v", got)
	}
	if !r.Contains(10, 20) || r.Contains(110, 20) || !r.Hit(50, 50) || r.Hit(9, 50) {
		t.Fatal("contains / hit")
	}
	if x, y := r.Center(); x != 60 || y != 50 {
		t.Fatalf("center %v,%v", x, y)
	}
	if in := r.Inset(5); in != (rect{X: 15, Y: 25, W: 90, H: 50}) {
		t.Fatalf("inset %+v", in)
	}
	if !r.Intersect(rect{X: 200, Y: 200, W: 5, H: 5}).Empty() || r.Intersect(rect{X: 50, Y: 50, W: 100, H: 100}) != (rect{X: 50, Y: 50, W: 60, H: 30}) {
		t.Fatal("intersect")
	}
	if c := r.Centered(20, 20); c != (rect{X: 50, Y: 40, W: 20, H: 20}) {
		t.Fatalf("centered %+v", c)
	}
}

func TestKeyRepeatSchedule(t *testing.T) {
	t.Parallel()
	var fired []int
	for n := 1; n <= 34; n++ {
		if repeatFire(n) {
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
	f.cur.held = []string{"ArrowDown"}
	f.frame(t, a)
	if !a.keyRepeat("ArrowDown") || a.keyRepeat("ArrowUp") {
		t.Fatal("keyRepeat follows the held keys")
	}
	f.cur.held = nil
	f.frame(t, a)
	if a.keyRepeat("ArrowDown") {
		t.Fatal("released keys do not repeat")
	}
}

func TestProgressAndSparkline(t *testing.T) {
	t.Parallel()
	r := rect{X: 0, Y: 0, W: 100, H: 40}
	pts := sparkPoints([]int{1, 5, 3, 9}, r)
	if len(pts) != 4 || pts[0][0] != sparkPad || pts[3][0] != 100-sparkPad {
		t.Fatalf("x spans the box: %v", pts)
	}
	if !(pts[3][1] < pts[1][1] && pts[1][1] < pts[2][1] && pts[2][1] < pts[0][1]) {
		t.Fatalf("larger values sit higher: %v", pts)
	}
	flat := sparkPoints([]int{4, 4, 4}, r)
	if flat[0][1] != 20 || sparkPoints(nil, r) != nil {
		t.Fatalf("flat series sits mid-height: %v", flat)
	}
	one := sparkPoints([]int{7}, r)
	if len(one) != 1 || one[0][0] != 50 {
		t.Fatalf("single point is centred: %v", one)
	}
	img := ebiten.NewImage(200, 100)
	drawSparkline(img, r, []int{1, 5, 3, 9}, uiAccent)
	drawSparkline(img, r, nil, uiAccent)
	drawProgress(img, rect{X: 0, Y: 50, W: 160, H: 40}, "World", 0.4, "Rivers")
	drawProgress(img, rect{X: 0, Y: 50, W: 160, H: 40}, "World", 1.7, "")
}

// testScreen is a screen id nothing else uses, for the registry path.
const testScreen screenID = 99

var testScreenDrawn int

// registerTestScreen adds the test screen to a's table, as NewApp does
// for the real ones.
func registerTestScreen(a *App) {
	a.shell.screens[testScreen] = func(a *App, sw, sh int) {
		testScreenDrawn++
		a.uiPanel(rect{X: 10, Y: 10, W: 100, H: 100})
		a.uiLabel(20, 20, "registered", tsBody)
	}
}

func TestRegisteredScreenUIAndPaintSmoke(t *testing.T) {
	a, f, u := newKit(t)
	registerTestScreen(a)
	before := testScreenDrawn
	a.nav.push(frame{screen: testScreen})
	f.frame(t, a)
	f.frame(t, a)
	if testScreenDrawn < before+2 {
		t.Fatal("the registered screen hook runs once per Update")
	}
	if a.uiHot("x") || a.uiFocused("x") {
		t.Fatal("nothing hot or focused")
	}
	img := ebiten.NewImage(1280, 720)
	a.uiPaint(img)
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
		a.uiSlider("st", rect{X: 50, Y: 50, W: 300, H: 40}, &v, sliderSpec{Stops: []string{"Low", "Normal", "High"}})
		a.uiSlider("ct", rect{X: 50, Y: 100, W: 300, H: 30}, &sv, sliderSpec{Min: 0, Max: 1, Disabled: true, Reason: "locked", Format: func(f float64) string { return "x" }})
		a.uiTextField("tf", rect{X: 50, Y: 150, W: 200, H: 30}, &text, textFieldSpec{})
		a.uiTextField("mf", rect{X: 50, Y: 200, W: 160, H: 80}, &multi, textFieldSpec{Multiline: true})
		a.uiTextField("ph", rect{X: 300, Y: 150, W: 200, H: 30}, new(string), textFieldSpec{Placeholder: "seed"})
		a.uiTextField("dis", rect{X: 300, Y: 200, W: 200, H: 30}, &text, textFieldSpec{Disabled: true, Reason: "locked"})
		a.uiList("ls", rect{X: 50, Y: 300, W: 200, H: 100}, 8, &sel, listSpec{Label: func(i int) string { return "item " + uiNum(i) }, Disabled: func(i int) string {
			if i == 5 {
				return "gone"
			}
			return ""
		}})
		a.uiCheck("ck", rect{X: 300, Y: 300, W: 150, H: 24}, "Check", &on, checkOpts{Disabled: true, Reason: "r"})
		a.uiRadio("rd", rect{X: 300, Y: 340, W: 150, H: 48}, []string{"a", "b"}, &pick, radioOpts{})
		a.uiButton("b1", rect{X: 300, Y: 400, W: 90, H: 30}, "Primary", btnOpts{Style: btnPrimary})
		a.uiButton("b2", rect{X: 400, Y: 400, W: 90, H: 30}, "Danger", btnOpts{Style: btnDanger, Tip: "Careful\nIt is final."})
	})
	f.clickAt(t, a, 100, 165) // edit the single-line field
	f.key(t, a, "ArrowLeft", input.ModShift)
	f.key(t, a, "ArrowLeft", input.ModShift)
	a.uiPaint(img)
	f.cur.held = []string{"Meta"}
	f.key(t, a, "A", 0)
	f.cur.held = nil
	if u.selStart != 0 || u.selEnd != len(text) {
		t.Fatalf("Cmd+A selects all: [%d,%d)", u.selStart, u.selEnd)
	}
	f.key(t, a, "ArrowLeft", 0)
	if u.caret != 0 || u.selStart != u.selEnd {
		t.Fatalf("Left collapses a selection to its start: caret=%d", u.caret)
	}
	f.key(t, a, "ArrowRight", input.ModShift)
	f.key(t, a, "ArrowRight", 0)
	if u.caret != 1 {
		t.Fatalf("Right collapses to its end: %d", u.caret)
	}
	a.settings.Accessibility.ReducedMotion = true
	a.uiPaint(img)
	a.settings.Accessibility.ReducedMotion = false
	for i := 0; i < 70; i++ { // the caret blinks through both phases
		a.updates++
		a.uiPaint(img)
	}
	f.clickAt(t, a, 100, 240) // the multi-line field takes over editing
	f.key(t, a, "ArrowDown", input.ModShift)
	f.key(t, a, "ArrowUp", 0)
	a.uiPaint(img)
	f.clickAt(t, a, 700, 600)
	f.move(t, a, 70, 320)
	a.uiPaint(img)
}
