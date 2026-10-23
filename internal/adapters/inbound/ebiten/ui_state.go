package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/input"
)

// uiSt is the App's widget-kit state.
func (a *App) uiSt() *uiState { return &a.ui }

// screenUIs are the per-screen widget declarations, filled at init time
// by the screen files (registerScreenUI) and only read afterwards.
var screenUIs = map[screenID]func(a *App, sw, sh int){}

// registerScreenUI sets the function that declares a screen's widgets.
// Call it from an init() in the screen's file. It runs once per Update,
// right after the input is polled, while the screen is on top with no
// overlay.
func registerScreenUI(s screenID, fn func(a *App, sw, sh int)) { screenUIs[s] = fn }

// Tooltip metrics (NG-1 §1.4).
const (
	tipDelay = 24  // frames the cursor rests before the card shows
	tipMaxW  = 360 // card width cap in px
)

// uiPreInput runs before the frame is resolved against the bindings: it
// counts held keys (keyRepeat) and takes the keys the kit owns out of the
// frame so no binding also fires: Esc closes the top modal, Tab moves
// focus.
func (a *App) uiPreInput(raw *rawInput) {
	u := a.uiSt()
	if u.heldN == nil {
		u.heldN = map[string]int{}
	}
	now := map[string]bool{}
	for _, k := range raw.held {
		now[k] = true
	}
	for _, k := range raw.pressed {
		now[k] = true
	}
	for k := range u.heldN {
		if !now[k] {
			delete(u.heldN, k)
		}
	}
	for k := range now {
		u.heldN[k]++
	}
	u.pageDelta = 0
	if a.textActive() {
		return
	}
	mx, my := raw.mx, raw.my
	if a.cursor != nil {
		mx, my = a.cursor[0], a.cursor[1]
	}
	if u.pageOwner(mx, my) { // a scroll view under the cursor or focused: no camera zoom
		for _, k := range []struct {
			key string
			dir int
		}{{"PageUp", -1}, {"PageDown", 1}} {
			if repeatFire(u.heldN[k.key]) {
				u.pageDelta += k.dir
			}
			dropPressed(raw, k.key)
		}
	}
	if len(u.modals) > 0 && rawPressed(raw, "Escape") {
		a.uiCloseTop()
		dropPressed(raw, "Escape")
	}
	if rawPressed(raw, "Tab") && len(u.focusables()) > 0 {
		dir := 1
		if raw.mods&input.ModShift != 0 {
			dir = -1
		}
		u.cycleFocus(dir)
		dropPressed(raw, "Tab")
	}
}

// pageOwner reports a scroll view is focused or under the cursor, so it
// takes PageUp and PageDown.
func (u *uiState) pageOwner(mx, my int) bool {
	for _, w := range u.cur {
		if w.wheel && (u.focus == w.id || w.r.hit(mx, my)) {
			return true
		}
	}
	return false
}

func rawPressed(raw *rawInput, k string) bool {
	for _, p := range raw.pressed {
		if p == k {
			return true
		}
	}
	return false
}

// dropPressed removes k from the frame's pressed keys (copying, since the
// source may reuse the slice).
func dropPressed(raw *rawInput, k string) {
	out := make([]string, 0, len(raw.pressed))
	for _, p := range raw.pressed {
		if p != k {
			out = append(out, p)
		}
	}
	raw.pressed = out
}

// uiBegin starts the kit's frame: tooltip timing, a new hit list, then
// the modals and the screen's widgets are declared.
func (a *App) uiBegin() {
	u := a.uiSt()
	u.frame++
	mx, my := a.currentCursor()
	if scr := a.nav.screen(); scr != u.lastScreen {
		u.lastScreen = scr
		u.reset()
		u.cur = nil
	}
	pressing := a.in.down(mbLeft) || a.in.down(mbRight)
	u.tipTick(mx, my, pressing)
	if u.active != "" && !a.in.down(mbLeft) && !a.in.released(mbLeft) {
		u.active = "" // a drag whose release was missed
	}
	if pressing {
		u.tabTo = ""
	}
	u.last, u.cur = u.cur, u.last[:0]
	u.paints = u.paints[:0]
	u.hot, u.layer, u.suppress = "", 0, false
	u.mx, u.my = mx, my

	a.uiRunModals()
	u.layer = len(u.modals)
	if a.nav.overlay() != overlayNone || a.nav.screen() == screenBoot {
		return
	}
	sw, sh := a.size()
	switch {
	case u.hook != nil:
		u.hook(a, sw, sh)
	case screenUIs[a.nav.screen()] != nil:
		screenUIs[a.nav.screen()](a, sw, sh)
	}
}

// tipTick advances the tooltip timer from the frame that just ended.
func (u *uiState) tipTick(mx, my int, pressing bool) {
	id := u.hot
	dragging := false
	if u.active != "" { // a captured widget keeps its tooltip while it is dragged
		if _, ok := u.find(u.active); ok {
			id, dragging = u.active, true
		}
	}
	if id == "" || pressing && !dragging {
		u.tipID, u.tipFrames, u.tipText = "", 0, ""
		u.tipX, u.tipY = mx, my
		return
	}
	moved := abs(mx-u.tipX) > 3 || abs(my-u.tipY) > 3
	if id != u.tipID || moved {
		u.tipID, u.tipFrames = id, 1
		u.tipX, u.tipY = mx, my
	} else {
		u.tipFrames++
	}
	u.tipText = ""
	if w, ok := u.find(id); ok {
		u.tipText = w.tip
	}
	if dragging {
		u.tipFrames = max(u.tipFrames, tipDelay) // a dragged slider shows its value at once
	}
}

// uiPaint draws the queued widgets, the modal stack over their scrims
// and the tooltip. App.Draw calls it after the screen's own drawing.
func (a *App) uiPaint(dst *ebiten.Image) {
	u := a.uiSt()
	for layer := 0; layer <= len(u.modals); layer++ {
		for _, p := range u.paints {
			if p.layer == layer {
				p.fn(dst)
			}
		}
		u.paintFocusRing(dst, layer)
	}
	a.uiPaintTooltip(dst)
}

// layoutHits is layoutButtons plus the widgets' hit rectangles, so the
// click path (and tests that click by id) reach both. Widgets come first:
// they sit on top. While a modal is open the screen's buttons are
// unreachable.
func (a *App) layoutHits(sw, sh int) []button {
	u := a.uiSt()
	var out []button
	for i := len(u.cur) - 1; i >= 0; i-- {
		w := u.cur[i]
		if w.click == nil {
			continue
		}
		b := w.r.asButton(w.id, w.label)
		b.disabled, b.reason = w.disabled, w.reason
		out = append(out, b)
	}
	if len(u.modals) == 0 {
		out = append(out, a.layoutButtons(sw, sh)...)
	}
	return out
}

// uiClick runs a clicked widget id's action (applyClick calls it first);
// it reports the id was a widget's.
func (a *App) uiClick(id string) bool {
	if w, ok := a.uiSt().find(id); ok && w.click != nil {
		w.click()
		return true
	}
	return false
}

// uiWheelOver reports the cursor is over a scroll view, which owns the
// mouse wheel there (the colony camera does not zoom).
func (a *App) uiWheelOver(mx, my int) bool {
	for _, w := range a.uiSt().cur {
		if w.wheel && w.r.hit(mx, my) {
			return true
		}
	}
	return false
}

// uiModalOpen reports a kit modal is on screen.
func (a *App) uiModalOpen() bool { return len(a.uiSt().modals) > 0 }

// uiTextEditing reports a kit text field has the keyboard.
func (a *App) uiTextEditing() bool { return a.uiSt().edit.id != "" }

// setupTooSmall reports the window is below the setup minimum and, if so,
// turns this frame's widget declarations into no-ops. Screens call it
// first in their hook and draw drawTooSmall instead of their content.
func (a *App) setupTooSmall(sw, sh int) bool {
	if !tooSmall(sw, sh) {
		return false
	}
	a.uiSt().suppress = true
	return true
}

// paintFocusRing outlines the keyboard-focused widget of a layer.
func (u *uiState) paintFocusRing(dst *ebiten.Image, layer int) {
	if !u.kbd || u.focus == "" || u.suppress {
		return
	}
	for _, w := range u.cur {
		if w.id == u.focus && w.layer == layer && w.focusable {
			strokeRect(dst, w.r.inset(-2), uiFieldFocus)
			return
		}
	}
}
