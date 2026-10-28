// Package hudlayout is the colony HUD's pure layout: region rectangles from the screen size and the regions' wanted sizes. No state, no drawing.
package hudlayout

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

const (
	HudMargin = 8
	HudGap    = 4

	TabsH = 28 // the bottom tab strip

	PaneW     = 400 // the inspect pane, lower-left
	PaneH     = 188
	PaneTabsH = 24 // its tabs, just above it

	ExpandMinW = 400 // an inspector tab's panel, opened above the pane tabs
	ExpandMaxW = 560

	CmdCell    = 64 // a command button
	CmdStride  = 68 // cell + gap
	CmdMaxRows = 4

	RightW      = 232 // the right column: learning helper, alerts, letters, global controls
	MinimapSize = 180
	LetterRowH  = 28
	MaxLetters  = 8 // then a "+N older" row
	AlertRowH   = 24
	// MinAlertRows are kept for alerts before the letters take the rest
	// of the right column: problems to act on beat news.
	MinAlertRows = 3
	// MinLetterRows are kept for the newest letters before the learning
	// helper unfolds.
	MinLetterRows = 2

	BarSlotW = 52 // a colonist bar slot at scale 1
	BarSlotH = 64
	BarGap   = 4

	ResourcesW   = 168
	ResourceRowH = 20
	ResourcesMin = 40 // less room than this hides the list

	MsgRowH   = 22
	MsgStride = MsgRowH + 2
	MsgMaxW   = 480
	MsgMinW   = 160 // narrower than this, the messages hide (a wide window)
	MaxMsgs   = 5

	ArchCatsW = 212 // the Architect's two category columns
	ArchCatsH = 140
	ArchDescW = 320
)

// barScales are the colonist bar's scale steps and the rows each allows.
var barScales = []struct {
	scale float32
	rows  int
}{{1, 1}, {0.75, 2}, {0.5, 3}}

// Inputs is what the layout needs to know about this frame. The owners
// of each region fill their fields (hud.go a.hud()); none of them may call
// a.hud() while doing so.
type Inputs struct {
	Colonists int  // colonist bar entries
	PaneShown bool // the inspect pane is up (a selection, or cell info)
	ArchOpen  bool
	Commands  int // command buttons to lay out

	ExpandedW, ExpandedH float32 // an open inspector tab's wanted size (0: none)
	GlobalH              float32 // the lower-right controls' height
	MinimapOn            bool
	LearnH               float32 // the learning helper's height (0: hidden)
	Alerts, Letters      int
	ResourcesH           float32 // the resource list's content height
	Messages             int
	ArchDescH            float32 // the Architect description box (0: none)
	WindowW, WindowH     float32 // an open main-tab window's wanted size (0: none)
	PerfW, PerfH         float32 // the performance box's size (0: off)
}

// Layout is every region of the colony HUD this frame, in logical
// pixels. A zero gfx.Rect is a region that is not shown.
type Layout struct {
	Tabs                     gfx.Rect
	Pane, PaneTabs, Expanded gfx.Rect
	Grid                     gfx.Rect   // bounding box of the occupied command cells
	GridCells                []gfx.Rect // one per command, bottom row first, left to right
	GridCols                 int        // cells in the bottom row's width
	ArchCats, ArchDesc       gfx.Rect
	Window                   gfx.Rect

	Right                                   gfx.Rect // the right column's band (x and width)
	Global, Minimap, Letters, Learn, Alerts gfx.Rect
	LetterRows, AlertRows                   int // rows that fit (the last may be "+N")

	Bar      gfx.Rect   // bounding box of the colonist bar
	BarSlots []gfx.Rect // one per shown colonist (the last may be "+N")
	BarScale float32
	BarMore  int // colonists not shown (the last slot reads "+N")

	Resources gfx.Rect
	Messages  gfx.Rect
	Perf      gfx.Rect // the performance overlay (perf_overlay.go, PERF-002); empty when off

	HintX, HintBottom float32 // disabled-reason hints and toasts sit above this
}

// Compute places the HUD for a sw x sh logical screen.
func Compute(sw, sh int, st Inputs) Layout {
	W, H := float32(sw), float32(sh)
	var l Layout
	l.Tabs = gfx.Rect{X: 0, Y: H - TabsH, W: W, H: TabsH}
	floorY := l.Tabs.Y // the top of the lowest-left stack, for the resource list

	// the right column
	l.Right = gfx.Rect{X: W - HudMargin - RightW, Y: 0, W: RightW, H: H}
	rx := l.Right.X
	if st.GlobalH > 0 {
		l.Global = gfx.Rect{X: rx, Y: l.Tabs.Y - HudMargin - st.GlobalH, W: RightW, H: st.GlobalH}
	} else {
		l.Global = gfx.Rect{X: rx, Y: l.Tabs.Y - HudMargin, W: RightW, H: 0}
	}
	stackBottom := l.Global.Y
	if st.MinimapOn {
		l.Minimap = gfx.Rect{X: W - HudMargin - MinimapSize, Y: l.Global.Y - HudMargin - MinimapSize, W: MinimapSize, H: MinimapSize}
		stackBottom = l.Minimap.Y
	}
	top := float32(HudMargin)
	alertReserve := float32(0) // kept for alerts, with their margin above the letters
	if st.Alerts > 0 {
		alertReserve = float32(min(st.Alerts, MinAlertRows))*(AlertRowH+HudGap) + HudMargin
	}
	letterReserve := float32(0) // the newest letters, before the learning helper unfolds
	if st.Letters > 0 {
		letterReserve = float32(min(st.Letters, MinLetterRows))*(LetterRowH+HudGap) + HudMargin
	}
	if st.LearnH > 0 {
		h := st.LearnH
		// the least urgent panel: open only if the alerts and the newest
		// letters keep their rows below it (and it never reaches the
		// minimap or the time controls)
		if room := stackBottom - 2*HudMargin - alertReserve - letterReserve; h > room {
			h = min(LearnHeaderH, stackBottom-2*HudMargin) // the header alone
		}
		if h > 0 {
			l.Learn = gfx.Rect{X: rx, Y: HudMargin, W: RightW, H: h}
			top = l.Learn.Bottom() + HudMargin
		}
	}
	if st.Letters > 0 {
		rows := min(st.Letters, MaxLetters)
		if st.Letters > MaxLetters {
			rows++ // "+N older"
		}
		room := int((stackBottom - HudMargin - top - alertReserve + HudGap) / (LetterRowH + HudGap))
		rows = max(0, min(rows, room))
		if rows > 0 {
			h := float32(rows)*(LetterRowH+HudGap) - HudGap
			l.Letters = gfx.Rect{X: rx, Y: stackBottom - HudMargin - h, W: RightW, H: h}
			l.LetterRows = rows
			stackBottom = l.Letters.Y
		}
	}
	if st.Alerts > 0 {
		room := int((stackBottom - HudMargin - top + HudGap) / (AlertRowH + HudGap))
		rows := max(0, min(st.Alerts, room))
		if rows > 0 {
			l.Alerts = gfx.Rect{X: rx, Y: top, W: RightW, H: float32(rows)*(AlertRowH+HudGap) - HudGap}
			l.AlertRows = rows
		}
	}

	// the colonist bar, top-centre, never reaching the resources or the
	// right column
	if st.Colonists > 0 {
		capW := W - 2*(HudMargin+RightW+HudMargin)
		l.BarScale, l.BarSlots, l.BarMore = layoutBar(W, capW, st.Colonists)
		l.Bar = BoundsOf(l.BarSlots)
	}
	barBottom := float32(HudMargin)
	if !l.Bar.Empty() {
		barBottom = l.Bar.Bottom()
	}

	// a main-tab window takes the lower-left above the tabs; the inspect
	// pane and its commands give way to it
	if st.WindowW > 0 && st.WindowH > 0 {
		h := min(st.WindowH, l.Tabs.Y-barBottom-2*HudMargin)
		w := min(st.WindowW, rx-HudMargin)
		l.Window = gfx.Rect{X: 0, Y: l.Tabs.Y - h, W: w, H: h}
		floorY = l.Window.Y
	}

	gridX0 := float32(0)
	switch {
	case st.ArchOpen && l.Window.Empty():
		l.ArchCats = gfx.Rect{X: 0, Y: l.Tabs.Y - ArchCatsH, W: ArchCatsW, H: ArchCatsH}
		floorY = l.ArchCats.Y
		if st.ArchDescH > 0 {
			h := min(st.ArchDescH, l.ArchCats.Y-HudGap-barBottom-HudMargin)
			l.ArchDesc = gfx.Rect{X: 0, Y: l.ArchCats.Y - HudGap - h, W: ArchDescW, H: h}
			floorY = l.ArchDesc.Y
		}
		gridX0 = l.ArchCats.Right() + HudMargin
	case st.PaneShown && l.Window.Empty():
		l.Pane = gfx.Rect{X: 0, Y: l.Tabs.Y - PaneH, W: PaneW, H: PaneH}
		l.PaneTabs = gfx.Rect{X: 0, Y: l.Pane.Y - PaneTabsH, W: PaneW, H: PaneTabsH}
		floorY = l.PaneTabs.Y
		if st.ExpandedW > 0 && st.ExpandedH > 0 {
			w := min(max(st.ExpandedW, ExpandMinW), ExpandMaxW)
			h := min(st.ExpandedH, l.PaneTabs.Y-barBottom-HudMargin)
			l.Expanded = gfx.Rect{X: 0, Y: l.PaneTabs.Y - h, W: w, H: h}
			floorY = l.Expanded.Y
		}
		gridX0 = l.Pane.Right() + HudMargin
	}

	// command buttons: right of the pane (or the Architect's categories),
	// bottom row first, wrapping upward, stopping short of the right column.
	// A row beside an open tab's panel or the Architect's description box
	// (both wider than what is under them) starts right of it, so no cell
	// covers them.
	if st.Commands > 0 && gridX0 > 0 {
		x1 := rx - HudMargin
		bottom := l.Tabs.Y - HudMargin
		maxRows := min(CmdMaxRows, int((bottom-barBottom-HudMargin+HudGap)/CmdStride))
		l.GridCols = max(0, int((x1-gridX0+HudGap)/CmdStride))
		for r := 0; r < maxRows && len(l.GridCells) < st.Commands; r++ {
			y := bottom - CmdCell - float32(r)*CmdStride
			x0 := gridX0
			for _, p := range []gfx.Rect{l.Expanded, l.ArchDesc} {
				if !p.Empty() && y < p.Bottom() && y+CmdCell > p.Y {
					x0 = max(x0, p.Right()+HudMargin)
				}
			}
			for x := x0; x+CmdCell <= x1 && len(l.GridCells) < st.Commands; x += CmdStride {
				l.GridCells = append(l.GridCells, gfx.Rect{X: x, Y: y, W: CmdCell, H: CmdCell})
			}
		}
		l.Grid = BoundsOf(l.GridCells)
	}

	// messages: right-aligned against the right column, under the bar, and
	// clear of whatever the lower-left holds in their rows (an open tab's
	// panel, the Architect's description, a window): they narrow to the room
	// left, and hide while a wide window leaves less than MsgMinW
	if n := min(st.Messages, MaxMsgs); n > 0 {
		band := gfx.Rect{X: 0, Y: barBottom + HudMargin, W: W, H: float32(n)*MsgStride - (MsgStride - MsgRowH)}
		left := float32(HudMargin + ResourcesW + HudMargin)
		for _, r := range append([]gfx.Rect{l.Window, l.ArchCats, l.ArchDesc, l.Pane, l.PaneTabs, l.Expanded}, l.GridCells...) {
			if !r.Empty() && r.Y < band.Bottom() && r.Bottom() > band.Y {
				left = max(left, r.Right()+HudMargin)
			}
		}
		if w := min(float32(MsgMaxW), rx-HudMargin-left); w >= MsgMinW {
			l.Messages = gfx.Rect{X: rx - HudMargin - w, Y: band.Y, W: w, H: band.H}
		}
	}

	// the resource list, top-left, above whatever the lower-left holds
	if st.ResourcesH > 0 {
		h := min(st.ResourcesH, floorY-2*HudMargin-HudMargin)
		if h >= ResourcesMin {
			l.Resources = gfx.Rect{X: HudMargin, Y: HudMargin, W: ResourcesW, H: h}
		}
	}

	l.Perf = PerfRect(l, st.PerfW, st.PerfH)

	// hints and toasts: just above the command buttons or the pane
	l.HintX = max(gridX0, HudMargin)
	if !l.Expanded.Empty() {
		l.HintX = max(l.HintX, l.Expanded.Right()+HudMargin)
	}
	l.HintBottom = l.Tabs.Y - HudMargin
	if !l.Grid.Empty() {
		l.HintBottom = l.Grid.Y - HudMargin
	} else if !l.PaneTabs.Empty() {
		l.HintBottom = l.PaneTabs.Y - HudMargin
	}
	return l
}

// layoutBar lays out n colonist slots centred in a capW-wide band: the
// biggest scale step whose allowed rows hold them all, else the smallest
// step with its last slot reading "+N".
func layoutBar(W, capW float32, n int) (float32, []gfx.Rect, int) {
	for i, s := range barScales {
		pitch := (BarSlotW + BarGap) * s.scale
		perRow := max(1, int((capW+BarGap*s.scale)/pitch))
		last := i == len(barScales)-1
		if n > perRow*s.rows && !last {
			continue
		}
		shown, more := n, 0
		if n > perRow*s.rows {
			shown, more = perRow*s.rows, n-perRow*s.rows+1 // the last slot reads "+N"
		}
		cols := min(shown, perRow)
		rowW := float32(cols)*pitch - BarGap*s.scale
		x0 := float32(math.Floor(float64((W - rowW) / 2)))
		slots := make([]gfx.Rect, shown)
		for j := range shown {
			r, c := j/perRow, j%perRow
			slots[j] = gfx.Rect{X: x0 + float32(c)*pitch, Y: HudMargin + float32(r)*(BarSlotH+BarGap)*s.scale, W: BarSlotW * s.scale, H: BarSlotH * s.scale}
		}
		return s.scale, slots, more
	}
	return 1, nil, 0
}

// BoundsOf is the smallest gfx.Rect around rs (zero for none).
func BoundsOf(rs []gfx.Rect) gfx.Rect {
	if len(rs) == 0 {
		return gfx.Rect{}
	}
	x0, y0, x1, y1 := rs[0].X, rs[0].Y, rs[0].Right(), rs[0].Bottom()
	for _, r := range rs[1:] {
		x0, y0 = min(x0, r.X), min(y0, r.Y)
		x1, y1 = max(x1, r.Right()), max(y1, r.Bottom())
	}
	return gfx.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// Chrome is every region the HUD draws, for blocking map clicks. The
// minimap keeps its own click path and is not in it; empty command cells
// are not either (only occupied cells exist).
func (l Layout) Chrome() []gfx.Rect {
	out := make([]gfx.Rect, 0, 16+len(l.GridCells))
	for _, r := range []gfx.Rect{l.Tabs, l.Pane, l.PaneTabs, l.Expanded, l.ArchCats, l.ArchDesc, l.Window,
		l.Global, l.Letters, l.Learn, l.Alerts, l.Bar, l.Resources, l.Messages, l.Perf} {
		if !r.Empty() {
			out = append(out, r)
		}
	}
	return append(out, l.GridCells...)
}

// LearnHeaderH is the learning helper's collapsed height (its header alone).
const LearnHeaderH = 28

// PerfRect is where the performance box sits: right of the resource list,
// clear of the colonist bar and the messages, never reaching the right
// column or the lower-left stack. Empty when it does not fit.
func PerfRect(l Layout, w, h float32) gfx.Rect {
	if w <= 0 || h <= 0 {
		return gfx.Rect{}
	}
	x := max(l.Resources.Right(), HudMargin+ResourcesW) + HudMargin
	y := float32(HudMargin)
	for _, r := range []gfx.Rect{l.Bar, l.Messages} {
		if !r.Empty() && r.X < x+w && r.Right() > x {
			y = max(y, r.Bottom()+HudMargin)
		}
	}
	bottom := l.Tabs.Y - HudMargin
	for _, r := range []gfx.Rect{l.Window, l.Expanded, l.PaneTabs, l.ArchDesc, l.ArchCats, l.Grid} {
		if !r.Empty() && r.X < x+w {
			bottom = min(bottom, r.Y-HudMargin)
		}
	}
	if x+w > l.Right.X-HudMargin || y+h > bottom {
		return gfx.Rect{}
	}
	return gfx.Rect{X: x, Y: y, W: w, H: h}
}
