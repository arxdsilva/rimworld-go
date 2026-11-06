package windows

import (
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/research"
)

// The research tree (RSCH-2): the window's state and caches, its geometry,
// the pan and the drawing of the graph. Everything derived from the
// research list (the layout, the node texts, the panel's lines) is rebuilt
// only when World().Research hands out a different list (the game memoises
// it on the research state), so an open window allocates nothing per frame
// for the tree itself.

// Tree geometry, in logical pixels.
const (
	rtNodeW   = 132 // a project's box
	rtNodeH   = 38
	rtColStep = 172 // node width + the gap the lines bend in
	rtRowStep = 50
	rtPad     = 14 // around the graph
	rtHeadH   = 30 // the title row
	rtSearchH = 22 // the search row above the graph
	rtBandH   = 16 // the tech-level names strip
	rtWheel   = 48 // px a wheel notch pans
	rtPanelW  = 290
	rtPanelLo = 150
)

// Widget ids (reported for the id registry).
const (
	researchTreeID   = "research.tree"      // the graph: wheel and drag pan it
	researchSearchID = "research.search"    // the search field
	researchStartID  = "research.start"     // Start research
	researchFinishID = "research.devfinish" // developer mode: finish now
	researchInfoID   = "research.info"      // the info panel's body (wheel scrolls it)
	researchLinkPfx  = "research.prereq."   // + project id: jump to a prerequisite
)

// researchGeom is the window's regions for one window rect.
type researchGeom struct {
	win, panel, body, right, search, band, view gfx.Rect
	start, finish                               gfx.Rect
}

// researchGeometry lays the window out: the info panel on the left, the
// search row, the tech-level strip and the graph on the right; Start
// research (and, in developer mode, Finish now) at the panel's foot.
func researchGeometry(r gfx.Rect, dev bool) researchGeom {
	g := researchGeom{win: r}
	pw := min(rtPanelW, max(rtPanelLo, r.W*0.3))
	g.panel = gfx.Rect{X: r.X + 8, Y: r.Y + rtHeadH, W: pw, H: max(0, r.H-rtHeadH-8)}
	g.start = gfx.Rect{X: g.panel.X + 8, Y: g.panel.Bottom() - 8 - 26, W: max(0, pw-16), H: 26}
	top := g.start.Y
	if dev {
		g.finish = gfx.Rect{X: g.start.X, Y: g.start.Y - 4 - 22, W: g.start.W, H: 22}
		top = g.finish.Y
	}
	g.body = gfx.Rect{X: g.panel.X + 8, Y: g.panel.Y + 8, W: max(0, pw-16), H: max(0, top-6-(g.panel.Y+8))}
	g.right = gfx.Rect{X: g.panel.Right() + 8, Y: g.panel.Y, W: max(0, r.Right()-8-(g.panel.Right()+8)), H: g.panel.H}
	sw := min(200, g.right.W*0.5)
	g.search = gfx.Rect{X: g.right.Right() - sw, Y: g.right.Y, W: sw, H: rtSearchH}
	g.band = gfx.Rect{X: g.right.X, Y: g.right.Y + rtSearchH + 4, W: g.right.W, H: rtBandH}
	g.view = gfx.Rect{X: g.right.X, Y: g.band.Bottom(), W: g.right.W, H: max(0, g.right.Bottom()-g.band.Bottom())}
	return g
}

// rtNode is one project in the tree: its place and its cached texts.
type rtNode struct {
	id    uint8
	idStr string // research.pick.<id>
	level int
	pre   []int  // drawn prerequisites, node indices
	find  string // lower-case label and unlocks, for the search

	v               game.ResearchView // the view the texts were made from
	label, sub, tip string
}

// researchState is the Research window's state (Set.research).
type researchState struct {
	src    listKey[game.ResearchView] // the list the caches came from (identity)
	sig    []uint16                   // id<<8 | tech level per node: the layout's inputs
	nodes  []rtNode
	g      treeGraph
	cw, ch float32 // the graph's content size

	open       bool
	panX, panY float32
	drag       dragState
	view       gfx.Rect // the graph's rect as of the last Declare

	sel   int    // the selected node (-1 none)
	chain []bool // the selected node's prerequisite chain
	stack []int

	query, qlow string
	match       []bool
	matches     int

	tr       lang.Tr                    // the shell, for the cached texts
	benchSrc listKey[game.BuildingView] // the building list hasBench was read from
	hasBench bool

	panel    researchPanel
	btns     []uikit.Button
	startFn  func()
	finishFn func()
}

// listKey is a read-model slice's identity: its first element and length
// (the game hands out the same slice while nothing changed).
type listKey[T any] struct {
	first *T
	n     int
}

// same reports s is the slice the key was taken from, and takes s's key.
func (k *listKey[T]) same(s []T) bool {
	var first *T
	if len(s) > 0 {
		first = &s[0]
	}
	if first == k.first && len(s) == k.n {
		return true
	}
	k.first, k.n = first, len(s)
	return false
}

// dragState is a pan drag under way: the pointer as of the last frame.
type dragState struct {
	on   bool
	x, y int
}

// rs is the Research window's state, made on first use.
func (w *Set) rs() *researchState {
	if w.research == nil {
		w.research = &researchState{sel: -1}
	}
	w.research.tr = w.sh
	return w.research
}

// close forgets the per-opening state: the next opening selects and
// centres afresh. The search text and the pan survive like RimWorld's.
func (st *researchState) close() {
	st.open, st.drag.on = false, false
}

// refresh rebuilds the caches when the research list changed.
func (st *researchState) refresh(views []game.ResearchView) {
	if st.src.same(views) {
		return
	}
	if !st.sameTopology(views) {
		st.rebuild(views)
	}
	for i := range views {
		st.nodes[i].update(st.tr, views[i])
	}
	st.panel.dirty = true
	st.searchAgain()
}

// treeLevel is a project's tech level from the view's word (Neolithic
// when it is unknown).
func treeLevel(v game.ResearchView) int {
	if v.TechLevel != "" {
		for l := research.TechLevel(0); l < research.NumTechLevels; l++ {
			if l.String() == v.TechLevel {
				return int(l)
			}
		}
	}
	return 0
}

// sameTopology reports the views have the projects and levels the layout
// was made for.
func (st *researchState) sameTopology(views []game.ResearchView) bool {
	if len(views) != len(st.sig) {
		return false
	}
	for i, v := range views {
		if st.sig[i] != uint16(v.ID)<<8|uint16(treeLevel(v)) || i < len(st.nodes) && len(st.nodes[i].v.Prereqs) != len(v.Prereqs) {
			return false
		}
	}
	return true
}

// rebuild lays the tree out again for a new set of projects.
func (st *researchState) rebuild(views []game.ResearchView) {
	byID := map[uint8]int{}
	for i, v := range views {
		byID[v.ID] = i
	}
	selID, hadSel := uint8(0), st.sel >= 0 && st.sel < len(st.nodes)
	if hadSel {
		selID = st.nodes[st.sel].id
	}
	st.sig = st.sig[:0]
	st.nodes = make([]rtNode, len(views))
	level := make([]int, len(views))
	pre := make([][]int, len(views))
	for i, v := range views {
		l := treeLevel(v)
		st.sig = append(st.sig, uint16(v.ID)<<8|uint16(l))
		n := rtNode{id: v.ID, idStr: ids.PrefixResearchPick + strconv.Itoa(int(v.ID)), level: l}
		for _, p := range v.Prereqs { // the view's drawn prerequisites (shared: read only)
			if j, ok := byID[uint8(p)]; ok && p >= 0 && p < 256 && j != i {
				n.pre = append(n.pre, j)
			}
		}
		st.nodes[i], level[i], pre[i] = n, l, n.pre
	}
	st.g = layoutTree(level, pre)
	st.cw = 2*rtPad + float32(max(0, st.g.Cols-1))*rtColStep + rtNodeW
	st.ch = 2*rtPad + float32(max(0, st.g.Rows-1))*rtRowStep + rtNodeH
	st.chain = make([]bool, len(views))
	st.match = make([]bool, len(views))
	st.sel = -1
	if j, ok := byID[selID]; ok && hadSel {
		st.sel = j
	}
	st.markChain()
}

// update remakes the node's texts when its view's shown fields changed.
func (n *rtNode) update(tr lang.Tr, v game.ResearchView) {
	o := n.v
	n.v = v
	if n.label != "" && o.Label == v.Label && o.Progress == v.Progress && o.Cost == v.Cost && o.CostNow == v.CostNow &&
		o.Done == v.Done && o.Available == v.Available && o.Current == v.Current && o.Desc == v.Desc {
		return
	}
	n.label = gfx.Ellipsize(v.Label, gfx.Label, rtNodeW-12)
	n.sub = nodeSub(tr, v)
	n.tip = gfx.Ellipsize(v.Desc, gfx.Body, 420)
	n.find = strings.ToLower(v.Label + "\n" + strings.Join(v.Unlocks, "\n"))
}

// researchCost is what a project costs the colony now (the tech-level
// multiplier applied), its table cost before RSCH-1 fills CostNow.
func researchCost(v game.ResearchView) int {
	if v.CostNow > 0 {
		return v.CostNow
	}
	return v.Cost
}

// researchPct is a project's progress in percent (Progress and Cost are
// both at the base cost).
func researchPct(v game.ResearchView) int {
	if v.Cost <= 0 {
		return 0
	}
	return min(100, 100*v.Progress/v.Cost)
}

// researchDone is the work put in at the colony's cost: the "X" of
// "X / Y" (Progress is at the base cost, Y is CostNow).
func researchDone(v game.ResearchView) int {
	if v.Cost <= 0 || v.CostNow <= 0 {
		return v.Progress
	}
	return min(v.Progress, v.Cost) * v.CostNow / v.Cost
}

// nodeSub is a box's second line: its state in words, so no state is
// told by colour alone.
func nodeSub(tr lang.Tr, v game.ResearchView) string {
	switch {
	case v.Done:
		return tr.T("Finished")
	case v.Current:
		return lang.Tf(tr, "Researching %d%%", researchPct(v))
	case !v.Available:
		return lang.Tf(tr, "Locked · %d", researchCost(v))
	case v.Progress > 0:
		return lang.Tf(tr, "%d%% of %d", researchPct(v), researchCost(v))
	}
	return strconv.Itoa(researchCost(v))
}

// select makes node i the selected one (its prerequisite chain lights up
// and the panel shows it).
func (st *researchState) selectNode(i int) {
	if i < -1 || i >= len(st.nodes) {
		return
	}
	if st.sel != i {
		st.panel.off = 0
	}
	st.sel = i
	st.panel.dirty = true
	st.markChain()
}

// markChain marks every project the selected one needs, directly or not.
func (st *researchState) markChain() {
	clear(st.chain)
	if st.sel < 0 || st.sel >= len(st.nodes) {
		return
	}
	st.stack = append(st.stack[:0], st.sel)
	for len(st.stack) > 0 {
		i := st.stack[len(st.stack)-1]
		st.stack = st.stack[:len(st.stack)-1]
		for _, p := range st.nodes[i].pre {
			if !st.chain[p] {
				st.chain[p] = true
				st.stack = append(st.stack, p)
			}
		}
	}
}

// searchAgain re-reads the search text: which boxes match.
func (st *researchState) searchAgain() {
	st.qlow = strings.ToLower(strings.TrimSpace(st.query))
	st.matches = 0
	for i := range st.nodes {
		st.match[i] = st.qlow == "" || strings.Contains(st.nodes[i].find, st.qlow)
		if st.match[i] && st.qlow != "" {
			st.matches++
		}
	}
}

// firstMatch is the first matching node in reading order (-1 none).
func (st *researchState) firstMatch() int {
	best := -1
	for i := range st.nodes {
		if st.qlow == "" || !st.match[i] {
			continue
		}
		if best < 0 || st.g.Col[i] < st.g.Col[best] || st.g.Col[i] == st.g.Col[best] && st.g.Row[i] < st.g.Row[best] {
			best = i
		}
	}
	return best
}

// nodeAt is the box's rectangle on screen for the graph view v.
func (st *researchState) nodeAt(v gfx.Rect, i int) gfx.Rect {
	return gfx.Rect{X: v.X - st.panX + rtPad + float32(st.g.Col[i])*rtColStep, Y: v.Y - st.panY + rtPad + float32(st.g.Row[i])*rtRowStep,
		W: rtNodeW, H: rtNodeH}
}

// hit is the node under the logical point inside the view (-1 none).
func (st *researchState) hit(v gfx.Rect, mx, my int) int {
	if !v.Hit(mx, my) {
		return -1
	}
	for i := range st.nodes {
		if st.nodeAt(v, i).Hit(mx, my) {
			return i
		}
	}
	return -1
}

// clampPan keeps the graph's content over the view.
func (st *researchState) clampPan(v gfx.Rect) {
	st.panX = max(0, min(st.panX, st.cw-v.W))
	st.panY = max(0, min(st.panY, st.ch-v.H))
}

// centerOn pans so node i is in the middle of the view.
func (st *researchState) centerOn(v gfx.Rect, i int) {
	if i < 0 || i >= len(st.nodes) {
		return
	}
	st.panX = rtPad + float32(st.g.Col[i])*rtColStep + rtNodeW/2 - v.W/2
	st.panY = rtPad + float32(st.g.Row[i])*rtRowStep + rtNodeH/2 - v.H/2
	st.clampPan(v)
}

// jump selects node i and brings it into view (a prerequisite link).
func (st *researchState) jump(i int) {
	st.selectNode(i)
	st.centerOn(st.view, i)
}

// opened picks what a fresh opening shows: the project under way, else
// the first one that can be started.
func (st *researchState) opened(v gfx.Rect) {
	st.open = true
	pick := -1
	for i, n := range st.nodes {
		if n.v.Current {
			pick = i
			break
		}
		if pick < 0 && n.v.Available {
			pick = i
		}
	}
	if pick < 0 && st.sel < 0 && len(st.nodes) > 0 {
		pick = 0
	}
	if pick >= 0 {
		st.selectNode(pick)
	}
	st.centerOn(v, st.sel)
}

// pan reads this frame's wheel and drag over the graph: the wheel pans
// (vertically, or sideways with Shift or when the graph is only one view
// tall), a left drag on empty space or a middle drag anywhere moves it.
func (st *researchState) pan(fr *inputframe.Frame, v gfx.Rect, mx, my int, modal bool) {
	if st.drag.on {
		if fr.Down(inputframe.Left) || fr.Down(inputframe.Middle) {
			st.panX -= float32(mx - st.drag.x)
			st.panY -= float32(my - st.drag.y)
			st.drag.x, st.drag.y = mx, my
		} else {
			st.drag.on = false
		}
	} else if !modal && v.Hit(mx, my) {
		middle := fr.Raw.Press[inputframe.Middle]
		if middle || fr.Clicked(inputframe.Left) && st.hit(v, mx, my) < 0 {
			if !middle {
				fr.Take(inputframe.Left)
			}
			st.drag = dragState{on: true, x: mx, y: my}
		}
	}
	if !modal && v.Hit(mx, my) && (fr.Raw.WheelY != 0 || fr.Raw.WheelX != 0) {
		dx, dy := fr.Raw.WheelX, fr.Raw.WheelY
		if fr.Shift() || st.ch <= v.H {
			dx, dy = dx+dy, 0
		}
		st.panX -= float32(dx) * rtWheel
		st.panY -= float32(dy) * rtWheel
		fr.Raw.WheelX, fr.Raw.WheelY = 0, 0
	}
	st.clampPan(v)
}

// Node colours (derived from the palette): done green, available dark,
// unavailable grey; the project under way fills amber as it progresses.
func nodeColors(v game.ResearchView) (fill, text color.RGBA) {
	switch {
	case v.Done:
		return uikit.LerpRGBA(uikit.Good, uikit.Space, 0.62), uikit.Text
	case v.Available:
		return uikit.LerpRGBA(uikit.Space, uikit.Btn, 0.2), uikit.Text
	}
	return uikit.LerpRGBA(uikit.Btn, uikit.PanelBorder, 0.35), uikit.TextDim
}

// drawTree draws the graph: the view's ground, the tech-level strip, the
// prerequisite lines (the selected project's chain on top), the boxes and
// the pan bars. Everything is clipped by hand to the view, so nothing
// lands outside the window.
func (st *researchState) drawTree(dst *ebiten.Image, g researchGeom, mx, my int) {
	v := g.view
	if v.Empty() {
		return
	}
	gfx.FillRect(dst, v.X, v.Y, v.W, v.H, uikit.LerpRGBA(uikit.Field, uikit.Space, 0.4), false)
	gfx.FillRect(dst, g.band.X, g.band.Y, g.band.W, g.band.H, uikit.Field, false)
	for _, b := range st.g.Bands {
		x0 := v.X - st.panX + rtPad + float32(b.Col0)*rtColStep - (rtColStep-rtNodeW)/2
		x1 := v.X - st.panX + rtPad + float32(b.Col1)*rtColStep - (rtColStep-rtNodeW)/2
		if b.Col0 > 0 && x0 > v.X && x0 < v.Right() {
			gfx.FillRect(dst, x0, g.band.Y, 1, v.Bottom()-g.band.Y, uikit.Fade(uikit.PanelBorder, 0x90), false)
		}
		lo, hi := max(x0, v.X), min(x1, v.Right())
		name := research.TechLevel(b.Level).String()
		if tw := gfx.TextW(name, gfx.Small); hi-lo >= tw+8 {
			gfx.Text(dst, name, float64(lo+(hi-lo-tw)/2), float64(g.band.Y)+(rtBandH-gfx.LineH(gfx.Small))/2,
				gfx.WithColor(gfx.Small, uikit.TextDim))
		}
	}
	searching := st.qlow != ""
	for pass := 0; pass < 2; pass++ { // the chain's lines over the others
		for i := range st.nodes {
			for _, p := range st.nodes[i].pre {
				lit := st.sel >= 0 && st.chain[p] && (i == st.sel || st.chain[i])
				if lit != (pass == 1) {
					continue
				}
				c, wd := uikit.Fade(uikit.TextDim, 0x70), float32(1)
				if lit {
					c, wd = uikit.Accent, 2
				} else if searching && !(st.match[i] && st.match[p]) {
					c = uikit.Fade(uikit.TextDim, 0x28)
				}
				a, b := st.nodeAt(v, p), st.nodeAt(v, i)
				x0, y0 := a.Right(), a.Y+a.H/2
				x3, y3 := b.X, b.Y+b.H/2
				bend := float32(rtColStep-rtNodeW) / 2
				clipLine(dst, v, x0, y0, x0+bend, y0, wd, c)
				clipLine(dst, v, x0+bend, y0, x3-bend, y3, wd, c)
				clipLine(dst, v, x3-bend, y3, x3, y3, wd, c)
			}
		}
	}
	hot := st.hit(v, mx, my)
	for i := range st.nodes {
		st.drawNode(dst, v, i, i == hot)
	}
	st.drawPanBars(dst, v)
}

// drawNode draws box i (hover lightens it).
func (st *researchState) drawNode(dst *ebiten.Image, v gfx.Rect, i int, hover bool) {
	n := &st.nodes[i]
	r := st.nodeAt(v, i)
	if r.Intersect(v).Empty() {
		return
	}
	fill, text := nodeColors(n.v)
	if hover {
		fill = uikit.LerpRGBA(fill, uikit.Text, 0.12)
	}
	dim := st.qlow != "" && !st.match[i]
	fade := func(c color.RGBA) color.RGBA {
		if dim {
			return uikit.Fade(c, 0x48)
		}
		return c
	}
	fillClip(dst, r, v, fade(fill))
	if cost := n.v.Cost; !n.v.Done && n.v.Progress > 0 && cost > 0 {
		f := float32(min(n.v.Progress, cost)) / float32(cost)
		bar := gfx.Rect{X: r.X, Y: r.Y, W: r.W * f, H: r.H}
		if !n.v.Current {
			bar = gfx.Rect{X: r.X, Y: r.Bottom() - 3, W: r.W * f, H: 3}
		}
		fillClip(dst, bar, v, fade(uikit.LerpRGBA(uikit.SliderFill, uikit.Space, 0.35)))
	}
	border, bw := uikit.Fade(uikit.PanelBorder, 0xC0), float32(1)
	switch {
	case i == st.sel:
		border, bw = uikit.Heading, 2
	case n.v.Current:
		border, bw = uikit.Accent, 2
	case st.sel >= 0 && st.chain[i]:
		border = uikit.Accent
	}
	strokeClip(dst, r, v, bw, fade(border))
	if r.X < v.X || r.Y < v.Y || r.Right() > v.Right() || r.Bottom() > v.Bottom() {
		return // a box cut by the view's edge shows no text
	}
	gfx.Text(dst, n.label, float64(r.X)+6, float64(r.Y)+3, gfx.WithColor(gfx.Label, fade(text)))
	sub := uikit.TextDim
	if n.v.Done {
		sub = uikit.Good
	}
	gfx.Text(dst, n.sub, float64(r.X)+6, float64(r.Y)+21, gfx.WithColor(gfx.Small, fade(sub)))
}

// drawPanBars draws thin bars on the view's bottom and right edges that
// show where the view is over the graph (only along an axis that pans).
func (st *researchState) drawPanBars(dst *ebiten.Image, v gfx.Rect) {
	const t = 3
	if st.cw > v.W {
		w := max(16, v.W*v.W/st.cw)
		x := v.X + (v.W-w)*st.panX/(st.cw-v.W)
		gfx.FillRect(dst, x, v.Bottom()-t-1, w, t, uikit.Fade(uikit.ScrollThumb, 0xC0), false)
	}
	if st.ch > v.H {
		h := max(16, v.H*v.H/st.ch)
		y := v.Y + (v.H-h)*st.panY/(st.ch-v.H)
		gfx.FillRect(dst, v.Right()-t-1, y, t, h, uikit.Fade(uikit.ScrollThumb, 0xC0), false)
	}
}

// fillClip fills r clipped to clip.
func fillClip(dst *ebiten.Image, r, clip gfx.Rect, c color.Color) {
	if r = r.Intersect(clip); !r.Empty() {
		gfx.FillRect(dst, r.X, r.Y, r.W, r.H, c, false)
	}
}

// strokeClip outlines r inside its edge, w wide, clipped to clip.
func strokeClip(dst *ebiten.Image, r, clip gfx.Rect, w float32, c color.Color) {
	fillClip(dst, gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: w}, clip, c)
	fillClip(dst, gfx.Rect{X: r.X, Y: r.Bottom() - w, W: r.W, H: w}, clip, c)
	fillClip(dst, gfx.Rect{X: r.X, Y: r.Y + w, W: w, H: r.H - 2*w}, clip, c)
	fillClip(dst, gfx.Rect{X: r.Right() - w, Y: r.Y + w, W: w, H: r.H - 2*w}, clip, c)
}

// clipLine draws the segment's part inside clip (Liang-Barsky), kept a
// half width in from its edges.
func clipLine(dst *ebiten.Image, clip gfx.Rect, x0, y0, x1, y1, w float32, c color.Color) {
	in := clip.Inset(w / 2)
	dx, dy := x1-x0, y1-y0
	t0, t1 := float32(0), float32(1)
	for _, e := range [4][2]float32{{-dx, x0 - in.X}, {dx, in.Right() - x0}, {-dy, y0 - in.Y}, {dy, in.Bottom() - y0}} {
		p, q := e[0], e[1]
		if p == 0 {
			if q < 0 {
				return
			}
			continue
		}
		t := q / p
		if p < 0 {
			t0 = max(t0, t)
		} else {
			t1 = min(t1, t)
		}
	}
	if t0 >= t1 {
		return
	}
	gfx.Line(dst, x0+t0*dx, y0+t0*dy, x0+t1*dx, y0+t1*dy, w, c, true)
}
