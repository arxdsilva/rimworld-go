package uikit

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Scroll views (NG-1 §1.4): a rectangle over taller content with an 8 px
// scrollbar, wheel routing, a draggable thumb, PageUp/PageDown and a
// virtualised selectable list on top. Offsets live in Kit.Scroll[id].

const (
	ScrollBarW   = 8
	scrollMinThb = 24
	ScrollStep   = 48 // px per wheel notch when the caller gives none
	ListRowH     = 26
	pageFrac     = 0.9 // a PageUp/PageDown moves this much of the view
)

// ScrollOpts are a scroll area's optional parts.
type ScrollOpts struct {
	Step      float32 // px per wheel notch (default 48)
	Focusable bool    // Tab can focus it, so PageUp/PageDown reach it
}

// ScrollArea is where a scroll view's content goes this frame.
type ScrollArea struct {
	R, View  gfx.Rect // View is R without the scrollbar
	Off      float32  // content offset in px
	ContentH float32
}

// ScrollThumbRect is the thumb's rectangle on the bar for an offset.
func ScrollThumbRect(bar gfx.Rect, off, maxOff, contentH float32) gfx.Rect {
	h := bar.H
	if contentH > 0 {
		h = max(scrollMinThb, bar.H*bar.H/contentH)
	}
	h = min(h, bar.H)
	y := bar.Y
	if maxOff > 0 {
		y += (bar.H - h) * off / maxOff
	}
	return gfx.Rect{X: bar.X, Y: y, W: bar.W, H: h}
}

// scrollOffFromThumb is the offset for a thumb whose top is at y.
func scrollOffFromThumb(bar gfx.Rect, thumbH, y, maxOff float32) float32 {
	span := bar.H - thumbH
	if span <= 0 {
		return 0
	}
	return max(0, min(maxOff, (y-bar.Y)/span*maxOff))
}

// ScrollArea declares a scroll view over contentH px of content and
// reads its input: the wheel over r (consumed, so the colony camera does
// not zoom), the thumb drag, a click on the track and PageUp/PageDown
// while it is focused or hovered. The caller draws its content in
// area.View shifted up by area.Off and clipped to it.
func (k *Kit) ScrollArea(id string, r gfx.Rect, contentH float32, o ScrollOpts) ScrollArea {
	if k.Scroll == nil {
		k.Scroll = map[string]float32{}
	}
	step := o.Step
	if step <= 0 {
		step = ScrollStep
	}
	view := gfx.Rect{X: r.X, Y: r.Y, W: max(0, r.W-ScrollBarW), H: r.H}
	bar := gfx.Rect{X: r.Right() - ScrollBarW, Y: r.Y, W: ScrollBarW, H: r.H}
	maxOff := max(0, contentH-r.H)
	off := max(0, min(maxOff, k.Scroll[id]))
	thumbID := id + ".thumb"
	if k.Add(Widget{ID: id, R: r, Wheel: true, Focusable: o.Focusable}) {
		mx, my := k.host.Cursor()
		hover := r.Hit(mx, my)
		if hover && k.in().Raw.WheelY != 0 {
			off -= float32(k.in().Raw.WheelY) * step
			k.in().Raw.WheelY = 0
		}
		if maxOff > 0 {
			thumb := ScrollThumbRect(bar, off, maxOff, contentH)
			if k.Press(bar) {
				if thumb.Hit(mx, my) {
					k.Active, k.dragY = thumbID, float32(my)-thumb.Y
				} else if float32(my) < thumb.Y {
					off -= r.H * pageFrac
				} else {
					off += r.H * pageFrac
				}
			}
			if k.Active == thumbID {
				off = scrollOffFromThumb(bar, thumb.H, float32(my)-k.dragY, maxOff)
				if !k.in().Down(inputframe.Left) {
					k.Active = ""
					k.release()
				}
			}
			if k.pageDelta != 0 && (k.Focus == id || hover) {
				off += float32(k.pageDelta) * r.H * pageFrac
				k.pageDelta = 0
			}
		}
	}
	off = max(0, min(maxOff, off))
	k.Scroll[id] = off
	k.Queue(func(dst *ebiten.Image) {
		if maxOff <= 0 {
			return
		}
		FillRect(dst, bar, ScrollTrack)
		thumb := ScrollThumbRect(bar, k.Scroll[id], maxOff, contentH)
		c := ScrollThumb
		if k.Active == thumbID || k.Hot == id && thumb.Hit(k.mx, k.my) {
			c = LerpRGBA(c, Text, 0.3)
		}
		FillRect(dst, thumb, c)
	})
	return ScrollArea{R: r, View: view, Off: off, ContentH: contentH}
}

// ScrollReveal scrolls the view id so the content span [y0, y1) shows.
func (k *Kit) ScrollReveal(id string, viewH, y0, y1 float32) {
	if k.Scroll == nil {
		k.Scroll = map[string]float32{}
	}
	off := k.Scroll[id]
	switch {
	case y0 < off:
		off = y0
	case y1 > off+viewH:
		off = y1 - viewH
	}
	k.Scroll[id] = max(0, off)
}

// ListSpec describes a list's rows.
type ListSpec struct {
	RowH float32 // default 26
	// Label is a row's text (the default row drawing).
	Label func(i int) string
	// Draw replaces the default row drawing; dst is clipped to the view and
	// row is in screen coordinates.
	Draw func(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool)
	// Tip is a row's hover card; Disabled a reason that makes the row unselectable.
	Tip      func(i int) string
	Disabled func(i int) string
}

func (sp ListSpec) rowH() float32 {
	if sp.RowH <= 0 {
		return ListRowH
	}
	return sp.RowH
}

// ListVisible is the span of rows [first, last] showing for an offset.
func ListVisible(off, viewH, rowH float32, n int) (first, last int) {
	if n <= 0 || rowH <= 0 {
		return 0, -1
	}
	first = int(math.Floor(float64(off / rowH)))
	last = int(math.Ceil(float64((off+viewH)/rowH))) - 1
	return max(0, first), min(n-1, last)
}

// List is a virtualised, selectable list of n rows bound to *sel (-1 for
// none). Only the visible rows are laid out. A click selects a row; while
// the list has focus Up, Down, Home and End move the selection (kept in
// view) and PageUp/PageDown scroll. Row ids are id+".N". It reports the
// selection changed.
func (k *Kit) List(id string, r gfx.Rect, n int, sel *int, sp ListSpec) bool {
	rowH := sp.rowH()
	changed := false
	disabled := func(i int) string {
		if sp.Disabled != nil {
			return sp.Disabled(i)
		}
		return ""
	}
	pick := func(i int) {
		if reason := disabled(i); reason != "" {
			k.Deny(reason)
			return
		}
		if *sel != i {
			*sel, changed = i, true
		}
		k.OK()
	}
	if k.Focus == id && k.Layer == len(k.Modals) && !k.Suppress && n > 0 {
		next := *sel
		switch {
		case k.host.KeyRepeat("ArrowUp"):
			next = max(0, *sel-1)
		case k.host.KeyRepeat("ArrowDown"):
			next = min(n-1, *sel+1)
		case k.in().Key("Home"):
			next = 0
		case k.in().Key("End"):
			next = n - 1
		}
		if next != *sel && disabled(next) == "" {
			*sel, changed = next, true
			k.ScrollReveal(id, r.H, float32(next)*rowH, float32(next+1)*rowH)
		}
	}
	area := k.ScrollArea(id, r, float32(n)*rowH, ScrollOpts{Step: rowH * 3, Focusable: true})
	first, last := ListVisible(area.Off, r.H, rowH, n)
	rowAt := func(i int) gfx.Rect {
		return gfx.Rect{X: area.View.X, Y: r.Y + float32(i)*rowH - area.Off, W: area.View.W, H: rowH}
	}
	for i := first; i <= last; i++ {
		i := i
		hit := rowAt(i).Intersect(area.View)
		w := Widget{ID: OptID(id, i), R: hit, Disabled: disabled(i) != "", Reason: disabled(i), Click: func() { k.Focus = id; pick(i) }}
		if sp.Tip != nil {
			w.Tip = sp.Tip(i)
		}
		if w.Disabled {
			w.Tip = w.Reason
		}
		if k.Add(w) && k.Press(hit) {
			k.Focus = id
			pick(i)
		}
	}
	k.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		for i := first; i <= last; i++ {
			row := rowAt(i)
			hover := k.Hot == OptID(id, i)
			selected := *sel == i
			if sp.Draw != nil {
				sp.Draw(sub, i, row, hover, selected)
				continue
			}
			DrawListRow(sub, row, labelOf(sp, i), hover, selected, disabled(i) != "")
		}
	})
	return changed
}

func labelOf(sp ListSpec, i int) string {
	if sp.Label != nil {
		return sp.Label(i)
	}
	return ""
}

// DrawListRow is the default row: selection fill, hover wash, one line.
func DrawListRow(dst *ebiten.Image, row gfx.Rect, label string, hover, selected, disabled bool) {
	switch {
	case selected:
		FillRect(dst, row, BtnSelected)
	case hover && !disabled:
		FillRect(dst, row, BtnHover)
	}
	st := gfx.Body
	if disabled {
		st.Color = TextDisabled
	}
	gfx.Text(dst, FitText(label, st, row.W-16), float64(row.X+8), float64(row.Y)+(float64(row.H)-gfx.LineH(st))/2, st)
}
