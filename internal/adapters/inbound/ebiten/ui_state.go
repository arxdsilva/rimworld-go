package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/input"
)

// uiSt is the App's widget-kit state.
func (a *App) uiSt() *uiState { return &a.ui }

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
	if u.HeldN == nil {
		u.HeldN = map[string]int{}
	}
	now := map[string]bool{}
	for _, k := range raw.Held {
		now[k] = true
	}
	for _, k := range raw.Pressed {
		now[k] = true
	}
	for k := range u.HeldN {
		if !now[k] {
			delete(u.HeldN, k)
		}
	}
	for k := range now {
		u.HeldN[k]++
	}
	u.pageDelta = 0
	if a.textActive() {
		return
	}
	mx, my := a.toLogical(raw.Mx, raw.My)
	if a.cursor != nil {
		mx, my = a.cursor[0], a.cursor[1]
	}
	if u.pageOwner(mx, my) { // a scroll view under the cursor or focused: no camera zoom
		for _, k := range []struct {
			key string
			dir int
		}{{"PageUp", -1}, {"PageDown", 1}} {
			if repeatFire(u.HeldN[k.key]) {
				u.pageDelta += k.dir
			}
			dropPressed(raw, k.key)
		}
	}
	if len(u.Modals) > 0 && rawPressed(raw, "Escape") {
		a.uiCloseTop()
		dropPressed(raw, "Escape")
	}
	if rawPressed(raw, "Tab") && len(u.focusables()) > 0 {
		dir := 1
		if raw.Mods&input.ModShift != 0 {
			dir = -1
		}
		u.cycleFocus(dir)
		dropPressed(raw, "Tab")
	}
}

// pageOwner reports a scroll view is focused or under the cursor, so it
// takes PageUp and PageDown.
func (u *uiState) pageOwner(mx, my int) bool {
	for _, w := range u.Cur {
		if w.Wheel && (u.Focus == w.ID || w.R.Hit(mx, my)) {
			return true
		}
	}
	return false
}

func rawPressed(raw *rawInput, k string) bool {
	for _, p := range raw.Pressed {
		if p == k {
			return true
		}
	}
	return false
}

// dropPressed removes k from the frame's pressed keys (copying, since the
// source may reuse the slice).
func dropPressed(raw *rawInput, k string) {
	out := make([]string, 0, len(raw.Pressed))
	for _, p := range raw.Pressed {
		if p != k {
			out = append(out, p)
		}
	}
	raw.Pressed = out
}

// uiBegin starts the kit's frame: tooltip timing, a new hit list, then
// the modals and the screen's widgets are declared.
func (a *App) uiBegin() {
	u := a.uiSt()
	u.frame++
	mx, my := a.currentCursor()
	if scr := a.nav.screen(); scr != u.LastScreen {
		u.LastScreen = scr
		u.reset()
		u.Cur = nil
	}
	pressing := a.in.Down(mbLeft) || a.in.Down(mbRight)
	u.tipTick(mx, my, pressing)
	if u.Active != "" && !a.in.Down(mbLeft) && !a.in.Released(mbLeft) {
		u.Active = "" // a drag whose release was missed
	}
	if pressing {
		u.TabTo = ""
	}
	u.last, u.Cur = u.Cur, u.last[:0]
	u.Paints = u.Paints[:0]
	u.Hot, u.Layer, u.Suppress = "", 0, false
	u.mx, u.my = mx, my

	a.syncDealings() // trade_ui.go: open/close the trade and comms dialogs
	a.uiRunModals()
	u.Layer = 0 // screen widgets sit under every modal (modals declare their own layers above)
	if a.nav.overlay() != overlayNone || a.nav.screen() == screenBoot {
		return
	}
	sw, sh := a.size()
	switch {
	case u.Hook != nil:
		u.Hook(a, sw, sh)
	case a.shell.screens[a.nav.screen()] != nil:
		a.shell.screens[a.nav.screen()](a, sw, sh)
	}
}

// tipTick advances the tooltip timer from the frame that just ended.
func (u *uiState) tipTick(mx, my int, pressing bool) {
	id := u.Hot
	dragging := false
	if u.Active != "" { // a captured widget keeps its tooltip while it is dragged
		if _, ok := u.Find(u.Active); ok {
			id, dragging = u.Active, true
		}
	}
	if id == "" || pressing && !dragging {
		u.tipID, u.TipFrames, u.tipText = "", 0, ""
		u.tipX, u.tipY = mx, my
		return
	}
	moved := abs(mx-u.tipX) > 3 || abs(my-u.tipY) > 3
	if id != u.tipID || moved {
		u.tipID, u.TipFrames = id, 1
		u.tipX, u.tipY = mx, my
	} else {
		u.TipFrames++
	}
	u.tipText = ""
	if w, ok := u.Find(id); ok {
		u.tipText = w.Tip
	}
	if dragging {
		u.TipFrames = max(u.TipFrames, tipDelay) // a dragged slider shows its value at once
	}
}

// uiPaint draws the queued widgets, the modal stack over their scrims
// and the tooltip. App.Draw calls it after the screen's own drawing.
func (a *App) uiPaint(dst *ebiten.Image) {
	u := a.uiSt()
	for layer := 0; layer <= len(u.Modals); layer++ {
		for _, p := range u.Paints {
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
	for i := len(u.Cur) - 1; i >= 0; i-- {
		w := u.Cur[i]
		if w.Click == nil {
			continue
		}
		b := asButton(w.R, w.ID, w.Label)
		b.Disabled, b.Reason = w.Disabled, w.Reason
		out = append(out, b)
	}
	if len(u.Modals) == 0 {
		out = append(out, a.layoutButtons(sw, sh)...)
	}
	return out
}

// uiClick runs a clicked widget id's action (applyClick calls it first);
// it reports the id was a widget's.
func (a *App) uiClick(id string) bool {
	if w, ok := a.uiSt().Find(id); ok && w.Click != nil {
		w.Click()
		return true
	}
	return false
}

// uiWheelOver reports the cursor is over a scroll view, which owns the
// mouse wheel there (the colony camera does not zoom).
func (a *App) uiWheelOver(mx, my int) bool {
	for _, w := range a.uiSt().Cur {
		if w.Wheel && w.R.Hit(mx, my) {
			return true
		}
	}
	return false
}

// uiModalOpen reports a kit modal is on screen.
func (a *App) uiModalOpen() bool { return len(a.uiSt().Modals) > 0 }

// uiTextEditing reports a kit text field has the keyboard.
func (a *App) uiTextEditing() bool { return a.uiSt().Edit.ID != "" }

// setupTooSmall reports the window is below the setup minimum and, if so,
// turns this frame's widget declarations into no-ops. Screens call it
// first in their hook and draw drawTooSmall instead of their content.
func (a *App) setupTooSmall(sw, sh int) bool {
	if !tooSmall(sw, sh) {
		return false
	}
	a.uiSt().Suppress = true
	return true
}

// paintFocusRing outlines the keyboard-focused widget of a layer.
func (u *uiState) paintFocusRing(dst *ebiten.Image, layer int) {
	if !u.kbd || u.Focus == "" || u.Suppress {
		return
	}
	for _, w := range u.Cur {
		if w.ID == u.Focus && w.layer == layer && w.Focusable {
			strokeRect(dst, w.R.Inset(-2), uiFieldFocus)
			return
		}
	}
}
