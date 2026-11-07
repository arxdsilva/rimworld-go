package inspector

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/entity"
)

// The inspect pane (PLAY-018: CORE-PANE owns it): a fixed pane at the
// lower-left (header, compact status bars, chips, a scrollable body), its
// tabs in a strip just above it, and a tab's larger panel opening upward
// from there. Each selection kind describes itself as a paneSpec
// (cmds_*.go); this file draws it.
//
// A producer fills a paneSpec from the selection's read models. It must
// not call a.shell.Layout(), a.paneRect() or a.expandedRect(): the layout calls the
// producers to size itself. Use paneW and paneInset when text has to fit.
// A spec with an empty Title shows no pane.

// paneBar is one compact status bar ("Health 82%").
type Bar struct {
	Label string
	Pct   int // 0..100
	Color color.RGBA
}

// paneChip is a small clickable chip on the status row (e.g. the allowed
// area, which opens the area picker).
type Chip struct {
	ID, Label, Tip string
	Disabled       bool
	Reason         string
}

// paneRow is a clickable line of the scrollable body (a group's members).
// It follows the Lines.
type Row struct {
	ID, Label, Tip string
}

// paneTab is one inspector tab. Its body draws into the expanded panel:
// legacy bodies through Draw (on a surface clipped to the panel's inner
// rectangle), widget-kit bodies through Declare (called from paneUI each
// Update). Both receive the inner rectangle: the panel inset by paneInset.
type Tab struct {
	ID, Label    string // ID "inspector.<tab>"
	Disabled     bool
	Reason       string
	WantW, WantH float32 // the panel size it asks for (clamped by the layout)
	Draw         func(screen *ebiten.Image, r gfx.Rect)
	Declare      func(r gfx.Rect)
}

// paneSpec is what the pane shows for the selection.
type Spec struct {
	Title, Subtitle string
	Bars            []Bar
	Chips           []Chip
	Lines           []string
	Rows            []Row
	Tabs            []Tab
	Close           string // the close button's id ("" none)

	// The colonist card's restyled pane (pane_body.go, RHUD); zero everywhere
	// else, and then the pane is the plain one.
	Restyled           bool            // the header row with its buttons
	Card               bool            // the card is the body
	LogBody            bool            // the character log is the body
	Kind               selection.Kind  // what the header acts on
	ID                 entity.EntityID //
	TabW               float32         // fixed tabs, right-aligned
	TitleColor         color.RGBA      // the name's tint
	TitleTip, TitleTab string          // its tooltip and the tab a click toggles
	Emblem, EmblemTip  string          // the faction icon and its tooltip
}

// Pane metrics (logical px): the reference sizes are hudlayout's (UIS-044 to
// UIS-048), the rest ours.
const (
	PaneInset   = hudlayout.PaneInset // text inset in the pane and in a tab's panel
	paneCloseW  = 20
	paneChipH   = 20
	paneChipGap = 4
	// paneNameGap is the name's baseline to the card's body, paneCardBottom
	// the edge under it: with hudlayout.PaneNameBaseline they are the 32 px
	// dossier/sheet.DockMargin counts.
	paneNameGap    = 2
	paneCardBottom = 2
	paneBarCols    = 4
	paneBarLabelX  = 3 // the bar's label inset (UIS-047)
	paneTabGap     = hudlayout.PaneTabGap
	paneTabPad     = 20 // text padding inside a chip
	PaneRowH       = hudlayout.PaneLineH
	// notoAscent is the UI font's ascent per pixel of size (Noto Sans: 1069
	// units of 1000), to put a baseline where a reference size says.
	notoAscent = 1.069

	PaneBodyID  = "pane.body"
	paneFrameID = "pane.frame"
	paneExpID   = "pane.expanded"
)

// selectionPane is the pane for the current selection.
func (a *Pane) SelectionPane() Spec {
	if a.sim() == nil {
		return Spec{}
	}
	selID, hasSel := a.shell.Selected()
	switch {
	case a.sel.Grouped():
		return a.Commands().GroupPane()
	case hasSel:
		return a.paneFor(selection.Pawn, selID)
	case a.sel.HasAnimal:
		return a.paneFor(selection.Animal, a.sel.SelAnimal)
	case a.sel.HasItem:
		return a.paneFor(selection.Item, a.sel.SelItem)
	case a.sel.HasPlant:
		return a.paneFor(selection.Plant, a.sel.SelPlant)
	case a.sel.HasBuilding:
		return a.paneFor(selection.Building, a.sel.SelBuilding)
	case a.sel.HasFrame:
		return a.paneFor(selection.Frame, a.sel.SelFrame)
	case a.sel.HasZone:
		return a.paneFor(selection.Zone, entity.EntityID(a.sel.ZoneSel))
	}
	return Spec{}
}

// paneFor dispatches to the selection kind's producer.
func (a *Pane) paneFor(k selection.Kind, id entity.EntityID) Spec {
	if p := a.TestPane; p != nil {
		return withKind(p(k, id), k)
	}
	if p := a.Producers[k].Pane; p != nil {
		return a.Card().dress(k, id, withKind(p(id), k))
	}
	return Spec{}
}

// withKind is spec of a selection of kind k (a producer may leave it out;
// a pawn's tabs fill the strip, UIS-045).
func withKind(spec Spec, k selection.Kind) Spec {
	if spec.Kind == selection.None {
		spec.Kind = k
	}
	return spec
}

// shownPane is the selection's pane when the layout shows one: something
// is selected, the Architect and the main-tab windows are closed, and the
// spec has a Title.
func (a *Pane) shownPane() (Spec, bool) {
	if a.sim() == nil || a.env().ArchOpen || a.env().WinOpen {
		return Spec{}, false
	}
	spec := a.SelectionPane()
	return spec, spec.Title != ""
}

// paneShown reports the layout should reserve the pane.
func (a *Pane) PaneShown() bool {
	_, ok := a.shownPane()
	return ok
}

// openTabOf is the open tab of spec, if it exists and is enabled.
func (a *Pane) openTabOf(spec Spec) (Tab, bool) {
	id := a.openTab
	if id == "" {
		return Tab{}, false
	}
	for _, t := range spec.Tabs {
		if t.ID == id && !t.Disabled {
			return t, true
		}
	}
	return Tab{}, false
}

// paneOpenTab is the id of the open tab ("inspector.needs"), "" when the
// pane is collapsed.
func (a *Pane) PaneOpenTab() string { return a.openTab }

// closePaneTab collapses the open tab and reports whether one was open:
// Escape closes it before it clears the selection.
func (a *Pane) ClosePaneTab() bool {
	if a.openTab == "" {
		return false
	}
	a.openTab = ""
	a.Tab = "character"
	return true
}

// expandedWant is the open tab's wanted panel size (0, 0: none).
func (a *Pane) ExpandedWant() (float32, float32) {
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
func (a *Pane) PaneRect() gfx.Rect     { return a.shell.Layout().Pane }
func (a *Pane) ExpandedRect() gfx.Rect { return a.shell.Layout().Expanded }

// Geom is where the parts of a pane sit, in logical px. The body
// scrolls: lines first, then rows, in content coordinates from the body's
// top.
type Geom struct {
	Close    gfx.Rect
	TitleW   float32
	Bars     []gfx.Rect // one per shown bar
	Chips    []gfx.Rect // one per shown chip
	Body     gfx.Rect   // the scroll view (empty: no room)
	Card     gfx.Rect   // the card's box, on a pane whose body it is
	Lines    []string
	RowsY0   float32 // content y of the first row
	ContentH float32
}

// TextTop is the top y to draw st at so its baseline lands on baseline (a
// reference size gives baselines, gfx.Text takes the line's top).
func TextTop(baseline float32, st gfx.TextStyle) float32 {
	return baseline - float32(st.Size*notoAscent)
}

// headY is the header buttons' top: 20 px centred on the name's caps.
func headY(p gfx.Rect) float32 { return p.Y + hudlayout.PaneNameBaseline - headBtn + 2 }

// paneLayout places spec inside the pane rectangle p: the name with its
// baseline PaneNameBaseline below the top (UIS-046), the status bars from
// PaneBarTop (UIS-047), then the chips and the body's lines at PaneLineH
// (UIS-048). Parts that do not fit are dropped rather than drawn over the
// pane's edge.
func PaneLayout(p gfx.Rect, spec Spec) Geom {
	var g Geom
	inner := p.InsetXY(PaneInset, 0)
	limit := p.Bottom() - paneChipGap
	g.TitleW = inner.W
	if spec.Close != "" {
		g.Close = gfx.Rect{X: p.Right() - PaneInset - paneCloseW, Y: headY(p), W: paneCloseW, H: paneCloseW}
		g.TitleW -= paneCloseW + headGap
	}
	if spec.Restyled {
		g.TitleW -= headReserve
		if spec.Emblem != "" {
			g.TitleW -= headEmblem
		}
	}
	y := p.Y + hudlayout.PaneNameBaseline + paneNameGap // under the name
	if spec.Card {
		// the compact card (RHUD-003) starts right under the name and keeps
		// a thin edge below
		limit = p.Bottom() - paneCardBottom
		g.Card = gfx.Rect{X: inner.X, Y: y, W: inner.W, H: max(0, limit-y)}
		return g
	}
	if spec.Subtitle != "" {
		y += gfx.LineStep(gfx.Body) + paneChipGap
	}
	y = max(y, p.Y+hudlayout.PaneBarTop)
	if n := len(spec.Bars); n > 0 {
		cols := min(n, paneBarCols)
		w := min(hudlayout.PaneBarW, (inner.W-float32(cols-1)*hudlayout.PaneBarGap)/float32(cols))
		h := float32(hudlayout.PaneBarH)
		for i := range n {
			r, c := i/cols, i%cols
			cell := gfx.Rect{X: inner.X + float32(c)*(w+hudlayout.PaneBarGap), Y: y + float32(r)*(h+hudlayout.PaneBarGap), W: w, H: h}
			if cell.Bottom() > limit {
				break
			}
			g.Bars = append(g.Bars, cell)
		}
		y += float32((n+cols-1)/cols) * (h + hudlayout.PaneBarGap)
	}
	if len(spec.Chips) > 0 {
		x := inner.X
		for _, c := range spec.Chips {
			w := min(inner.W, gfx.TextW(c.Label, gfx.Body)+paneTabPad)
			if x > inner.X && x+w > inner.Right() {
				x, y = inner.X, y+paneChipH+paneChipGap
			}
			if y+paneChipH <= limit {
				g.Chips = append(g.Chips, gfx.Rect{X: x, Y: y, W: w, H: paneChipH})
			}
			x += w + paneChipGap
		}
		y += paneChipH + paneChipGap
	}
	body := gfx.Rect{X: inner.X, Y: y, W: inner.W, H: max(0, limit-y)}
	if body.H < gfx.LineStep(gfx.Body) {
		return g
	}
	g.Body = body
	step := gfx.LineStep(gfx.Body)
	for _, l := range spec.Lines {
		g.Lines = append(g.Lines, gfx.WrapPx(l, gfx.Body, float64(body.W-uikit.ScrollBarW-4))...)
	}
	g.RowsY0 = float32(len(g.Lines)) * step
	if len(spec.Rows) > 0 && len(g.Lines) > 0 {
		g.RowsY0 += paneChipGap
	}
	g.ContentH = g.RowsY0 + float32(len(spec.Rows))*PaneRowH
	return g
}

// paneTabRects lays spec's tabs along the strip (UIS-045): the card's own
// width when it sets one, a pawn's tabs in equal widths across the pane,
// any other kind's PaneTabW wide against the strip's right edge.
func paneTabRects(strip gfx.Rect, spec Spec) []gfx.Rect {
	n := len(spec.Tabs)
	switch {
	case spec.TabW > 0:
		return fixedTabRects(strip, n, spec.TabW)
	case spec.Kind == selection.Pawn && n > 0:
		return fixedTabRects(strip, n, (strip.W+paneTabGap)/float32(n))
	}
	return fixedTabRects(strip, n, hudlayout.PaneTabW+paneTabGap)
}

// fixedTabRects lays n tabs at a pitch of w (a tab and its gap) out against
// the strip's right edge, first tab leftmost, so the order stays the same
// (RHUD-005, 007); they shrink to share a strip too narrow for them.
func fixedTabRects(strip gfx.Rect, n int, w float32) []gfx.Rect {
	if n == 0 {
		return nil
	}
	w = min(w, (strip.W+paneTabGap)/float32(n))
	out := make([]gfx.Rect, n)
	x := max(strip.X, strip.Right()-float32(n)*w+paneTabGap) // a full strip starts at its left edge, not a rounding error left of it
	for i := range out {
		out[i] = gfx.Rect{X: x, Y: strip.Y, W: w - paneTabGap, H: strip.H}
		x += w
	}
	return out
}

// paneBodyOff is the body's scroll offset.
func (a *Pane) paneBodyOff() float32 { return a.kit().Scroll[PaneBodyID] }

// paneButtons are the pane's buttons: close, chips, tabs and body rows.
func (a *Pane) PaneButtons() []uikit.Button {
	spec, ok := a.shownPane()
	if !ok {
		return nil
	}
	l := a.shell.Layout()
	if l.Pane.Empty() {
		return nil
	}
	g := PaneLayout(l.Pane, spec)
	var out []uikit.Button
	if spec.Close != "" {
		out = append(out, uikit.AsButton(g.Close, spec.Close, "X"))
	}
	for i, c := range g.Chips {
		b := uikit.AsButton(c, spec.Chips[i].ID, spec.Chips[i].Label)
		b.Disabled, b.Reason, b.Tip, b.Quiet = spec.Chips[i].Disabled, spec.Chips[i].Reason, spec.Chips[i].Tip, true
		out = append(out, b)
	}
	out = append(out, a.paneTabButtons(l.PaneTabs, spec)...)
	if !g.Body.Empty() {
		off := a.paneBodyOff()
		view := gfx.Rect{X: g.Body.X, Y: g.Body.Y, W: g.Body.W - uikit.ScrollBarW, H: g.Body.H}
		for i, row := range spec.Rows {
			r := gfx.Rect{X: view.X, Y: g.Body.Y + g.RowsY0 + float32(i)*PaneRowH - off, W: view.W, H: PaneRowH}.Intersect(view)
			if r.H < PaneRowH/2 {
				continue
			}
			b := uikit.AsButton(r, row.ID, row.Label)
			b.Tip = row.Tip
			out = append(out, b)
		}
	}
	return out
}

// paneTabButtons are the tab strip's buttons.
func (a *Pane) paneTabButtons(strip gfx.Rect, spec Spec) []uikit.Button {
	rs := paneTabRects(strip, spec)
	out := make([]uikit.Button, len(rs))
	for i, r := range rs {
		t := spec.Tabs[i]
		out[i] = uikit.AsButton(r, t.ID, t.Label)
		out[i].Disabled, out[i].Reason, out[i].Quiet = t.Disabled, t.Reason, true
		out[i].Selected = t.ID == a.openTab && !t.Disabled
	}
	return out
}

// drawPane draws the pane, its tabs and the open tab's panel.
func (a *Pane) DrawPane(screen *ebiten.Image) {
	spec, ok := a.shownPane()
	if !ok {
		return
	}
	l := a.shell.Layout()
	if l.Pane.Empty() {
		return
	}
	mx, my := a.shell.Cursor()
	for _, b := range a.paneTabButtons(l.PaneTabs, spec) {
		uikit.DrawButton(screen, b, b.Contains(mx, my), a.shell)
	}
	if t, open := a.openTabOf(spec); open && !l.Expanded.Empty() {
		uikit.DrawPanel(screen, l.Expanded.X, l.Expanded.Y, l.Expanded.W, l.Expanded.H)
		if t.Draw != nil {
			inner := l.Expanded.Inset(PaneInset)
			t.Draw(gfx.ClipTo(screen, inner), inner)
		}
	}

	p := l.Pane
	uikit.DrawPanel(screen, p.X, p.Y, p.W, p.H)
	g := PaneLayout(p, spec)
	x := float64(p.X + PaneInset)
	titleC := uikit.Heading
	if spec.TitleColor.A != 0 {
		titleC = spec.TitleColor
	}
	if spec.Emblem != "" {
		if img := a.Card().headIcon(spec.Emblem); img != nil {
			gfx.Icon(screen, img, x, float64(headY(p)), headBtn)
		}
		x += headEmblem
	}
	base := p.Y + hudlayout.PaneNameBaseline
	gfx.Text(screen, gfx.Ellipsize(spec.Title, gfx.Title, float64(g.TitleW)), x, float64(TextTop(base, gfx.Title)), gfx.WithColor(gfx.Title, titleC))
	if spec.Subtitle != "" {
		gfx.Text(screen, gfx.Ellipsize(spec.Subtitle, gfx.Body, float64(g.TitleW)), x, float64(base+paneNameGap), gfx.WithColor(gfx.Body, uikit.TextDim))
	}
	if spec.Close != "" {
		uikit.DrawButton(screen, uikit.AsButton(g.Close, spec.Close, "X"), g.Close.Hit(mx, my), a.shell)
	}
	for i, r := range g.Bars {
		drawPaneBar(screen, r, spec.Bars[i])
	}
	for i, r := range g.Chips {
		c := spec.Chips[i]
		b := uikit.AsButton(r, c.ID, c.Label)
		b.Disabled, b.Quiet = c.Disabled, true
		uikit.DrawButton(screen, b, b.Contains(mx, my), a.shell)
	}
	if a.Card().drawBody(screen, spec, g) || g.Body.Empty() {
		return
	}
	a.drawPaneBody(screen, spec, g, mx, my)
}

// drawPaneBar draws one compact bar (UIS-047): the fill, and its label and
// value inside it.
func drawPaneBar(screen *ebiten.Image, r gfx.Rect, b Bar) {
	pct := max(0, min(100, b.Pct))
	uikit.FillRect(screen, r, uikit.Field)
	if w := r.W * float32(pct) / 100; w > 0 {
		uikit.FillRect(screen, gfx.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, uikit.LerpRGBA(uikit.Field, b.Color, 0.7))
	}
	label := gfx.Ellipsize(fmt.Sprintf("%s %d%%", b.Label, pct), gfx.Small, float64(r.W-2*paneBarLabelX))
	gfx.Text(screen, label, float64(r.X+paneBarLabelX), gfx.CenterTop(float64(r.Y), float64(r.H), gfx.Small), gfx.Small)
}

// drawPaneBody draws the scrolled lines and rows, clipped to the body.
func (a *Pane) drawPaneBody(screen *ebiten.Image, spec Spec, g Geom, mx, my int) {
	off := a.paneBodyOff()
	view := gfx.Rect{X: g.Body.X, Y: g.Body.Y, W: g.Body.W - uikit.ScrollBarW, H: g.Body.H}
	sub := gfx.ClipTo(screen, g.Body)
	step := gfx.LineStep(gfx.Body)
	for i, ln := range g.Lines {
		ly := g.Body.Y + float32(i)*step - off
		if ly+step < g.Body.Y || ly > g.Body.Bottom() {
			continue
		}
		st := gfx.Body
		if PaneLineWarns(ln) { // cmds_power.go: a power warning
			st = gfx.WithColor(st, uikit.Warning)
		}
		gfx.Text(sub, ln, float64(g.Body.X), float64(ly), st)
	}
	for i, row := range spec.Rows {
		r := gfx.Rect{X: view.X, Y: g.Body.Y + g.RowsY0 + float32(i)*PaneRowH - off, W: view.W, H: PaneRowH}
		if r.Bottom() < g.Body.Y || r.Y > g.Body.Bottom() {
			continue
		}
		if r.Intersect(view).Hit(mx, my) {
			uikit.FillRect(sub, r, uikit.BtnHover)
		}
		gfx.Text(sub, gfx.Ellipsize(row.Label, gfx.Body, float64(r.W-8)), float64(r.X+4), gfx.CenterTop(float64(r.Y), float64(r.H), gfx.Body), gfx.Body)
	}
}

// selectPaneTab handles a pane tab click ("inspector.<tab>"): a closed
// tab opens, the open tab collapses; a disabled tab shows its reason.
func (a *Pane) SelectPaneTab(id string) {
	name := strings.TrimPrefix(id, ids.PrefixInspector)
	for _, t := range a.SelectionPane().Tabs {
		if t.ID != id {
			continue
		}
		if t.Disabled {
			a.shell.Hint(t.Reason)
			return
		}
		st := a
		if st.openTab == id {
			a.ClosePaneTab()
			return
		}
		st.openTab = id
		a.Tab = name
		return
	}
}

// openPaneTab opens the selection's tab name ("needs", "log", ...) unless
// it is open already, and reports it is open (false: the pane has no such
// live tab). Captures use it to show a tab.
func (a *Pane) OpenPaneTab(name string) bool {
	id := ids.PrefixInspector + name
	if a.openTab != id {
		a.SelectPaneTab(id)
	}
	return a.openTab == id
}

// paneUI declares the pane's widget-kit parts: the wheel areas, the body's
// scroll view and the open tab's own widgets.
func (a *Pane) PaneUI(sw, sh int) {
	st := a
	spec, ok := a.shownPane()
	if !ok {
		st.openTab, st.lastTitle = "", ""
		return
	}
	if _, open := a.openTabOf(spec); !open {
		st.openTab = ""
	}
	l := a.shell.Layout()
	if l.Pane.Empty() {
		return
	}
	u := a.kit()
	if spec.Title != st.lastTitle {
		st.lastTitle = spec.Title
		if u.Scroll != nil {
			u.Scroll[PaneBodyID] = 0
		}
	}
	u.Add(uikit.Widget{ID: paneFrameID, R: l.Pane, Wheel: true}) // the wheel never reaches the camera here
	g := PaneLayout(l.Pane, spec)
	if spec.Restyled {
		a.Card().declareHeader(l.Pane, spec, g)
	}
	switch {
	case spec.Card:
		a.Card().declareBody(spec, g)
	case !spec.LogBody && !g.Body.Empty():
		a.kit().ScrollArea(PaneBodyID, g.Body, g.ContentH, uikit.ScrollOpts{Step: gfx.LineStep(gfx.Body) * 3})
	}
	if t, open := a.openTabOf(spec); open && !l.Expanded.Empty() {
		u.Add(uikit.Widget{ID: paneExpID, R: l.Expanded, Wheel: true})
		if t.Declare != nil {
			t.Declare(l.Expanded.Inset(PaneInset))
		}
	}
}
