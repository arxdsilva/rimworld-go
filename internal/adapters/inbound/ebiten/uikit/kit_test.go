package uikit

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
)

// fakeHost is a headless host: the test sets the raw input of each frame.
type fakeHost struct {
	f      inputframe.Frame
	k      *Kit
	mx, my int
	n      uint64
	cues   []string
	hint   string
	calm   bool
}

func (h *fakeHost) Frame() *inputframe.Frame { return &h.f }
func (h *fakeHost) Cursor() (int, int)       { return h.mx, h.my }
func (h *fakeHost) Size() (int, int)         { return 1280, 720 }
func (h *fakeHost) Updates() uint64          { return h.n }
func (h *fakeHost) KeyRepeat(key string) bool {
	return h.f.Key(key) || inputframe.RepeatFire(h.k.HeldN[key])
}
func (h *fakeHost) Cue(id string)       { h.cues = append(h.cues, id) }
func (h *fakeHost) Hint(text string)    { h.hint = text }
func (h *fakeHost) ReducedMotion() bool { return h.calm }

func newTestKit() (*Kit, *fakeHost) {
	h := &fakeHost{}
	k := New(h)
	h.k = k
	return k, h
}

// step runs one frame the way a host does: held keys, the kit's keys, the
// frame, the modals, then the screen's widgets (declare), then a paint.
func step(k *Kit, h *fakeHost, raw inputframe.Raw, declare func()) {
	h.n++
	h.mx, h.my = raw.Mx, raw.My
	k.CountHeld(&raw)
	k.TakePages(&raw, raw.Mx, raw.My)
	k.TakeTab(&raw, false)
	h.f = inputframe.NewFrame(raw)
	k.Begin()
	k.RunModals(nil)
	if declare != nil {
		declare()
	}
	k.Paint(ebiten.NewImage(64, 64))
}

func at(x, y int) inputframe.Raw { return inputframe.Raw{Mx: x, My: y} }

func click(x, y int) inputframe.Raw {
	r := at(x, y)
	r.Press[inputframe.Left], r.Down[inputframe.Left] = true, true
	return r
}

func keys(x, y int, k ...string) inputframe.Raw {
	r := at(x, y)
	r.Pressed = k
	return r
}

func TestKitButtonClickDenyAndHost(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	r := gfx.Rect{X: 10, Y: 10, W: 100, H: 30}
	var got, gotOff bool
	ui := func() {
		got = k.Button("b.on", r, "Go", BtnOpts{Style: BtnPrimary, Tip: "Go\nnow"})
		gotOff = k.Button("b.off", gfx.Rect{X: 200, Y: 10, W: 80, H: 30}, "No",
			BtnOpts{Disabled: true, Reason: "Not yet"})
	}
	step(k, h, at(20, 20), ui)
	if !k.IsHot("b.on") || got {
		t.Fatalf("hover: hot %q got %v", k.Hot, got)
	}
	step(k, h, click(20, 20), ui)
	if !got || strings.Join(h.cues, ",") != "ui.click" {
		t.Fatalf("a click reports and cues: %v %v", got, h.cues)
	}
	step(k, h, click(210, 20), ui)
	if gotOff || h.hint != "Not yet" || h.cues[len(h.cues)-1] != "ui.deny" {
		t.Fatalf("a disabled click says why: %v %q %v", gotOff, h.hint, h.cues)
	}
	if w, ok := k.Find("b.off"); !ok || w.Tip != "Not yet" || !w.Disabled {
		t.Fatalf("disabled widget %+v", w)
	}
	if !k.Click("b.on") || k.Click("nope") {
		t.Fatal("the click path reaches a widget by id")
	}
	if k.Host() != Host(h) || !k.Bound() {
		t.Fatal("the kit keeps its host")
	}
}

func TestKitCheckRadioTabs(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	on, sel, tab := false, 0, 0
	var chk, rad, tb bool
	ui := func() {
		chk = k.Check("c", gfx.Rect{X: 0, Y: 0, W: 200, H: 24}, "Check", &on, CheckOpts{Tip: "tip"})
		rad = k.Radio("r", gfx.Rect{X: 0, Y: 100, W: 200, H: 72}, []string{"a", "b", "c"}, &sel,
			RadioOpts{Disabled: []string{"", "", "off"}, Tips: []string{"A"}})
		tb = k.Tabs("t", gfx.Rect{X: 0, Y: 300, W: 600, H: 30}, []string{"One", "Two"}, &tab, TabsOpts{})
	}
	step(k, h, click(5, 10), ui)
	if !chk || !on {
		t.Fatalf("check toggles: %v %v", chk, on)
	}
	step(k, h, click(5, 100+24+10), ui)
	if !rad || sel != 1 {
		t.Fatalf("radio selects: %v %d", rad, sel)
	}
	step(k, h, click(5, 100+48+10), ui)
	if sel != 1 || h.hint != "off" {
		t.Fatalf("a disabled option is refused: %d %q", sel, h.hint)
	}
	r := TabRects(gfx.Rect{X: 0, Y: 300, W: 600, H: 30}, []string{"One", "Two"})
	step(k, h, click(int(r[1].X)+5, 310), ui)
	if !tb || tab != 1 || k.Focus != "t" {
		t.Fatalf("tabs select: %v %d focus %q", tb, tab, k.Focus)
	}
	step(k, h, keys(0, 0, "ArrowLeft"), ui)
	if tab != 0 {
		t.Fatalf("Left moves to the previous tab: %d", tab)
	}
}

func TestKitSliderDragKeysAndTick(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	r := gfx.Rect{X: 100, Y: 100, W: 214, H: 40}
	val := 0.0
	ticks := 0
	sp := SliderSpec{Min: 0, Max: 10, Title: "Speed", Tick: func(lo, hi, old, cur float64) { ticks++ }}
	ui := func() { k.Slider("s", r, &val, sp) }
	track := SliderTrackRect(r)
	step(k, h, click(int(track.X+track.W/2), 105), ui)
	if val < 4.9 || val > 5.1 || k.Active != "s" || ticks == 0 {
		t.Fatalf("a press sets the value: %v active %q ticks %d", val, k.Active, ticks)
	}
	step(k, h, at(int(track.Right()), 105), ui) // released: the drag ends
	if k.Active != "" {
		t.Fatal("a release ends the drag")
	}
	step(k, h, keys(0, 0, "Home"), ui)
	if val != 0 {
		t.Fatalf("Home jumps to the start: %v", val)
	}
	n := 3
	ui2 := func() { k.SliderInt("i", r, &n, SliderSpec{Stops: []string{"a", "b", "c", "d"}}) }
	step(k, h, click(int(track.Right()), 105), ui2)
	if n != 3 {
		t.Fatalf("a discrete slider snaps to the last stop: %d", n)
	}
	if w, ok := k.Find("i"); !ok || w.Tip != "d" {
		t.Fatalf("the tooltip is the stop label: %+v", w)
	}
}

func TestKitTextFieldTypeAndCommit(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	r := gfx.Rect{X: 0, Y: 0, W: 200, H: 30}
	val := "ab"
	var changed, committed bool
	ui := func() {
		changed, committed = k.TextField("f", r, &val, TextFieldSpec{MaxLen: 4, Valid: DigitsOnly, Normalize: strings.TrimSpace})
	}
	step(k, h, click(190, 10), ui)
	if !k.Editing() || k.Focus != "f" {
		t.Fatal("a click starts editing")
	}
	raw := at(190, 10)
	raw.Chars = []rune("1x23")
	step(k, h, raw, func() { ui(); k.TextInput() })
	step(k, h, at(190, 10), ui)
	if val != "ab12" || !changed {
		t.Fatalf("typing honours Valid and MaxLen: %q %v", val, changed)
	}
	step(k, h, keys(190, 10, "Backspace"), func() { ui(); k.TextInput() })
	step(k, h, keys(190, 10, "Enter"), func() { ui(); k.TextInput() })
	step(k, h, at(190, 10), ui)
	if val != "ab1" || !committed || k.Editing() {
		t.Fatalf("Enter commits: %q %v %v", val, committed, k.Editing())
	}
	ml := "one two three four five six seven"
	mr := gfx.Rect{X: 0, Y: 100, W: 80, H: 60}
	step(k, h, click(5, 105), func() { k.TextField("m", mr, &ml, TextFieldSpec{Multiline: true}) })
	step(k, h, keys(5, 105, "ArrowDown"), func() { k.TextField("m", mr, &ml, TextFieldSpec{Multiline: true}); k.TextInput() })
	step(k, h, keys(5, 105, "End"), func() { k.TextField("m", mr, &ml, TextFieldSpec{Multiline: true}); k.TextInput() })
	if k.Caret == 0 {
		t.Fatal("the caret moves down the wrapped lines")
	}
	step(k, h, keys(5, 105, "Escape"), func() { k.TextField("m", mr, &ml, TextFieldSpec{Multiline: true}); k.TextInput() })
	if k.Editing() {
		t.Fatal("Esc stops editing")
	}
}

func TestKitScrollListTable(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	lr := gfx.Rect{X: 0, Y: 0, W: 200, H: 100}
	sel := -1
	ui := func() {
		k.List("l", lr, 50, &sel, ListSpec{Label: func(i int) string { return Num(i) }, Disabled: func(i int) string {
			if i == 2 {
				return "no"
			}
			return ""
		}})
	}
	step(k, h, click(10, 30), ui)
	if sel != 1 || k.Focus != "l" {
		t.Fatalf("a click selects row 1: %d", sel)
	}
	raw := at(10, 30)
	raw.WheelY = -2
	step(k, h, raw, ui)
	if k.Scroll["l"] <= 0 || !k.WheelOver(10, 30) {
		t.Fatalf("the wheel scrolls the list: %v", k.Scroll["l"])
	}
	step(k, h, keys(10, 30, "End"), ui)
	if sel != 49 {
		t.Fatalf("End selects the last row: %d", sel)
	}

	tr := gfx.Rect{X: 300, Y: 0, W: 300, H: 200}
	vals := []int{3, 1, 2}
	row := -1
	cols := []TableCol{
		{ID: "v", Title: "Value", Cell: func(i int) string { return Num(vals[i]) }, Less: func(a, b int) bool { return vals[a] < vals[b] }, Tip: func(i int) string { return "v" }},
		{ID: "n", Title: "Name", Align: gfx.AlignRight, Cell: func(i int) string { return "row" }},
	}
	tui := func() { k.Table("t", tr, cols, len(vals), &row, 0) }
	step(k, h, click(310, 10), tui)
	if c, desc := k.TableSort("t"); c != 0 || desc {
		t.Fatalf("a header click sorts: %d %v", c, desc)
	}
	if got := k.TableOrder("t", cols, 3); got[0] != 1 || got[2] != 0 {
		t.Fatalf("display order %v", got)
	}
	step(k, h, click(310, TableHeaderH+5), tui)
	if row != 1 {
		t.Fatalf("the first display row is data row 1: %d", row)
	}
	k.TableSetSort("t", -1, false)
	if c, _ := k.TableSort("t"); c != -1 {
		t.Fatal("a negative column clears the sort")
	}
}

func TestKitModalStackAndTooltip(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	closed := ""
	k.Push(Modal{ID: "one", Title: "One", W: 400, H: 300, OnClose: func() { closed += "one" }})
	k.Confirm("two", "Sure?", "Really do it?", "Do", true, func() { closed += "ok" })
	k.Push(Modal{ID: "two"}) // already open: nothing
	if len(k.Modals) != 2 || k.Top() != "two" || !k.ModalOpen() {
		t.Fatalf("stack %v", k.Modals)
	}
	pressed := false
	screen := func() { pressed = k.Button("under", gfx.Rect{X: 0, Y: 0, W: 50, H: 50}, "U", BtnOpts{}) }
	step(k, h, click(5, 5), screen)
	if pressed {
		t.Fatal("the screen under a modal is not live")
	}
	ok, found := k.Find("modal.two.ok")
	if !found {
		t.Fatal("the confirm's OK is declared")
	}
	cx, cy := ok.R.Center()
	step(k, h, click(int(cx), int(cy)), screen)
	if closed != "ok" || k.Top() != "one" {
		t.Fatalf("OK runs and closes: %q top %q", closed, k.Top())
	}
	if !k.CloseTop() || closed != "okone" || k.CloseTop() {
		t.Fatalf("CloseTop runs OnClose: %q", closed)
	}
	var body gfx.Rect
	anchor := gfx.Rect{X: 0, Y: 300, W: 500, H: 380}
	shut := 0
	win := Modal{ID: "w", Title: "W", W: 900, H: 900, Anchor: func() gfx.Rect { return anchor }, Draw: func(b gfx.Rect) { body = b },
		Buttons: []ModalButton{{ID: "x", Label: "Done", Closes: true}}}
	step(k, h, at(0, 0), func() { k.RunWindow(win, func() { shut += 10 }, func() { shut++ }) })
	if body.X != anchor.X+ModalPad || body.Y != anchor.Y+ModalTitleH+ModalPad {
		t.Fatalf("the window sits on its anchor: %+v", body)
	}
	done, _ := k.Find("modal.w.x")
	dx, dy := done.R.Center()
	step(k, h, click(int(dx), int(dy)), func() { k.RunWindow(win, func() { shut += 10 }, func() { shut++ }) })
	if shut != 1 {
		t.Fatalf("a closing footer button dismisses the window: %d", shut)
	}

	ui := func() { k.Tip("tip", gfx.Rect{X: 0, Y: 0, W: 50, H: 50}, "Title\nbody") }
	for i := 0; i <= TipDelay; i++ {
		step(k, h, at(10, 10), ui)
	}
	if !k.TipVisible() {
		t.Fatal("the tooltip shows after the delay")
	}
	c := TipLayout("Title\nbody", 1280, 720, 1270, 710)
	if c.Box.Right() > 1280-TipMargin || c.Box.Bottom() > 720-TipMargin || len(c.Body) != 1 {
		t.Fatalf("the card stays on screen: %+v", c)
	}
}

func TestKitTabFocusAndTooSmall(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	ui := func() {
		k.Button("a", gfx.Rect{X: 0, Y: 0, W: 50, H: 20}, "A", BtnOpts{})
		k.Button("b", gfx.Rect{X: 0, Y: 30, W: 50, H: 20}, "B", BtnOpts{})
	}
	step(k, h, at(500, 500), ui)
	step(k, h, keys(500, 500, "Tab"), ui)
	if k.Focus != "a" || !k.IsFocused("a") {
		t.Fatalf("Tab focuses the first widget: %q", k.Focus)
	}
	pressed := false
	step(k, h, keys(500, 500, "Tab"), ui)
	step(k, h, keys(500, 500, "Space"), func() {
		k.Button("a", gfx.Rect{X: 0, Y: 0, W: 50, H: 20}, "A", BtnOpts{})
		pressed = k.Button("b", gfx.Rect{X: 0, Y: 30, W: 50, H: 20}, "B", BtnOpts{})
	})
	if !pressed {
		t.Fatal("Space presses the focused button")
	}
	step(k, h, at(0, 0), func() {
		if !k.SetupTooSmall(800, 600) || k.SetupTooSmall(MinSetupW, MinSetupH) {
			t.Error("the setup minimum is 1280x720")
		}
		k.Button("a", gfx.Rect{X: 0, Y: 0, W: 50, H: 20}, "A", BtnOpts{})
	})
	if _, ok := k.Find("a"); ok {
		t.Fatal("a too-small screen declares no widgets")
	}
	k.Reset()
	if k.Focus != "" || len(k.Cur) != 0 {
		t.Fatal("Reset forgets focus and the hit list")
	}
}

func TestKitPagesAndHeldKeys(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	r := gfx.Rect{X: 0, Y: 0, W: 200, H: 100}
	ui := func() { k.ScrollArea("s", r, 1000, ScrollOpts{Focusable: true}) }
	step(k, h, at(10, 10), ui)
	raw := keys(10, 10, "PageDown")
	step(k, h, raw, ui)
	if k.Scroll["s"] <= 0 {
		t.Fatal("PageDown over a scroll view scrolls it")
	}
	k.ScrollReveal("s", 100, 0, 10)
	if k.Scroll["s"] != 0 {
		t.Fatalf("reveal scrolls back to the top: %v", k.Scroll["s"])
	}
	held := at(10, 10)
	held.Held = []string{"ArrowDown"}
	for i := 0; i < 3; i++ {
		step(k, h, held, nil)
	}
	if k.HeldN["ArrowDown"] != 3 {
		t.Fatalf("held frames counted: %d", k.HeldN["ArrowDown"])
	}
	pr := inputframe.Raw{Pressed: []string{"Escape", "A"}}
	if !RawPressed(&pr, "Escape") {
		t.Fatal("RawPressed")
	}
	DropPressed(&pr, "Escape")
	if RawPressed(&pr, "Escape") || !RawPressed(&pr, "A") {
		t.Fatalf("DropPressed: %v", pr.Pressed)
	}
}

func TestTextBufEdits(t *testing.T) {
	t.Parallel()
	b := TextBuf{Rs: []rune("héllo")}
	b.Move(2, false)
	b.Insert([]rune("XY"), 0)
	if string(b.Rs) != "héXYllo" || b.Caret != 4 {
		t.Fatalf("%q %d", string(b.Rs), b.Caret)
	}
	b.Move(0, true) // select to the start
	if lo, hi := b.Sel(); lo != 0 || hi != 4 {
		t.Fatalf("sel [%d,%d)", lo, hi)
	}
	if !b.Backspace() || string(b.Rs) != "llo" {
		t.Fatalf("backspace deletes the selection: %q", string(b.Rs))
	}
	if b.Backspace() {
		t.Fatal("nothing before the caret")
	}
	b.Move(3, false)
	if b.Del() {
		t.Fatal("nothing after the caret")
	}
	if b.Insert([]rune("abcdef"), 5); string(b.Rs) != "llo"+"ab" {
		t.Fatalf("max length cuts the insert: %q", string(b.Rs))
	}
	if b.Insert(nil, 0) {
		t.Fatal("inserting nothing changes nothing")
	}
	if got := NearestIndex([]rune("abc"), 0); got != 0 {
		t.Fatalf("nearest at 0 = %d", got)
	}
	if got := NearestIndex([]rune("abc"), 1e6); got != 3 {
		t.Fatalf("nearest far right = %d", got)
	}
}
