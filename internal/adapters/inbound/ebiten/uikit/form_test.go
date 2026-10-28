package uikit

import (
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// kitOwner is a screen that hands out its kit (uikit.Owner).
type kitOwner struct{ k *Kit }

func (o kitOwner) Kit() *Kit { return o.k }

func TestFormRowsScrollAndClip(t *testing.T) {
	t.Parallel()
	area := ScrollArea{View: gfx.Rect{X: 10, Y: 100, W: 208, H: 100}, Off: 30}
	f := NewForm(area)
	if f.X != 14 || f.W != 200 || f.Off != 30 {
		t.Fatalf("padding: %+v", f)
	}
	r, ok := f.Row(20) // content 0..20 shifted up by 30: above the view
	if ok || r.Y != 70 {
		t.Fatalf("a row above the view is not shown: %+v %v", r, ok)
	}
	f.Gap(10)
	r, ok = f.Row(40) // content 30..70: view 100..140
	if !ok || r.Y != 100 || r.H != 40 || r.W != 200 {
		t.Fatalf("a row inside the view: %+v %v", r, ok)
	}
	if f.Height() != 70 || f.Content() != 74 {
		t.Fatalf("height %v content %v", f.Height(), f.Content())
	}
	if _, ok := f.Row(80); ok {
		t.Fatal("a row past the bottom is not shown")
	}
}

func TestFormRowsDeclareTheirWidgets(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	o := kitOwner{k}
	v, on, days := 50, false, 3
	ticks := 0
	sp := SliderSpec{Min: 0, Max: 100, Step: 5, Tick: func(lo, hi, old, cur float64) { ticks++ }}
	var f *Form
	ui := func() {
		f = NewForm(ScrollArea{View: gfx.Rect{X: 0, Y: 0, W: 408, H: 400}})
		f.SectionTitle(o, "Threats")
		f.Slider(o, "ts", "Threat scale", &v, sp, func(x int) string { return Num(x) + "%" }, "", "Threat scale\nHow big.")
		f.Check(o, "bt", "Big threats", &on, "", "")
		f.Stepper(o, "pd", "Peaceful days", &days, 0, 5, "Peaceful\nDays.")
		off := 1
		f.Slider(o, "off", "Off", &off, SliderSpec{Min: 0, Max: 2}, Num, "Not now", "")
	}
	step(k, h, at(0, 0), ui)
	for _, id := range []string{"diff.f.ts", "diff.f.bt", "diff.f.pd.dec", "diff.f.pd.inc", "diff.f.off"} {
		if _, ok := k.Find(id); !ok {
			t.Fatalf("form row %s not declared", id)
		}
	}
	if w, _ := k.Find("diff.f.off"); !w.Disabled || w.Reason != "Not now" {
		t.Fatalf("a reason disables the slider: %+v", w)
	}
	if f.Height() != 26+40+28+32+40 {
		t.Fatalf("form height %v", f.Height())
	}
	// The check row toggles; the stepper's + steps within its range.
	step(k, h, click(5, 26+40+10), ui)
	if !on {
		t.Fatal("the check row toggles")
	}
	inc, _ := k.Find("diff.f.pd.inc")
	for range 4 {
		step(k, h, click(int(inc.R.X)+2, int(inc.R.Y)+2), ui)
		step(k, h, at(0, 0), ui)
	}
	if days != 5 {
		t.Fatalf("the stepper stops at its maximum: %d", days)
	}
	if w, _ := k.Find("diff.f.pd.inc"); !w.Disabled {
		t.Fatal("+ is disabled at the maximum")
	}
	// The slider row honours the caller's range, step and tick.
	sl, _ := k.Find("diff.f.ts")
	track := SliderTrackRect(sl.R)
	step(k, h, click(int(track.Right()), int(sl.R.Y)+4), ui)
	if v != 100 || ticks == 0 {
		t.Fatalf("slider to the end: %d ticks %d", v, ticks)
	}
}

func TestComposites(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	fired, sel, pick := 0, 0, -1
	ui := func() {
		StyledButton(k, "go", gfx.Rect{X: 0, Y: 0, W: 80, H: 30}, "Go", BtnOpts{}, func() { fired++ })
		StyledButton(k, "no", gfx.Rect{X: 100, Y: 0, W: 80, H: 30}, "No", BtnOpts{Disabled: true, Reason: "Later"}, func() { fired += 100 })
		Segmented(k, "seg", gfx.Rect{X: 0, Y: 50, W: 308, H: 30}, []string{"A", "B", "C"}, &sel, []string{"a"})
		Segmented(k, "none", gfx.Rect{}, nil, &sel, nil)
		Choice(k, "ch", gfx.Rect{X: 0, Y: 100, W: 200, H: 30}, "Pick", []string{"one", "two"}, 1, "", func(i int) { pick = i })
	}
	step(k, h, click(5, 5), ui)
	if fired != 1 {
		t.Fatalf("a press runs fn at once: %d", fired)
	}
	if !k.Click("go") || fired != 2 {
		t.Fatalf("the id-driven click runs fn at once: %d", fired)
	}
	if k.Click("no"); fired != 2 || h.hint != "Later" {
		t.Fatalf("a disabled button says why: %d %q", fired, h.hint)
	}
	step(k, h, click(210, 55), ui)
	if sel != 2 {
		t.Fatalf("segmented selects the third: %d", sel)
	}
	if w, _ := k.Find("seg.0"); w.Tip != "a" {
		t.Fatalf("segment tip: %+v", w)
	}
	if w, _ := k.Find("ch"); w.Label != "two  v" {
		t.Fatalf("the dropdown shows its option: %q", w.Label)
	}
	k.Click("ch")
	if k.Top() != "pick.ch" {
		t.Fatalf("the dropdown opens its list: %q", k.Top())
	}
	_, _, body, _ := ModalGeometry(1280, 720, k.Modals[0])
	step(k, h, click(int(body.X)+10, int(body.Y)+int(ListRowH)/2), ui)
	step(k, h, at(0, 0), ui)
	if pick != 0 || k.ModalOpen() {
		t.Fatalf("a row picks and closes: pick %d open %v", pick, k.ModalOpen())
	}
	step(k, h, at(0, 0), func() {
		Choice(k, "off", gfx.Rect{X: 0, Y: 0, W: 100, H: 30}, "Off", nil, 5, "No options", func(int) {})
	})
	if w, _ := k.Find("off"); !w.Disabled || w.Label != "-  v" {
		t.Fatalf("a disabled dropdown with no option: %+v", w)
	}
}

func TestTextHelpers(t *testing.T) {
	t.Parallel()
	k, h := newTestKit()
	var hgt float32
	var ctl gfx.Rect
	step(k, h, at(0, 0), func() {
		k.Layer = 3
		OnScreenLayer(k)
		hgt = TextLines(k, 0, 0, 60, "one two three four five six", gfx.Body)
		ctl = RowLabel(k, gfx.Rect{X: 10, Y: 0, W: 300, H: 30}, "A long label", 120)
	})
	if k.Layer != 0 {
		t.Fatal("OnScreenLayer puts the hook on the screen layer")
	}
	if lines := gfx.WrapPx("one two three four five six", gfx.Body, 60); hgt != float32(len(lines))*float32(gfx.LineH(gfx.Body)) || len(lines) < 2 {
		t.Fatalf("TextLines height %v for %d lines", hgt, len(lines))
	}
	if ctl != (gfx.Rect{X: 130, Y: 0, W: 180, H: 30}) {
		t.Fatalf("RowLabel leaves the control %+v", ctl)
	}
	if DimText().Color != TextDim {
		t.Fatal("DimText is dim")
	}
	if got := WrapCols("a bb ccccccccc dd", 4); !slices.Equal(got, []string{"a bb", "cccc", "cccc", "c dd"}) {
		t.Fatalf("WrapCols = %q", got)
	}
	if got := WrapCols("", 1); got != nil {
		t.Fatalf("WrapCols empty = %q", got)
	}
}
