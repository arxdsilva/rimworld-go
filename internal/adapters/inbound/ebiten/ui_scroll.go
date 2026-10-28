package ebiten

import (
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Scroll views (NG-1 §1.4): a rectangle over taller content with an 8 px
// scrollbar, wheel routing, a draggable thumb, PageUp/PageDown and a
// virtualised selectable list on top. Offsets live in uiState.scroll[id].

const (
	scrollBarW   = 8
	scrollMinThb = 24
	scrollStep   = 48 // px per wheel notch when the caller gives none
	listRowH     = 26
	pageFrac     = 0.9 // a PageUp/PageDown moves this much of the view
)

// scrollOpts are a scroll area's optional parts.
type scrollOpts struct {
	Step      float32 // px per wheel notch (default 48)
	Focusable bool    // Tab can focus it, so PageUp/PageDown reach it
}

// scrollArea is where a scroll view's content goes this frame.
type scrollArea struct {
	R, View  rect    // View is R without the scrollbar
	Off      float32 // content offset in px
	ContentH float32
}

// scrollThumb is the thumb's rectangle on the bar for an offset.
func scrollThumb(bar rect, off, maxOff, contentH float32) rect {
	h := bar.H
	if contentH > 0 {
		h = max(scrollMinThb, bar.H*bar.H/contentH)
	}
	h = min(h, bar.H)
	y := bar.Y
	if maxOff > 0 {
		y += (bar.H - h) * off / maxOff
	}
	return rect{X: bar.X, Y: y, W: bar.W, H: h}
}

// scrollOffFromThumb is the offset for a thumb whose top is at y.
func scrollOffFromThumb(bar rect, thumbH, y, maxOff float32) float32 {
	span := bar.H - thumbH
	if span <= 0 {
		return 0
	}
	return max(0, min(maxOff, (y-bar.Y)/span*maxOff))
}

// uiScrollArea declares a scroll view over contentH px of content and
// reads its input: the wheel over r (consumed, so the colony camera does
// not zoom), the thumb drag, a click on the track and PageUp/PageDown
// while it is focused or hovered. The caller draws its content in
// area.View shifted up by area.Off and clipped to it.
func (a *App) uiScrollArea(id string, r rect, contentH float32, o scrollOpts) scrollArea {
	u := a.uiSt()
	if u.scroll == nil {
		u.scroll = map[string]float32{}
	}
	step := o.Step
	if step <= 0 {
		step = scrollStep
	}
	view := rect{X: r.X, Y: r.Y, W: max(0, r.W-scrollBarW), H: r.H}
	bar := rect{X: r.Right() - scrollBarW, Y: r.Y, W: scrollBarW, H: r.H}
	maxOff := max(0, contentH-r.H)
	off := max(0, min(maxOff, u.scroll[id]))
	thumbID := id + ".thumb"
	if u.add(uiWidget{id: id, r: r, wheel: true, focusable: o.Focusable}) {
		mx, my := a.currentCursor()
		hover := r.Hit(mx, my)
		if hover && a.in.raw.wheelY != 0 {
			off -= float32(a.in.raw.wheelY) * step
			a.in.raw.wheelY = 0
		}
		if maxOff > 0 {
			thumb := scrollThumb(bar, off, maxOff, contentH)
			if a.uiPress(bar) {
				if thumb.Hit(mx, my) {
					u.active, u.dragY = thumbID, float32(my)-thumb.Y
				} else if float32(my) < thumb.Y {
					off -= r.H * pageFrac
				} else {
					off += r.H * pageFrac
				}
			}
			if u.active == thumbID {
				off = scrollOffFromThumb(bar, thumb.H, float32(my)-u.dragY, maxOff)
				if !a.in.down(mbLeft) {
					u.active = ""
					a.uiRelease()
				}
			}
			if u.pageDelta != 0 && (u.focus == id || hover) {
				off += float32(u.pageDelta) * r.H * pageFrac
				u.pageDelta = 0
			}
		}
	}
	off = max(0, min(maxOff, off))
	u.scroll[id] = off
	u.queue(func(dst *ebiten.Image) {
		if maxOff <= 0 {
			return
		}
		fillRect(dst, bar, uiScrollTrack)
		thumb := scrollThumb(bar, u.scroll[id], maxOff, contentH)
		c := uiScrollThumb
		if u.active == thumbID || u.hot == id && thumb.Hit(u.mx, u.my) {
			c = lerpRGBA(c, uiText, 0.3)
		}
		fillRect(dst, thumb, c)
	})
	return scrollArea{R: r, View: view, Off: off, ContentH: contentH}
}

// uiScrollReveal scrolls the view id so the content span [y0, y1) shows.
func (a *App) uiScrollReveal(id string, viewH, y0, y1 float32) {
	u := a.uiSt()
	if u.scroll == nil {
		u.scroll = map[string]float32{}
	}
	off := u.scroll[id]
	switch {
	case y0 < off:
		off = y0
	case y1 > off+viewH:
		off = y1 - viewH
	}
	u.scroll[id] = max(0, off)
}

// listSpec describes a list's rows.
type listSpec struct {
	RowH float32 // default 26
	// Label is a row's text (the default row drawing).
	Label func(i int) string
	// Draw replaces the default row drawing; dst is clipped to the view and
	// row is in screen coordinates.
	Draw func(dst *ebiten.Image, i int, row rect, hover, selected bool)
	// Tip is a row's hover card; Disabled a reason that makes the row unselectable.
	Tip      func(i int) string
	Disabled func(i int) string
}

func (sp listSpec) rowH() float32 {
	if sp.RowH <= 0 {
		return listRowH
	}
	return sp.RowH
}

// listVisible is the span of rows [first, last] showing for an offset.
func listVisible(off, viewH, rowH float32, n int) (first, last int) {
	if n <= 0 || rowH <= 0 {
		return 0, -1
	}
	first = int(math.Floor(float64(off / rowH)))
	last = int(math.Ceil(float64((off+viewH)/rowH))) - 1
	return max(0, first), min(n-1, last)
}

// uiList is a virtualised, selectable list of n rows bound to *sel (-1 for
// none). Only the visible rows are laid out. A click selects a row; while
// the list has focus Up, Down, Home and End move the selection (kept in
// view) and PageUp/PageDown scroll. Row ids are id+".N". It reports the
// selection changed.
func (a *App) uiList(id string, r rect, n int, sel *int, sp listSpec) bool {
	u := a.uiSt()
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
			a.uiDeny(reason)
			return
		}
		if *sel != i {
			*sel, changed = i, true
		}
		a.uiOK()
	}
	if u.focus == id && u.layer == len(u.modals) && !u.suppress && n > 0 {
		next := *sel
		switch {
		case a.keyRepeat("ArrowUp"):
			next = max(0, *sel-1)
		case a.keyRepeat("ArrowDown"):
			next = min(n-1, *sel+1)
		case a.in.key("Home"):
			next = 0
		case a.in.key("End"):
			next = n - 1
		}
		if next != *sel && disabled(next) == "" {
			*sel, changed = next, true
			a.uiScrollReveal(id, r.H, float32(next)*rowH, float32(next+1)*rowH)
		}
	}
	area := a.uiScrollArea(id, r, float32(n)*rowH, scrollOpts{Step: rowH * 3, Focusable: true})
	first, last := listVisible(area.Off, r.H, rowH, n)
	rowAt := func(i int) rect {
		return rect{X: area.View.X, Y: r.Y + float32(i)*rowH - area.Off, W: area.View.W, H: rowH}
	}
	for i := first; i <= last; i++ {
		i := i
		hit := rowAt(i).Intersect(area.View)
		w := uiWidget{id: optID(id, i), r: hit, disabled: disabled(i) != "", reason: disabled(i), click: func() { u.focus = id; pick(i) }}
		if sp.Tip != nil {
			w.tip = sp.Tip(i)
		}
		if w.disabled {
			w.tip = w.reason
		}
		if u.add(w) && a.uiPress(hit) {
			u.focus = id
			pick(i)
		}
	}
	u.queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		for i := first; i <= last; i++ {
			row := rowAt(i)
			hover := u.hot == optID(id, i)
			selected := *sel == i
			if sp.Draw != nil {
				sp.Draw(sub, i, row, hover, selected)
				continue
			}
			drawListRow(sub, row, labelOf(sp, i), hover, selected, disabled(i) != "")
		}
	})
	return changed
}

func labelOf(sp listSpec, i int) string {
	if sp.Label != nil {
		return sp.Label(i)
	}
	return ""
}

// drawListRow is the default row: selection fill, hover wash, one line.
func drawListRow(dst *ebiten.Image, row rect, label string, hover, selected, disabled bool) {
	switch {
	case selected:
		fillRect(dst, row, uiBtnSelected)
	case hover && !disabled:
		fillRect(dst, row, uiBtnHover)
	}
	st := tsBody
	if disabled {
		st.Color = uiTextDisabled
	}
	drawText(dst, fitText(label, st, row.W-16), float64(row.X+8), float64(row.Y)+(float64(row.H)-lineH(st))/2, st)
}
