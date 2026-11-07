package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
)

// TestWorkCellTextStaysInBox (UIS-061): every priority number fits the 25 px
// box and sits centred in it, the passion glyph sits in the slot's gutter
// right of the box, and nothing leaves the slot (the button's rect).
func TestWorkCellTextStaysInBox(t *testing.T) {
	t.Parallel()
	_, w := workFixture()
	r := w.Rect()
	slot := WorkCellRect(r, 0, 3)
	box := workBoxRect(slot)
	if box.W != hudlayout.WorkBox || box.H != hudlayout.WorkBox || box.X != slot.X || slot.W != workCellW {
		t.Fatalf("box %+v in slot %+v", box, slot)
	}
	if want := r.X + hudlayout.WorkPadL + WorkNameW; WorkCellRect(r, 0, int(firstSlotCol())).X != want {
		t.Fatalf("first slot at %v, want %v", WorkCellRect(r, 0, int(firstSlotCol())).X, want)
	}
	for _, v := range []string{"1", "2", "3", "4", "-"} {
		tw, th := gfx.Measure(v, gfx.Body)
		top := gfx.CenterTop(float64(box.Y), float64(box.H), gfx.Body)
		cx := float64(box.X + box.W/2)
		if cx-tw/2 < float64(box.X) || cx+tw/2 > float64(box.Right()) || top < float64(box.Y) || top+th > float64(box.Bottom()) {
			t.Errorf("%q (%vx%v) at %v,%v leaves the box %+v", v, tw, th, cx, top, box)
		}
	}
	g := workGlyphRect(slot)
	if g.X < box.Right() || g.Right() > slot.Right() || g.Y < slot.Y || g.Bottom() > slot.Bottom() {
		t.Errorf("glyph %+v outside the gutter of %+v", g, slot)
	}
	if cy := g.Y + g.H/2; cy != slot.Y+slot.H/2 {
		t.Errorf("glyph centre %v, slot centre %v", cy, slot.Y+slot.H/2)
	}
	// the buttons are the slots
	for _, b := range w.Work().cellButtons() {
		if b.W != workCellW || b.H != hudlayout.WorkBox {
			t.Fatalf("cell button %s is %vx%v", b.ID, b.W, b.H)
		}
	}
}

func firstSlotCol() uint {
	for col := 0; col < len(WorkColumns); col++ {
		if workSlot(col) == 0 {
			return uint(col)
		}
	}
	return 0
}

// TestWorkTintGrowsWithSkill (UIS-061): the box fill brightens with every
// skill level from 0 to 20.
func TestWorkTintGrowsWithSkill(t *testing.T) {
	t.Parallel()
	sum := func(l int) int { c := workTint(l); return int(c.R) + int(c.G) + int(c.B) }
	for l := 1; l <= 20; l++ {
		if sum(l) < sum(l-1) {
			t.Fatalf("level %d (%d) is darker than level %d (%d)", l, sum(l), l-1, sum(l-1))
		}
	}
	if sum(20) <= sum(0) {
		t.Fatal("level 20 is not brighter than level 0")
	}
	if workTint(-3) != workTint(0) || workTint(99) != workTint(20) {
		t.Fatal("levels clamp to 0..20")
	}
	// urgency: 1 is the brightest number
	for p := 2; p <= 4; p++ {
		a, b := workNumColor(p-1), workNumColor(p)
		if int(b.R)+int(b.G)+int(b.B) > int(a.R)+int(a.G)+int(a.B) {
			t.Fatalf("priority %d is brighter than %d", p, p-1)
		}
	}
}
