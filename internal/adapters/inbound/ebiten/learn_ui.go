package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The learning helper (PLAY-018: LEARN owns it): a collapsible top-right
// panel in the layout's Learn rect, in the warm tutor colours. Expanded it
// lists the next three unfinished lessons (lessons.go); clicking one opens
// its text in a detail card left of the panel, with a "Got it" button.
// Progress (Gameplay.LearnedLessons), the collapse and the on/off switch live
// in the settings and are saved through setSettings.
//
// Lessons are done by what the player does: commands arrive through
// Renderer.onEnqueue (hooked on the first Update of a colony), UI events are
// read each Update (learnUI): the selection and the speed from App state, the
// camera from the player's pan and zoom input. The panel hides itself while the
// setting is off and once every lesson is done; Options › Gameplay turns it
// back on and resets the lessons.

const (
	learnHeaderH  = 28
	learnRowH     = 44
	learnRowGap   = 4
	learnFooterH  = 20
	learnPanelPad = 4
	learnShown    = 3 // lessons listed at once

	learnRowW   = rightW - 2*learnPanelPad // a lesson row
	learnRowPad = 10                       // text inset in a row (after its accent bar)

	learnCardW   = 280 // the detail card, at most
	learnCardPad = 12
	learnGotItW  = 96
	learnGotItH  = 28

	learnPanFrames   = 24  // frames of pan input (0.4 s held) that finish the camera lesson
	learnZoomSteps   = 2   // so do this many wheel notches or zoom keys
	learnFlashFrames = 150 // how long a lesson done on its own is highlighted
)

// learnState is this region's presentation state (App.hst).
type learnState struct {
	hooked *Renderer // the renderer onEnqueue is installed on
	memo   learnMemo // earlier commands, for two-step lessons

	// what the UI-event lessons compare against, taken when the helper
	// starts looking (a new colony, the helper turned on, a reset)
	based      bool
	lastSel    string
	lastSpeed  speed
	panFrames  float64 // camera: frames of pan input so far
	zoomSteps  float64 // camera: wheel notches and zoom keys so far
	nLearned   int     // lessons done at the last Update: a drop is a reset
	open       string  // the lesson whose card is up
	flashID    string  // the lesson just done on its own
	flashUntil uint64  // App.updates when its highlight ends
}

// ------------------------------------------------------------ progress

// learnCount is the number of lessons not done yet.
func (a *App) learnCount() int {
	n := 0
	for i := range lessonCatalog {
		if !slices.Contains(a.settings.Gameplay.LearnedLessons, lessonCatalog[i].id) {
			n++
		}
	}
	return n
}

// learnLeft is the lessons not done yet, in order.
func (a *App) learnLeft() []*lesson {
	out := make([]*lesson, 0, len(lessonCatalog))
	for i := range lessonCatalog {
		if !slices.Contains(a.settings.Gameplay.LearnedLessons, lessonCatalog[i].id) {
			out = append(out, &lessonCatalog[i])
		}
	}
	return out
}

// learnVisible reports the helper is on screen: a colony runs, the setting
// is on and a lesson is left.
func (a *App) learnVisible() bool {
	return a.renderer != nil && a.settings.Gameplay.ShowLearningHelper && a.learnCount() > 0
}

// learnTracking reports the player's actions count: the helper is visible
// and this is not a scripted capture.
func (a *App) learnTracking() bool {
	return a.cfg.Capture == nil && a.learnVisible()
}

// learnSave stores the helper's progress or fold through the settings save
// hook, as setSettings does. It does not re-apply the settings (setSettings
// rebuilds the key bindings and the scale and pushes every option into the
// renderer): progress changes nothing of that, and it happens in the middle
// of play.
func (a *App) learnSave(s config.Settings) {
	a.settings = s.Normalize()
	if a.cfg.Settings.Save != nil {
		if err := a.cfg.Settings.Save(a.settings); err != nil {
			a.hint = "settings not saved: " + err.Error()
		}
	}
}

// learnComplete marks a lesson done and saves it. flash highlights it
// briefly (a lesson done on its own; "Got it" is the player's own word).
func (a *App) learnComplete(id string, flash bool) {
	if lessonByID(id) == nil || slices.Contains(a.settings.Gameplay.LearnedLessons, id) {
		return
	}
	s := a.settings
	s.Gameplay.LearnedLessons = append(slices.Clone(s.Gameplay.LearnedLessons), id)
	a.learnSave(s)
	st := &a.hst.learn
	st.nLearned = len(a.settings.Gameplay.LearnedLessons)
	if st.open == id {
		st.open = ""
	}
	if flash {
		st.flashID, st.flashUntil = id, a.updates+learnFlashFrames
	}
}

// learnFlash is the title of the lesson just done on its own, while it is
// highlighted.
func (a *App) learnFlash() (string, bool) {
	st := &a.hst.learn
	if st.flashID == "" || a.updates >= st.flashUntil {
		return "", false
	}
	if l := lessonByID(st.flashID); l != nil {
		return l.title, true
	}
	return "", false
}

// learnObserve sees every command the player sends (Renderer.onEnqueue).
func (a *App) learnObserve(c command.Command) {
	if !a.learnTracking() {
		return
	}
	m := &a.hst.learn.memo
	for _, l := range a.learnLeft() {
		if l.onCmd != nil && l.onCmd(m, c) {
			a.learnComplete(l.id, true)
		}
	}
}

// learnHook installs learnObserve on the colony's renderer, once per
// renderer, after whatever observer is already there.
func (a *App) learnHook() {
	st := &a.hst.learn
	r := a.renderer
	if r == nil || st.hooked == r {
		return
	}
	st.hooked = r
	prev := r.onEnqueue
	r.onEnqueue = func(c command.Command) {
		if prev != nil {
			prev(c)
		}
		a.learnObserve(c)
	}
	st.rebase()
}

// rebase makes the helper start counting from now.
func (st *learnState) rebase() {
	st.based, st.memo, st.panFrames, st.zoomSteps = false, learnMemo{}, 0, 0
}

// learnSelection names what is selected ("" for nothing) so a change of
// selection can be told from a stale one.
func (a *App) learnSelection() string {
	r := a.renderer
	switch {
	case a.grouped():
		return fmt.Sprintf("g%d:%v", a.group.kind, a.group.ids)
	case r.HasSelect():
		return fmt.Sprintf("p%d", r.Selected())
	case a.hasBuilding:
		return fmt.Sprintf("b%d", a.selBuilding)
	case a.hasAnimal:
		return fmt.Sprintf("a%d", a.selAnimal)
	case a.hasPlant:
		return fmt.Sprintf("l%d", a.selPlant)
	case a.hasItem:
		return fmt.Sprintf("i%d", a.selItem)
	case a.hasFrame:
		return fmt.Sprintf("f%d", a.selFrame)
	}
	return ""
}

// learnUI runs once per Update on the colony (the widget-kit hook): it
// hooks the renderer, notices a reset and reads the UI events the lessons
// finish on.
func (a *App) learnUI(sw, sh int) {
	st := &a.hst.learn
	if a.renderer == nil {
		return
	}
	a.learnHook()
	if n := len(a.settings.Gameplay.LearnedLessons); n < st.nLearned {
		st.rebase() // lessons were reset: count from now
	}
	st.nLearned = len(a.settings.Gameplay.LearnedLessons)
	if !a.learnTracking() {
		st.rebase()
		st.open = ""
		return
	}
	a.learnWatch()
}

// learnCameraInput is this Update's camera input: whether a pan control is
// in use (a pan key held, a middle-button drag, edge scrolling) and how many
// zoom steps were taken (wheel notches, zoom keys). The lesson reads the
// player's input, not the camera, so a window resize, a UI scale change or a
// jump to an alert never counts as a pan.
func (a *App) learnCameraInput() (pan, zoom float64) {
	f := &a.in
	if f.top() == input.CtxText {
		return 0, 0
	}
	if f.held(input.CamLeft) || f.held(input.CamRight) || f.held(input.CamUp) || f.held(input.CamDown) || f.down(mbMiddle) {
		pan = 1
	}
	if a.renderer.edgeScroll && f.raw.focused {
		mx, my := a.mapCursor()
		if ex, ey := edgePan(mx, my, a.mapViewport()); ex != 0 || ey != 0 {
			pan = 1
		}
	}
	if f.fired(input.ZoomIn) || f.fired(input.ZoomOut) {
		zoom++
	}
	if lx, ly := a.currentCursor(); f.raw.wheelY != 0 && !a.uiWheelOver(lx, ly) { // a scroll view under the cursor owns the wheel
		zoom += math.Abs(f.raw.wheelY)
	}
	return pan, zoom
}

// learnWatch compares the camera input, the selection and the speed with
// what the helper saw before and finishes the lessons that ask for a change.
func (a *App) learnWatch() {
	st := &a.hst.learn
	r := a.renderer
	if !st.based {
		st.lastSel, st.lastSpeed, st.based = a.learnSelection(), r.speed, true
		return
	}
	pan, zoom := a.learnCameraInput()
	st.panFrames += pan
	st.zoomSteps += zoom
	var f learnFrame
	f.panned = st.panFrames >= learnPanFrames || st.zoomSteps >= learnZoomSteps
	sel := a.learnSelection()
	f.selected = sel != "" && sel != st.lastSel
	st.lastSel = sel
	// the game pausing itself on a threat is not the player's doing
	f.speedSet = r.speed != st.lastSpeed && !(r.speed == speedPaused && r.threatPaused)
	st.lastSpeed = r.speed
	for _, l := range a.learnLeft() {
		if l.onUI != nil && l.onUI(f) {
			a.learnComplete(l.id, true)
		}
	}
}

// ------------------------------------------------------------ layout

// learnHeight is the helper's height the layout reserves: 0 hidden, the
// header alone collapsed, else the header, up to three lessons and the
// footer. It must not call a.hud().
func (a *App) learnHeight() float32 {
	if !a.learnVisible() {
		return 0
	}
	if a.settings.Gameplay.LearnCollapsed {
		return learnHeaderH
	}
	n := min(learnShown, a.learnCount())
	return learnHeaderH + learnPanelPad + float32(n)*(learnRowH+learnRowGap) + learnFooterH
}

// learnRow is a listed lesson and its rect.
type learnRow struct {
	l *lesson
	r rect
}

// learnLayout is where the helper's parts are this frame.
type learnLayout struct {
	panel, header, footer rect
	rows                  []learnRow
	open                  *lesson // the lesson whose card is up (nil: none)
	card, gotIt           rect
	body, goal            []string // the card's wrapped text
}

// learnFolded reports the helper shows its header alone: the player folded
// it, or the layout's Learn rect p has no room for the lessons (a short
// screen with the minimap up).
func (a *App) learnFolded(p rect) bool {
	return a.settings.Gameplay.LearnCollapsed || p.H < a.learnHeight()-0.01
}

// learnLayout places the panel's parts inside the layout's Learn rect and
// the detail card left of it.
func (a *App) learnLayout() learnLayout {
	var lay learnLayout
	if a.renderer == nil {
		return lay
	}
	h := a.hud()
	p := h.Learn
	if p.empty() {
		return lay
	}
	lay.panel = p
	lay.header = rect{p.X, p.Y, p.W, min(learnHeaderH, p.H)}
	if a.learnFolded(p) {
		return lay
	}
	lay.footer = rect{p.X, p.bottom() - learnFooterH, p.W, learnFooterH}
	left := a.learnLeft()
	var openRow rect
	for i := 0; i < min(learnShown, len(left)); i++ {
		r := rect{p.X + learnPanelPad, p.Y + learnHeaderH + learnPanelPad + float32(i)*(learnRowH+learnRowGap), learnRowW, learnRowH}
		lay.rows = append(lay.rows, learnRow{left[i], r})
		if left[i].id == a.hst.learn.open {
			lay.open, openRow = left[i], r
		}
	}
	if lay.open != nil {
		a.learnCardLayout(&lay, h, openRow)
	}
	return lay
}

// learnCardLayout places the detail card of lay.open left of the panel,
// level with its row, clear of the colonist bar and the tab strip, and
// wraps its text.
func (a *App) learnCardLayout(lay *learnLayout, h hudLayout, row rect) {
	p := lay.panel
	w := min(float32(learnCardW), p.X-hudGap-hudMargin)
	if w < 2*learnCardPad+80 {
		lay.open = nil // no room beside the panel
		return
	}
	inner := float64(w - 2*learnCardPad)
	lay.body = wrapPx(lay.open.body(a), tsBody, inner)
	lay.goal = wrapPx("Try it: "+lay.open.goal, tsSmall, inner)
	hgt := learnCardHeight(lay)
	maxH := h.Tabs.Y - 2*hudMargin
	for hgt > maxH && len(lay.body) > 1 { // a very short screen: cut the text
		lay.body = lay.body[:len(lay.body)-1]
		last := &lay.body[len(lay.body)-1]
		*last = strings.TrimRight(*last, " .,;:") + "…"
		hgt = learnCardHeight(lay)
	}
	x := p.X - hudGap - w
	y := row.Y
	if !h.Bar.empty() && x < h.Bar.right() && x+w > h.Bar.X {
		y = max(y, h.Bar.bottom()+hudGap)
	}
	y = max(min(y, h.Tabs.Y-hudMargin-hgt), hudMargin)
	lay.card = rect{x, y, w, hgt}
	lay.gotIt = rect{x + w - learnCardPad - learnGotItW, y + hgt - learnCardPad - learnGotItH, learnGotItW, learnGotItH}
}

// learnCardHeight is the card's height for its wrapped text.
func learnCardHeight(lay *learnLayout) float32 {
	return learnCardPad + lineStep(tsLabel) + 6 + float32(len(lay.body))*lineStep(tsBody) +
		8 + float32(len(lay.goal))*lineStep(tsSmall) + 10 + learnGotItH + learnCardPad
}

// learnButtons are the helper's buttons, topmost first: the card's Got it,
// the lesson rows, the header toggle.
func (a *App) learnButtons() []button {
	lay := a.learnLayout()
	if lay.panel.empty() {
		return nil
	}
	var out []button
	if lay.open != nil && !lay.card.empty() {
		b := lay.gotIt.asButton("learn.done."+lay.open.id, "Got it")
		b.tip = "Mark this lesson learned"
		out = append(out, b)
	}
	for _, rw := range lay.rows {
		b := rw.r.asButton("learn.lesson."+rw.l.id, rw.l.title)
		b.selected = lay.open == rw.l
		b.tip = "Read this lesson"
		out = append(out, b)
	}
	t := lay.header.asButton("learn.toggle", "Learning helper")
	t.tip = "Fold or unfold the lessons (Options › Gameplay turns them off)"
	return append(out, t)
}

// learnChrome is the helper's area beyond the layout's Learn rect: the
// detail card, so a click on it never reaches the map.
func (a *App) learnChrome() []rect {
	lay := a.learnLayout()
	if lay.open == nil || lay.card.empty() {
		return nil
	}
	return []rect{lay.card}
}

// applyLearnClick handles a "learn." click.
func (a *App) applyLearnClick(id string) {
	st := &a.hst.learn
	switch {
	case id == "learn.toggle":
		s := a.settings
		s.Gameplay.LearnCollapsed = !s.Gameplay.LearnCollapsed
		st.open = ""
		a.learnSave(s)
	case strings.HasPrefix(id, "learn.lesson."):
		lid := strings.TrimPrefix(id, "learn.lesson.")
		switch {
		case lessonByID(lid) == nil || slices.Contains(a.settings.Gameplay.LearnedLessons, lid):
		case st.open == lid:
			st.open = "" // the open lesson again: close its card
		default:
			st.open = lid
		}
	case strings.HasPrefix(id, "learn.done."):
		a.learnComplete(strings.TrimPrefix(id, "learn.done."), false)
	}
}

// closeLearnCard closes the detail card and reports there was one up (the
// colony's Escape handling can take it before anything else).
func (a *App) closeLearnCard() bool {
	shown := len(a.learnChrome()) > 0
	a.hst.learn.open = ""
	return shown
}

// ------------------------------------------------------------ drawing

// learnMix blends a toward b by t (0: a, 1: b), keeping a's alpha.
func learnMix(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-t) + float64(y)*t)) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), a.A}
}

// drawLearn draws the helper: the panel, its lessons and the detail card.
func (a *App) drawLearn(screen *ebiten.Image) {
	lay := a.learnLayout()
	p := lay.panel
	if p.empty() {
		return
	}
	mx, my := a.currentCursor()
	flashTitle, flashing := a.learnFlash()
	pulse := 1.0
	if flashing && !a.settings.Accessibility.ReducedFlashing {
		pulse = 0.65 + 0.35*math.Sin(float64(a.updates)/5)
	}
	collapsed := a.learnFolded(p)

	uiFillRect(screen, p.X, p.Y, p.W, p.H, uiTutorPanel, false)
	if flashing {
		uiFillRect(screen, p.X, p.Y, p.W, p.H, fade(uiTutorHeading, 0.28*pulse), false)
	}
	uiStrokeRect(screen, p.X, p.Y, p.W, p.H, 1, uiTutorBorder, false)

	// header: title, what is left, and the fold arrow
	if lay.header.hit(mx, my) {
		uiFillRect(screen, p.X+1, p.Y+1, p.W-2, learnHeaderH-1, fade(uiTutorHeading, 0.14), false)
	}
	hy := float64(p.Y) + (learnHeaderH-lineH(tsLabel))/2
	drawText(screen, "Learning helper", float64(p.X)+10, hy, withColor(tsLabel, uiTutorHeading))
	right := "Done!"
	if !flashing {
		right = fmt.Sprintf("%d left", a.learnCount())
	}
	st := withColor(tsSmall, uiTutorText)
	st.Align = alignRight
	drawText(screen, right, float64(p.right())-28, float64(p.Y)+(learnHeaderH-lineH(tsSmall))/2, st)
	a.drawLearnChevron(screen, p.right()-16, p.Y+learnHeaderH/2, collapsed)

	if !collapsed {
		a.drawLearnRows(screen, lay, mx, my)
		note := fmt.Sprintf("%d of %d learned", len(lessonCatalog)-a.learnCount(), len(lessonCatalog))
		nst := withColor(tsSmall, learnMix(uiTutorText, uiTutorPanel, 0.2))
		if flashing {
			note, nst = "Done: "+flashTitle, withColor(tsSmall, uiTutorHeading)
		}
		drawText(screen, fitText(note, nst, p.W-20), float64(p.X)+10, float64(lay.footer.Y)+(learnFooterH-lineH(tsSmall))/2, nst)
	}
	if lay.open != nil && !lay.card.empty() {
		a.drawLearnCard(screen, lay, mx, my)
	}
}

// drawLearnChevron draws the fold arrow at (cx, cy): down while open, right
// while folded.
func (a *App) drawLearnChevron(screen *ebiten.Image, cx, cy float32, collapsed bool) {
	if collapsed {
		uiLine(screen, cx-2, cy-5, cx+3, cy, 2, uiTutorHeading, true)
		uiLine(screen, cx+3, cy, cx-2, cy+5, 2, uiTutorHeading, true)
		return
	}
	uiLine(screen, cx-5, cy-2, cx, cy+3, 2, uiTutorHeading, true)
	uiLine(screen, cx, cy+3, cx+5, cy-2, 2, uiTutorHeading, true)
}

// drawLearnRows draws the listed lessons.
func (a *App) drawLearnRows(screen *ebiten.Image, lay learnLayout, mx, my int) {
	dark := learnMix(uiTutorPanel, color.RGBA{A: 0xFF}, 0.28)
	for _, rw := range lay.rows {
		r := rw.r
		fill := dark
		switch {
		case lay.open == rw.l:
			fill = learnMix(dark, uiTutorBorder, 0.45)
		case r.hit(mx, my):
			fill = learnMix(dark, uiTutorBorder, 0.25)
		}
		uiFillRect(screen, r.X, r.Y, r.W, r.H, fill, false)
		accent := uiTutorBorder
		if lay.open == rw.l {
			accent = uiTutorHeading
		}
		uiFillRect(screen, r.X, r.Y, 4, r.H, accent, false)
		uiStrokeRect(screen, r.X, r.Y, r.W, r.H, 1, uiTutorBorder, false)
		inner := r.W - learnRowPad*2
		block := lineH(tsLabel) + lineH(tsSmall)
		ty := float64(r.Y) + (float64(r.H)-block)/2
		drawText(screen, fitText(rw.l.title, tsLabel, inner), float64(r.X)+learnRowPad, ty, withColor(tsLabel, uiTutorHeading))
		drawText(screen, fitText(rw.l.goal, tsSmall, inner), float64(r.X)+learnRowPad, ty+lineH(tsLabel), withColor(tsSmall, uiTutorText))
	}
}

// drawLearnCard draws the detail card of the open lesson.
func (a *App) drawLearnCard(screen *ebiten.Image, lay learnLayout, mx, my int) {
	c := lay.card
	uiFillRect(screen, c.X, c.Y, c.W, c.H, uiTutorPanel, false)
	uiStrokeRect(screen, c.X, c.Y, c.W, c.H, 1, uiTutorBorder, false)
	x := float64(c.X) + learnCardPad
	y := float64(c.Y) + learnCardPad
	drawText(screen, fitText(lay.open.title, tsLabel, c.W-2*learnCardPad), x, y, withColor(tsLabel, uiTutorHeading))
	y += float64(lineStep(tsLabel)) + 6
	for _, ln := range lay.body {
		drawText(screen, ln, x, y, withColor(tsBody, uiTutorText))
		y += float64(lineStep(tsBody))
	}
	y += 8
	for _, ln := range lay.goal {
		drawText(screen, ln, x, y, withColor(tsSmall, uiTutorHeading))
		y += float64(lineStep(tsSmall))
	}
	g := lay.gotIt
	fill := learnMix(uiTutorPanel, color.RGBA{A: 0xFF}, 0.3)
	if g.hit(mx, my) {
		fill = learnMix(fill, uiTutorBorder, 0.45)
	}
	uiFillRect(screen, g.X, g.Y, g.W, g.H, fill, false)
	uiStrokeRect(screen, g.X, g.Y, g.W, g.H, 1, uiTutorBorder, false)
	cx, cy := g.center()
	drawCentered(screen, "Got it", float64(cx), float64(cy), withColor(tsLabel, uiTutorHeading))
}
