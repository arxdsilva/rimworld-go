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
	PaneTabsH = 24  // its tabs, just above it
	CardMinH  = 100 // the floating card never shrinks below this

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

	// MaxBarGroups caps the colonist bar's map groups (MAP-1): the most
	// open maps. Inputs keeps them in an array so it stays comparable.
	MaxBarGroups = 8
	barGroupPad  = 3  // a group frame's inset around its slots, at scale 1
	barChipH     = 14 // a group's label chip above its slots
	barGroupGap  = 8  // between two group frames

	ResourcesW   = 168
	ResourceRowH = 20
	ResourcesMin = 40 // less room than this hides the list

	MsgRowH   = 22
	MsgStride = MsgRowH + 2
	MsgMaxW   = 480
	MsgMinW   = 160 // narrower than this, the messages hide (a wide window)
	MaxMsgs   = 5

	ArchCatsW = 212 // the Architect's two category columns
	ArchCatsH = 216 // seven rows of two category buttons (12 vanilla categories and storage, STK-AG, NST): 2*8 pad + 7*26 + 6*3
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
	Colonists int // colonist bar entries
	// BarGroups are the slots of each map's colonists in bar order, zero
	// after the last group. All zero (a single map) draws the bar as one
	// plain run of cards, with no frames; otherwise Colonists is their sum.
	BarGroups [MaxBarGroups]int
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
	// The colonist card (RHUD), zero keeps today's layout. PaneW and PaneH
	// size the pane; Card is the floating card; letters avoid LetterAvoid and
	// use LetterGap between them (0: HudGap).
	PaneW, PaneH float32
	Card         gfx.Rect
	LetterAvoid  gfx.Rect
	LetterGap    float32
}

// BarGroup is one map's frame on the colonist bar: its label chip on top
// and the N slots from BarSlots[First] inside.
type BarGroup struct {
	Frame, Chip gfx.Rect
	First, N    int
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
	LetterRows, AlertRows                   int     // rows that fit (the last may be "+N")
	LetterGap                               float32 // the padding between letter rows (0: none shown)

	Bar      gfx.Rect   // bounding box of the colonist bar
	BarSlots []gfx.Rect // one per shown colonist (the last may be "+N")
	BarScale float32
	BarMore  int // colonists not shown (the last slot reads "+N")
	// BarGroups are the map frames of a bar with several maps (empty with
	// one); the groups' slots are consecutive runs of BarSlots.
	BarGroups []BarGroup

	Resources gfx.Rect
	Messages  gfx.Rect
	Perf      gfx.Rect // the performance overlay (perf_overlay.go, PERF-002); empty when off
	Card      gfx.Rect // the floating colonist card; empty when docked or off

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
		g := float32(HudGap) // RHUD-475: the padding between letters
		if st.LetterGap > 0 {
			g = st.LetterGap
		}
		rows := min(st.Letters, MaxLetters)
		if st.Letters > MaxLetters {
			rows++ // "+N older"
		}
		lo, hi := top, stackBottom-HudMargin // the band the letters may use
		if a := st.LetterAvoid; !a.Empty() && a.Right() > rx && a.X < rx+RightW && a.Bottom() > lo && a.Y < hi {
			// RHUD-471, 472: a floating card over the column. The letters take
			// the larger free band, below the card (kept at the bottom) or above.
			fit := func(lo, hi float32) int { return max(0, int((hi-lo+g)/(LetterRowH+g))) }
			if below := fit(a.Bottom()+HudMargin, hi); below >= fit(lo, a.Y-HudMargin) {
				lo = a.Bottom() + HudMargin
			} else {
				hi = a.Y - HudMargin
			}
		}
		floor := lo
		if lo == top {
			floor += alertReserve // the alerts keep their rows above
		}
		room := int((hi - floor + g) / (LetterRowH + g))
		rows = max(0, min(rows, room))
		if rows > 0 {
			h := float32(rows)*(LetterRowH+g) - g
			l.Letters = gfx.Rect{X: rx, Y: hi - h, W: RightW, H: h}
			l.LetterRows = rows
			l.LetterGap = g
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
		if st.BarGroups[0] > 0 {
			l.BarScale, l.BarSlots, l.BarMore, l.BarGroups = layoutBarGroups(W, capW, st.BarGroups)
			l.Bar = BoundsOf(frames(l.BarGroups))
		} else {
			l.BarScale, l.BarSlots, l.BarMore = layoutBar(W, capW, st.Colonists)
			l.Bar = BoundsOf(l.BarSlots)
		}
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
		pw, ph := paneSize(st, rx, l.Tabs.Y-barBottom)
		l.Pane = gfx.Rect{X: 0, Y: l.Tabs.Y - ph, W: pw, H: ph}
		l.PaneTabs = gfx.Rect{X: 0, Y: l.Pane.Y - PaneTabsH, W: pw, H: PaneTabsH}
		floorY = l.PaneTabs.Y
		l.Card = placeCard(st.Card, l, W)
		if st.ExpandedW > 0 && st.ExpandedH > 0 {
			l.Expanded = placeExpanded(st, l, barBottom, rx)
			if l.Expanded.X == 0 {
				floorY = l.Expanded.Y
			}
		}
		if !l.Card.Empty() && l.Card.X < HudMargin+ResourcesW+HudMargin {
			floorY = min(floorY, l.Card.Y)
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
			for _, p := range []gfx.Rect{l.Expanded, l.ArchDesc, l.Card} {
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
		for _, r := range append([]gfx.Rect{l.Window, l.ArchCats, l.ArchDesc, l.Pane, l.PaneTabs, l.Expanded, l.Card}, l.GridCells...) {
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

// paneSize is the pane's size: today's constants unless the card asks for
// others, never past the right column (rx) or the room above the tab strip
// (room, measured from the colonist bar's bottom to the strip).
func paneSize(st Inputs, rx, room float32) (w, h float32) {
	w, h = PaneW, PaneH
	if st.PaneW > 0 {
		w = min(st.PaneW, rx-HudMargin)
	}
	if st.PaneH > 0 {
		h = max(PaneH, min(st.PaneH, room-HudMargin-PaneTabsH))
	}
	return w, h
}

// placeExpanded is an open tab's panel: above the pane's tabs. Under a tall
// card pane with too little room there, it stands beside the pane; over a
// floating card, beside the card.
func placeExpanded(st Inputs, l Layout, barBottom, rx float32) gfx.Rect {
	w := min(max(st.ExpandedW, ExpandMinW), ExpandMaxW)
	room := l.PaneTabs.Y - barBottom - HudMargin
	r := gfx.Rect{X: 0, Y: l.PaneTabs.Y - min(st.ExpandedH, room), W: w, H: min(st.ExpandedH, room)}
	if st.PaneH > 0 && room < st.ExpandedH {
		h := min(st.ExpandedH, l.Tabs.Y-barBottom-HudMargin)
		r = gfx.Rect{X: l.Pane.Right() + HudGap, Y: l.Tabs.Y - h, W: w, H: h}
	}
	if !l.Card.Empty() && r.Y < l.Card.Bottom() && r.Bottom() > l.Card.Y && r.X < l.Card.Right() && r.Right() > l.Card.X {
		r.X = l.Card.Right() + HudGap
	}
	r.W = min(r.W, rx-HudMargin-r.X) // beside the pane it can reach the right column
	return r
}

// placeCard keeps the floating card on the screen and over the tab strip,
// and lifts it clear of the pane and its tabs when they would share a column,
// shrinking it when the room above them is short. A smaller window never
// loses the card.
func placeCard(c gfx.Rect, l Layout, W float32) gfx.Rect {
	if c.Empty() {
		return gfx.Rect{}
	}
	r := fitCard(c, W, l.Tabs.Y)
	if r.X < l.Pane.Right() && r.Bottom() > l.PaneTabs.Y {
		r.H = max(CardMinH, min(r.H, l.PaneTabs.Y-HudGap-HudMargin))
		r.Y = max(0, l.PaneTabs.Y-HudGap-r.H)
	}
	return r
}

// fitCard cuts r to W x floor, moving it in before it shrinks.
func fitCard(r gfx.Rect, W, floor float32) gfx.Rect {
	r.W, r.H = min(r.W, W), min(r.H, floor)
	r.X = max(0, min(r.X, W-r.W))
	r.Y = max(0, min(r.Y, floor-r.H))
	return r
}

// cardBase is where a w x h card sits at an anchor (0..8, row by row from the
// top left) with no offset: HudMargin off the window's edges, the bottom row
// resting over the tab strip.
func cardBase(sw, sh, anchor, w, h int) (x, y int) {
	floor := sh - TabsH
	switch anchor % 3 {
	case 0:
		x = HudMargin
	case 1:
		x = (sw - w) / 2
	default:
		x = sw - w - HudMargin
	}
	switch anchor / 3 {
	case 0:
		y = HudMargin
	case 1:
		y = (floor - h) / 2
	default:
		y = floor - h - HudMargin
	}
	return x, y
}

// CardRect is the floating card at an anchor and offset, held on the screen.
func CardRect(sw, sh, anchor, offX, offY, w, h int) gfx.Rect {
	x, y := cardBase(sw, sh, anchor, w, h)
	r := gfx.Rect{X: float32(x + offX), Y: float32(y + offY), W: float32(w), H: float32(h)}
	return fitCard(r, float32(sw), float32(sh-TabsH))
}

// CardOffsetBounds are the offsets that keep a w x h card at an anchor wholly
// on the screen and over the tab strip. A card too big for the screen gets the
// one offset that holds its top left corner on it.
func CardOffsetBounds(sw, sh, anchor, w, h int) (minX, maxX, minY, maxY int) {
	x, y := cardBase(sw, sh, anchor, w, h)
	minX, maxX = -x, max(-x, sw-w-x)
	minY, maxY = -y, max(-y, sh-TabsH-h-y)
	return minX, maxX, minY, maxY
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

func frames(gs []BarGroup) []gfx.Rect {
	out := make([]gfx.Rect, len(gs))
	for i, g := range gs {
		out[i] = g.Frame
	}
	return out
}

// layoutBarGroups lays the bar out as one framed block per map (MAP-1),
// left to right, centred: the biggest scale step whose allowed rows hold
// every group, else the smallest step with the last slot reading "+N".
func layoutBarGroups(W, capW float32, sizes [MaxBarGroups]int) (float32, []gfx.Rect, int, []BarGroup) {
	var ns []int
	total := 0
	for _, n := range sizes {
		if n <= 0 {
			break
		}
		ns = append(ns, n)
		total += n
	}
	for i, s := range barScales {
		if slots, groups, ok := fitGroups(W, capW, s.scale, s.rows, ns); ok {
			return s.scale, slots, 0, groups
		} else if i == len(barScales)-1 {
			for keep := total - 1; keep >= 1; keep-- {
				if slots, groups, ok = fitGroups(W, capW, s.scale, s.rows, truncate(ns, keep)); ok {
					return s.scale, slots, total - keep + 1, groups
				}
			}
			return s.scale, slots, total, groups
		}
	}
	return 1, nil, 0, nil
}

// truncate is ns cut to keep slots in all.
func truncate(ns []int, keep int) []int {
	var out []int
	for _, n := range ns {
		if keep <= 0 {
			break
		}
		out = append(out, min(n, keep))
		keep -= n
	}
	return out
}

// fitGroups places the groups at one scale step with rows rows each; ok
// when they fit capW.
func fitGroups(W, capW, scale float32, rows int, ns []int) ([]gfx.Rect, []BarGroup, bool) {
	pitch := (BarSlotW + BarGap) * scale
	pad, gap := barGroupPad*scale, barGroupGap*scale
	chip := barChipH * scale
	widths := make([]float32, len(ns))
	var sum float32
	for i, n := range ns {
		cols := (n + rows - 1) / rows
		widths[i] = float32(cols)*pitch - BarGap*scale + 2*pad
		sum += widths[i]
	}
	sum += float32(len(ns)-1) * gap
	x := float32(math.Floor(float64((W - sum) / 2)))
	var slots []gfx.Rect
	groups := make([]BarGroup, len(ns))
	for i, n := range ns {
		cols := (n + rows - 1) / rows
		g := BarGroup{First: len(slots), N: n}
		g.Frame = gfx.Rect{X: x, Y: HudMargin, W: widths[i], H: chip + pad + float32(min(rows, (n+cols-1)/cols))*(BarSlotH+BarGap)*scale - BarGap*scale + pad}
		g.Chip = gfx.Rect{X: g.Frame.X, Y: g.Frame.Y, W: g.Frame.W, H: chip}
		for j := range n {
			r, c := j/cols, j%cols
			slots = append(slots, gfx.Rect{X: x + pad + float32(c)*pitch, Y: HudMargin + chip + pad + float32(r)*(BarSlotH+BarGap)*scale, W: BarSlotW * scale, H: BarSlotH * scale})
		}
		groups[i] = g
		x += widths[i] + gap
	}
	return slots, groups, sum <= capW
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
		l.Global, l.Letters, l.Learn, l.Alerts, l.Bar, l.Resources, l.Messages, l.Perf, l.Card} {
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
