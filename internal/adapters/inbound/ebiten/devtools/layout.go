package devtools

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
)

// Debug panels use the small type so a long log stays on one screen.
const (
	devStep      = 15  // line pitch
	devLogMaxW   = 460 // widest log line, px
	devLogChrome = 28  // the log panel's title and padding
)

// Column is the free band of the screen the developer panels live in:
// right of the resource list, under the colonist bar and the messages,
// above the lower-left stack (pane, Architect, window, command buttons)
// and left of the right column. Empty when there is no room.
func Column(l hudlayout.Layout) gfx.Rect {
	x := max(l.Resources.Right(), hudlayout.HudMargin+hudlayout.ResourcesW) + hudlayout.HudMargin
	y := float32(hudlayout.HudMargin)
	for _, r := range []gfx.Rect{l.Bar, l.Messages, l.Perf} {
		if !r.Empty() {
			y = max(y, r.Bottom()+hudlayout.HudMargin)
		}
	}
	bottom := l.Tabs.Y - hudlayout.HudMargin
	for _, r := range []gfx.Rect{l.Window, l.Expanded, l.PaneTabs, l.ArchDesc, l.ArchCats, l.Grid} {
		if !r.Empty() {
			bottom = min(bottom, r.Y-hudlayout.HudMargin)
		}
	}
	w, h := l.Right.X-hudlayout.HudMargin-x, bottom-y
	if w <= 0 || h <= 0 {
		return gfx.Rect{}
	}
	return gfx.Rect{X: x, Y: y, W: w, H: h}
}

// PanelRects are the developer panels' rects (empty: not shown).
type PanelRects struct{ Fields, Insp, Log gfx.Rect }

// Stack stacks the panels that are on, top to bottom in the dev column:
// the map-fields legend, the debug inspector, then the debug log, which
// takes the lines that fit.
func (p *Panel) Stack() PanelRects {
	var st PanelRects
	if !p.active() {
		return st
	}
	col := Column(p.sh.HUD())
	if col.Empty() {
		return st
	}
	y := col.Y
	place := func(w, h float32) gfx.Rect {
		if h <= 0 || y+h > col.Bottom() {
			return gfx.Rect{}
		}
		r := gfx.Rect{X: col.X, Y: y, W: min(w, col.W), H: h}
		y += h + hudlayout.HudGap
		return r
	}
	if p.State.Fields.Shown {
		st.Fields = place(p.legendSize())
	}
	if p.State.Insp {
		st.Insp = place(p.inspectorSize())
	}
	if p.State.Log {
		lines := p.logShown(int((col.Bottom()-y-devLogChrome)/devStep), col.W-20)
		if len(lines) > 0 {
			var w float32
			for _, l := range lines {
				w = max(w, gfx.TextW(l, gfx.Small))
			}
			st.Log = place(w+20, float32(len(lines)*devStep+devLogChrome))
		}
	}
	return st
}
