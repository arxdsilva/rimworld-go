package ebiten

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/domain/sim/entity"
)

// The inspect pane (PLAY-018: CORE-PANE owns it): a fixed pane at the
// lower-left (header, compact status bars, chips, a scrollable body), its
// tabs in a strip just above it, and a tab's larger panel opening upward
// from there. Each selection kind describes itself as a paneSpec
// (cmds_*.go); this file draws it.
//
// A producer fills a paneSpec from the selection's read models. It must
// not call a.hud(), a.paneRect() or a.expandedRect(): the layout calls the
// producers to size itself. Use paneW and paneInset when text has to fit.
// A spec with an empty Title shows no pane.

// paneBar is one compact status bar ("Health 82%").
type paneBar struct {
	Label string
	Pct   int // 0..100
	Color color.RGBA
}

// paneChip is a small clickable chip on the status row (e.g. the allowed
// area, which opens the area picker).
type paneChip struct {
	ID, Label, Tip string
	Disabled       bool
	Reason         string
}

// paneRow is a clickable line of the scrollable body (a group's members).
// It follows the Lines.
type paneRow struct {
	ID, Label, Tip string
}

// paneTab is one inspector tab. Its body draws into the expanded panel:
// legacy bodies through Draw (on a surface clipped to the panel's inner
// rectangle), widget-kit bodies through Declare (called from paneUI each
// Update). Both receive the inner rectangle: the panel inset by paneInset.
type paneTab struct {
	ID, Label    string // ID "inspector.<tab>"
	Disabled     bool
	Reason       string
	WantW, WantH float32 // the panel size it asks for (clamped by the layout)
	Draw         func(screen *ebiten.Image, r rect)
	Declare      func(r rect)
}

// paneSpec is what the pane shows for the selection.
type paneSpec struct {
	Title, Subtitle string
	Bars            []paneBar
	Chips           []paneChip
	Lines           []string
	Rows            []paneRow
	Tabs            []paneTab
	Close           string // the close button's id ("" none)
}

// Pane metrics (logical px).
const (
	paneInset   = 12 // text inset in the pane and in a tab's panel
	paneTop     = 8
	paneCloseW  = 20
	paneChipH   = 20
	paneChipGap = 4
	paneBarCols = 4
	paneBarGap  = 8
	paneBarFill = 6 // the fill's height under its label
	paneTabGap  = 2
	paneTabPad  = 20 // text padding inside a tab
	paneTabMinW = 48
	paneRowH    = 20

	paneBodyID  = "pane.body"
	paneFrameID = "pane.frame"
	paneExpID   = "pane.expanded"
)

// paneState is this region's presentation state (App.hst).
type paneState struct {
	openTab   string // id of the open tab ("inspector.needs"); "" collapsed
	lastTitle string // the body scrolls back to the top when the title changes

	// testProducer replaces the per-kind pane producers (tests only).
	testProducer func(a *App, k selKind, id entity.EntityID) paneSpec
}

// selectionPane is the pane for the current selection.
func (a *App) selectionPane() paneSpec {
	if a.renderer == nil {
		return paneSpec{}
	}
	selID, hasSel := a.shell.Selected()
	switch {
	case a.grouped():
		return a.groupPane()
	case hasSel:
		return a.paneFor(selPawn, selID)
	case a.hasAnimal:
		return a.paneFor(selAnimal, a.selAnimal)
	case a.hasItem:
		return a.paneFor(selItem, a.selItem)
	case a.hasPlant:
		return a.paneFor(selPlant, a.selPlant)
	case a.hasBuilding:
		return a.paneFor(selBuilding, a.selBuilding)
	case a.hasFrame:
		return a.paneFor(selFrame, a.selFrame)
	case a.hasZone:
		return a.paneFor(selZone, entity.EntityID(a.zoneSel))
	}
	return paneSpec{}
}

// paneFor dispatches to the selection kind's producer.
func (a *App) paneFor(k selKind, id entity.EntityID) paneSpec {
	if p := a.hst.pane.testProducer; p != nil {
		return p(a, k, id)
	}
	switch k {
	case selPawn:
		return a.pawnPane(id)
	case selAnimal:
		return a.animalPane(id)
	case selItem:
		return a.itemPane(id)
	case selPlant:
		return a.plantPane(id)
	case selBuilding:
		return a.buildingPane(id)
	case selFrame:
		return a.framePane(id)
	case selZone:
		return a.zonePane(uint16(id)) // zone_select.go
	}
	return paneSpec{}
}

// shownPane is the selection's pane when the layout shows one: something
// is selected, the Architect and the main-tab windows are closed, and the
// spec has a Title.
func (a *App) shownPane() (paneSpec, bool) {
	if a.renderer == nil || a.archOpen || a.workOpen {
		return paneSpec{}, false
	}
	spec := a.selectionPane()
	return spec, spec.Title != ""
}

// paneShown reports the layout should reserve the pane.
func (a *App) paneShown() bool {
	_, ok := a.shownPane()
	return ok
}

// openTabOf is the open tab of spec, if it exists and is enabled.
func (a *App) openTabOf(spec paneSpec) (paneTab, bool) {
	id := a.hst.pane.openTab
	if id == "" {
		return paneTab{}, false
	}
	for _, t := range spec.Tabs {
		if t.ID == id && !t.Disabled {
			return t, true
		}
	}
	return paneTab{}, false
}

// paneOpenTab is the id of the open tab ("inspector.needs"), "" when the
// pane is collapsed.
func (a *App) paneOpenTab() string { return a.hst.pane.openTab }

// closePaneTab collapses the open tab and reports whether one was open:
// Escape closes it before it clears the selection.
func (a *App) closePaneTab() bool {
	if a.hst.pane.openTab == "" {
		return false
	}
	a.hst.pane.openTab = ""
	a.inspectTab = "character"
	return true
}

// expandedWant is the open tab's wanted panel size (0, 0: none).
func (a *App) expandedWant() (float32, float32) {
	spec, ok := a.shownPane()
	if !ok {
		return 0, 0
	}
	t, ok := a.openTabOf(spec)
	if !ok {
		return 0, 0
	}
	return t.WantW, t.WantH
}

// paneRect and expandedRect are the pane and the open tab's panel.
func (a *App) paneRect() rect     { return a.hud().Pane }
func (a *App) expandedRect() rect { return a.hud().Expanded }

// paneGeom is where the parts of a pane sit, in logical px. The body
// scrolls: lines first, then rows, in content coordinates from the body's
// top.
type paneGeom struct {
	close    rect
	titleW   float32
	bars     []rect // one per shown bar
	chips    []rect // one per shown chip
	body     rect   // the scroll view (empty: no room)
	lines    []string
	rowsY0   float32 // content y of the first row
	contentH float32
}

// paneLayout places spec inside the pane rectangle p. Parts that do not
// fit are dropped rather than drawn over the pane's edge.
func paneLayout(p rect, spec paneSpec) paneGeom {
	var g paneGeom
	inner := p.InsetXY(paneInset, 0)
	limit := p.Bottom() - paneChipGap
	y := p.Y + paneTop
	g.titleW = inner.W
	if spec.Close != "" {
		g.close = rect{X: p.Right() - paneTop - paneCloseW, Y: p.Y + paneTop, W: paneCloseW, H: paneCloseW}
		g.titleW -= paneCloseW + paneTop
	}
	y += lineStep(tsLarge)
	if spec.Subtitle != "" {
		y += lineStep(tsBody)
	}
	y += 6
	if n := len(spec.Bars); n > 0 {
		cols := min(n, paneBarCols)
		w := (inner.W - float32(cols-1)*paneBarGap) / float32(cols)
		h := lineStep(tsSmall) + 2 + paneBarFill
		for i := range n {
			r, c := i/cols, i%cols
			cell := rect{X: inner.X + float32(c)*(w+paneBarGap), Y: y + float32(r)*(h+paneChipGap), W: w, H: h}
			if cell.Bottom() > limit {
				break
			}
			g.bars = append(g.bars, cell)
		}
		y += float32((n+cols-1)/cols) * (h + paneChipGap)
	}
	if len(spec.Chips) > 0 {
		x := inner.X
		for _, c := range spec.Chips {
			w := min(inner.W, textW(c.Label, tsBody)+paneTabPad)
			if x > inner.X && x+w > inner.Right() {
				x, y = inner.X, y+paneChipH+paneChipGap
			}
			if y+paneChipH <= limit {
				g.chips = append(g.chips, rect{X: x, Y: y, W: w, H: paneChipH})
			}
			x += w + paneChipGap
		}
		y += paneChipH + paneChipGap
	}
	body := rect{X: inner.X, Y: y, W: inner.W, H: max(0, limit-y)}
	if body.H < lineStep(tsBody) {
		return g
	}
	g.body = body
	step := lineStep(tsBody)
	for _, l := range spec.Lines {
		g.lines = append(g.lines, wrapPx(l, tsBody, float64(body.W-scrollBarW-4))...)
	}
	g.rowsY0 = float32(len(g.lines)) * step
	if len(spec.Rows) > 0 && len(g.lines) > 0 {
		g.rowsY0 += paneChipGap
	}
	g.contentH = g.rowsY0 + float32(len(spec.Rows))*paneRowH
	return g
}

// paneTabRects lays the tabs out along the strip: widths measured from the
// labels, squeezed together when they do not fit.
func paneTabRects(strip rect, tabs []paneTab) []rect {
	if len(tabs) == 0 {
		return nil
	}
	ws := make([]float32, len(tabs))
	var sum float32
	for i, t := range tabs {
		ws[i] = max(paneTabMinW, textW(t.Label, tsBody)+paneTabPad)
		sum += ws[i]
	}
	room := strip.W - float32(len(tabs)-1)*paneTabGap
	if sum > room && sum > 0 {
		for i := range ws {
			ws[i] *= room / sum
		}
	}
	out := make([]rect, len(tabs))
	x := strip.X
	for i := range tabs {
		out[i] = rect{X: x, Y: strip.Y, W: ws[i], H: strip.H}
		x += ws[i] + paneTabGap
	}
	return out
}

// paneBodyOff is the body's scroll offset.
func (a *App) paneBodyOff() float32 { return a.uiSt().scroll[paneBodyID] }

// paneButtons are the pane's buttons: close, chips, tabs and body rows.
func (a *App) paneButtons() []button {
	spec, ok := a.shownPane()
	if !ok {
		return nil
	}
	l := a.hud()
	if l.Pane.Empty() {
		return nil
	}
	g := paneLayout(l.Pane, spec)
	var out []button
	if spec.Close != "" {
		out = append(out, asButton(g.close, spec.Close, "X"))
	}
	for i, c := range g.chips {
		b := asButton(c, spec.Chips[i].ID, spec.Chips[i].Label)
		b.disabled, b.reason, b.tip, b.quiet = spec.Chips[i].Disabled, spec.Chips[i].Reason, spec.Chips[i].Tip, true
		out = append(out, b)
	}
	out = append(out, a.paneTabButtons(l.PaneTabs, spec)...)
	if !g.body.Empty() {
		off := a.paneBodyOff()
		view := rect{X: g.body.X, Y: g.body.Y, W: g.body.W - scrollBarW, H: g.body.H}
		for i, row := range spec.Rows {
			r := rect{X: view.X, Y: g.body.Y + g.rowsY0 + float32(i)*paneRowH - off, W: view.W, H: paneRowH}.Intersect(view)
			if r.H < paneRowH/2 {
				continue
			}
			b := asButton(r, row.ID, row.Label)
			b.tip = row.Tip
			out = append(out, b)
		}
	}
	return out
}

// paneTabButtons are the tab strip's buttons.
func (a *App) paneTabButtons(strip rect, spec paneSpec) []button {
	rs := paneTabRects(strip, spec.Tabs)
	out := make([]button, len(rs))
	for i, r := range rs {
		t := spec.Tabs[i]
		out[i] = asButton(r, t.ID, t.Label)
		out[i].disabled, out[i].reason, out[i].quiet = t.Disabled, t.Reason, true
		out[i].selected = t.ID == a.hst.pane.openTab && !t.Disabled
	}
	return out
}

// drawPane draws the pane, its tabs and the open tab's panel.
func (a *App) drawPane(screen *ebiten.Image) {
	spec, ok := a.shownPane()
	if !ok {
		return
	}
	l := a.hud()
	if l.Pane.Empty() {
		return
	}
	mx, my := a.currentCursor()
	for _, b := range a.paneTabButtons(l.PaneTabs, spec) {
		drawButton(screen, b, b.contains(mx, my))
	}
	if t, open := a.openTabOf(spec); open && !l.Expanded.Empty() {
		drawPanel(screen, l.Expanded.X, l.Expanded.Y, l.Expanded.W, l.Expanded.H)
		if t.Draw != nil {
			inner := l.Expanded.Inset(paneInset)
			t.Draw(clipTo(screen, inner), inner)
		}
	}

	p := l.Pane
	drawPanel(screen, p.X, p.Y, p.W, p.H)
	g := paneLayout(p, spec)
	x := float64(p.X + paneInset)
	y := float64(p.Y + paneTop)
	drawText(screen, ellipsize(spec.Title, tsLarge, float64(g.titleW)), x, y, withColor(tsLarge, uiHeading))
	y += float64(lineStep(tsLarge))
	if spec.Subtitle != "" {
		drawText(screen, ellipsize(spec.Subtitle, tsBody, float64(g.titleW)), x, y, withColor(tsBody, uiTextDim))
	}
	if spec.Close != "" {
		drawButton(screen, asButton(g.close, spec.Close, "X"), g.close.Hit(mx, my))
	}
	for i, r := range g.bars {
		drawPaneBar(screen, r, spec.Bars[i])
	}
	for i, r := range g.chips {
		c := spec.Chips[i]
		b := asButton(r, c.ID, c.Label)
		b.disabled, b.quiet = c.Disabled, true
		drawButton(screen, b, b.contains(mx, my))
	}
	if g.body.Empty() {
		return
	}
	a.drawPaneBody(screen, spec, g, mx, my)
}

// drawPaneBar draws one compact bar: its label and value over a fill.
func drawPaneBar(screen *ebiten.Image, r rect, b paneBar) {
	pct := max(0, min(100, b.Pct))
	label := ellipsize(fmt.Sprintf("%s %d%%", b.Label, pct), tsSmall, float64(r.W))
	drawText(screen, label, float64(r.X), float64(r.Y), withColor(tsSmall, uiTextDim))
	track := rect{X: r.X, Y: r.Bottom() - paneBarFill, W: r.W, H: paneBarFill}
	fillRect(screen, track, uiField)
	if w := track.W * float32(pct) / 100; w > 0 {
		fillRect(screen, rect{X: track.X, Y: track.Y, W: w, H: track.H}, b.Color)
	}
}

// drawPaneBody draws the scrolled lines and rows, clipped to the body.
func (a *App) drawPaneBody(screen *ebiten.Image, spec paneSpec, g paneGeom, mx, my int) {
	off := a.paneBodyOff()
	view := rect{X: g.body.X, Y: g.body.Y, W: g.body.W - scrollBarW, H: g.body.H}
	sub := clipTo(screen, g.body)
	step := lineStep(tsBody)
	for i, ln := range g.lines {
		ly := g.body.Y + float32(i)*step - off
		if ly+step < g.body.Y || ly > g.body.Bottom() {
			continue
		}
		st := tsBody
		if paneLineWarns(ln) { // cmds_power.go: a power warning
			st = withColor(st, uiWarning)
		}
		drawText(sub, ln, float64(g.body.X), float64(ly), st)
	}
	for i, row := range spec.Rows {
		r := rect{X: view.X, Y: g.body.Y + g.rowsY0 + float32(i)*paneRowH - off, W: view.W, H: paneRowH}
		if r.Bottom() < g.body.Y || r.Y > g.body.Bottom() {
			continue
		}
		if r.Intersect(view).Hit(mx, my) {
			fillRect(sub, r, uiBtnHover)
		}
		drawText(sub, ellipsize(row.Label, tsBody, float64(r.W-8)), float64(r.X+4), float64(r.Y)+(float64(r.H)-lineH(tsBody))/2, tsBody)
	}
}

// selectPaneTab handles a pane tab click ("inspector.<tab>"): a closed
// tab opens, the open tab collapses; a disabled tab shows its reason.
func (a *App) selectPaneTab(id string) {
	name := strings.TrimPrefix(id, ids.PrefixInspector)
	for _, t := range a.selectionPane().Tabs {
		if t.ID != id {
			continue
		}
		if t.Disabled {
			a.hint = t.Reason
			return
		}
		st := &a.hst.pane
		if st.openTab == id {
			a.closePaneTab()
			return
		}
		st.openTab = id
		a.inspectTab = name
		return
	}
}

// openPaneTab opens the selection's tab name ("needs", "log", ...) unless
// it is open already, and reports it is open (false: the pane has no such
// live tab). Captures use it to show a tab.
func (a *App) openPaneTab(name string) bool {
	id := ids.PrefixInspector + name
	if a.hst.pane.openTab != id {
		a.selectPaneTab(id)
	}
	return a.hst.pane.openTab == id
}

// paneUI declares the pane's widget-kit parts: the wheel areas, the body's
// scroll view and the open tab's own widgets.
func (a *App) paneUI(sw, sh int) {
	st := &a.hst.pane
	spec, ok := a.shownPane()
	if !ok {
		st.openTab, st.lastTitle = "", ""
		return
	}
	if _, open := a.openTabOf(spec); !open {
		st.openTab = ""
	}
	l := a.hud()
	if l.Pane.Empty() {
		return
	}
	u := a.uiSt()
	if spec.Title != st.lastTitle {
		st.lastTitle = spec.Title
		if u.scroll != nil {
			u.scroll[paneBodyID] = 0
		}
	}
	u.add(uiWidget{id: paneFrameID, r: l.Pane, wheel: true}) // the wheel never reaches the camera here
	if g := paneLayout(l.Pane, spec); !g.body.Empty() {
		a.uiScrollArea(paneBodyID, g.body, g.contentH, scrollOpts{Step: lineStep(tsBody) * 3})
	}
	if t, open := a.openTabOf(spec); open && !l.Expanded.Empty() {
		u.add(uiWidget{id: paneExpID, r: l.Expanded, wheel: true})
		if t.Declare != nil {
			t.Declare(l.Expanded.Inset(paneInset))
		}
	}
}
