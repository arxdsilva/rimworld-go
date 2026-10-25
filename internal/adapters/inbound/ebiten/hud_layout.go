package ebiten

import "math"

// The colony HUD's layout (PLAY-018): one pure function places every
// region, so the regions never overlap and every owner draws and hit-tests
// inside its own rectangle. The arrangement follows the familiar colony-
// sim convention (resources top-left, colonist bar top-centre, alerts and
// letters on the right, the inspect pane and its command buttons
// lower-left, time and speed lower-right, the tab strip along the bottom)
// on this project's own grid: logical pixels, multiples of 4, margin 8,
// gap 4. The map fills the whole screen; the HUD floats over it.

const (
	hudMargin = 8
	hudGap    = 4

	tabsH = 28 // the bottom tab strip

	paneW     = 400 // the inspect pane, lower-left
	paneH     = 188
	paneTabsH = 24 // its tabs, just above it

	expandMinW = 400 // an inspector tab's panel, opened above the pane tabs
	expandMaxW = 560

	cmdCell    = 64 // a command button
	cmdStride  = 68 // cell + gap
	cmdMaxRows = 4

	rightW      = 232 // the right column: learning helper, alerts, letters, global controls
	minimapSize = 180
	letterRowH  = 28
	maxLetters  = 8 // then a "+N older" row
	alertRowH   = 24

	barSlotW = 52 // a colonist bar slot at scale 1
	barSlotH = 64
	barGap   = 4

	resourcesW   = 168
	resourceRowH = 20
	resourcesMin = 40 // less room than this hides the list

	msgRowH   = 22
	msgStride = msgRowH + 2
	msgMaxW   = 480
	maxMsgs   = 5

	archCatsW = 212 // the Architect's two category columns
	archCatsH = 140
	archDescW = 320
)

// barScales are the colonist bar's scale steps and the rows each allows.
var barScales = []struct {
	scale float32
	rows  int
}{{1, 1}, {0.75, 2}, {0.5, 3}}

// hudState is what the layout needs to know about this frame. The owners
// of each region fill their fields (hud.go a.hud()); none of them may call
// a.hud() while doing so.
type hudState struct {
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
}

// hudLayout is every region of the colony HUD this frame, in logical
// pixels. A zero rect is a region that is not shown.
type hudLayout struct {
	Tabs                     rect
	Pane, PaneTabs, Expanded rect
	Grid                     rect   // bounding box of the occupied command cells
	GridCells                []rect // one per command, bottom row first, left to right
	GridCols                 int
	ArchCats, ArchDesc       rect
	Window                   rect

	Right                                   rect // the right column's band (x and width)
	Global, Minimap, Letters, Learn, Alerts rect
	LetterRows, AlertRows                   int // rows that fit (the last may be "+N")

	Bar      rect   // bounding box of the colonist bar
	BarSlots []rect // one per shown colonist (the last may be "+N")
	BarScale float32
	BarMore  int // colonists not shown (the last slot reads "+N")

	Resources rect
	Messages  rect

	HintX, HintBottom float32 // disabled-reason hints and toasts sit above this
}

// computeHUDLayout places the HUD for a sw x sh logical screen.
func computeHUDLayout(sw, sh int, st hudState) hudLayout {
	W, H := float32(sw), float32(sh)
	var l hudLayout
	l.Tabs = rect{0, H - tabsH, W, tabsH}
	floorY := l.Tabs.Y // the top of the lowest-left stack, for the resource list

	// the right column
	l.Right = rect{W - hudMargin - rightW, 0, rightW, H}
	rx := l.Right.X
	if st.GlobalH > 0 {
		l.Global = rect{rx, l.Tabs.Y - hudMargin - st.GlobalH, rightW, st.GlobalH}
	} else {
		l.Global = rect{rx, l.Tabs.Y - hudMargin, rightW, 0}
	}
	stackBottom := l.Global.Y
	if st.MinimapOn {
		l.Minimap = rect{W - hudMargin - minimapSize, l.Global.Y - hudMargin - minimapSize, minimapSize, minimapSize}
		stackBottom = l.Minimap.Y
	}
	top := float32(hudMargin)
	if st.LearnH > 0 {
		l.Learn = rect{rx, hudMargin, rightW, st.LearnH}
		top = l.Learn.bottom() + hudMargin
	}
	if st.Letters > 0 {
		rows := min(st.Letters, maxLetters)
		if st.Letters > maxLetters {
			rows++ // "+N older"
		}
		room := int((stackBottom - hudMargin - top + hudGap) / (letterRowH + hudGap))
		rows = max(0, min(rows, room))
		if rows > 0 {
			h := float32(rows)*(letterRowH+hudGap) - hudGap
			l.Letters = rect{rx, stackBottom - hudMargin - h, rightW, h}
			l.LetterRows = rows
			stackBottom = l.Letters.Y
		}
	}
	if st.Alerts > 0 {
		room := int((stackBottom - hudMargin - top + hudGap) / (alertRowH + hudGap))
		rows := max(0, min(st.Alerts, room))
		if rows > 0 {
			l.Alerts = rect{rx, top, rightW, float32(rows)*(alertRowH+hudGap) - hudGap}
			l.AlertRows = rows
		}
	}

	// the colonist bar, top-centre, never reaching the resources or the
	// right column
	if st.Colonists > 0 {
		capW := W - 2*(hudMargin+rightW+hudMargin)
		l.BarScale, l.BarSlots, l.BarMore = layoutBar(W, capW, st.Colonists)
		l.Bar = boundsOf(l.BarSlots)
	}
	barBottom := float32(hudMargin)
	if !l.Bar.empty() {
		barBottom = l.Bar.bottom()
	}

	// messages: right-aligned against the right column, under the bar
	if n := min(st.Messages, maxMsgs); n > 0 {
		w := min(float32(msgMaxW), rx-hudMargin-(hudMargin+resourcesW+hudMargin))
		l.Messages = rect{rx - hudMargin - w, barBottom + hudMargin, w, float32(n)*msgStride - (msgStride - msgRowH)}
	}

	// a main-tab window takes the lower-left above the tabs; the inspect
	// pane and its commands give way to it
	if st.WindowW > 0 && st.WindowH > 0 {
		h := min(st.WindowH, l.Tabs.Y-barBottom-2*hudMargin)
		w := min(st.WindowW, rx-hudMargin)
		l.Window = rect{0, l.Tabs.Y - h, w, h}
		floorY = l.Window.Y
	}

	gridX0 := float32(0)
	switch {
	case st.ArchOpen && l.Window.empty():
		l.ArchCats = rect{0, l.Tabs.Y - archCatsH, archCatsW, archCatsH}
		floorY = l.ArchCats.Y
		if st.ArchDescH > 0 {
			h := min(st.ArchDescH, l.ArchCats.Y-hudGap-barBottom-hudMargin)
			l.ArchDesc = rect{0, l.ArchCats.Y - hudGap - h, archDescW, h}
			floorY = l.ArchDesc.Y
		}
		gridX0 = l.ArchCats.right() + hudMargin
	case st.PaneShown && l.Window.empty():
		l.Pane = rect{0, l.Tabs.Y - paneH, paneW, paneH}
		l.PaneTabs = rect{0, l.Pane.Y - paneTabsH, paneW, paneTabsH}
		floorY = l.PaneTabs.Y
		if st.ExpandedW > 0 && st.ExpandedH > 0 {
			w := min(max(st.ExpandedW, expandMinW), expandMaxW)
			h := min(st.ExpandedH, l.PaneTabs.Y-barBottom-hudMargin)
			l.Expanded = rect{0, l.PaneTabs.Y - h, w, h}
			floorY = l.Expanded.Y
		}
		gridX0 = l.Pane.right() + hudMargin
	}

	// command buttons: right of the pane (or the Architect's categories),
	// bottom row first, wrapping upward, stopping short of the right column
	if st.Commands > 0 && gridX0 > 0 {
		x1 := rx - hudMargin
		cols := int((x1 - gridX0 + hudGap) / cmdStride)
		bottom := l.Tabs.Y - hudMargin
		maxRows := min(cmdMaxRows, int((bottom-barBottom-hudMargin+hudGap)/cmdStride))
		if cols > 0 && maxRows > 0 {
			n := min(st.Commands, cols*maxRows)
			l.GridCols = cols
			l.GridCells = make([]rect, n)
			for i := range n {
				r, c := i/cols, i%cols
				l.GridCells[i] = rect{gridX0 + float32(c)*cmdStride, bottom - cmdCell - float32(r)*cmdStride, cmdCell, cmdCell}
			}
			l.Grid = boundsOf(l.GridCells)
		}
	}

	// the resource list, top-left, above whatever the lower-left holds
	if st.ResourcesH > 0 {
		h := min(st.ResourcesH, floorY-2*hudMargin-hudMargin)
		if h >= resourcesMin {
			l.Resources = rect{hudMargin, hudMargin, resourcesW, h}
		}
	}

	// hints and toasts: just above the command buttons or the pane
	l.HintX = max(gridX0, hudMargin)
	if !l.Expanded.empty() {
		l.HintX = max(l.HintX, l.Expanded.right()+hudMargin)
	}
	l.HintBottom = l.Tabs.Y - hudMargin
	if !l.Grid.empty() {
		l.HintBottom = l.Grid.Y - hudMargin
	} else if !l.PaneTabs.empty() {
		l.HintBottom = l.PaneTabs.Y - hudMargin
	}
	return l
}

// layoutBar lays out n colonist slots centred in a capW-wide band: the
// biggest scale step whose allowed rows hold them all, else the smallest
// step with its last slot reading "+N".
func layoutBar(W, capW float32, n int) (float32, []rect, int) {
	for i, s := range barScales {
		pitch := (barSlotW + barGap) * s.scale
		perRow := max(1, int((capW+barGap*s.scale)/pitch))
		last := i == len(barScales)-1
		if n > perRow*s.rows && !last {
			continue
		}
		shown, more := n, 0
		if n > perRow*s.rows {
			shown, more = perRow*s.rows, n-perRow*s.rows+1 // the last slot reads "+N"
		}
		cols := min(shown, perRow)
		rowW := float32(cols)*pitch - barGap*s.scale
		x0 := float32(math.Floor(float64((W - rowW) / 2)))
		slots := make([]rect, shown)
		for j := range shown {
			r, c := j/perRow, j%perRow
			slots[j] = rect{x0 + float32(c)*pitch, hudMargin + float32(r)*(barSlotH+barGap)*s.scale, barSlotW * s.scale, barSlotH * s.scale}
		}
		return s.scale, slots, more
	}
	return 1, nil, 0
}

// boundsOf is the smallest rect around rs (zero for none).
func boundsOf(rs []rect) rect {
	if len(rs) == 0 {
		return rect{}
	}
	x0, y0, x1, y1 := rs[0].X, rs[0].Y, rs[0].right(), rs[0].bottom()
	for _, r := range rs[1:] {
		x0, y0 = min(x0, r.X), min(y0, r.Y)
		x1, y1 = max(x1, r.right()), max(y1, r.bottom())
	}
	return rect{x0, y0, x1 - x0, y1 - y0}
}

// chrome is every region the HUD draws, for blocking map clicks. The
// minimap keeps its own click path and is not in it; empty command cells
// are not either (only occupied cells exist).
func (l hudLayout) chrome() []rect {
	out := make([]rect, 0, 16+len(l.GridCells))
	for _, r := range []rect{l.Tabs, l.Pane, l.PaneTabs, l.Expanded, l.ArchCats, l.ArchDesc, l.Window,
		l.Global, l.Letters, l.Learn, l.Alerts, l.Bar, l.Resources, l.Messages} {
		if !r.empty() {
			out = append(out, r)
		}
	}
	return append(out, l.GridCells...)
}
