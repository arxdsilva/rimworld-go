package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/input"
)

// The widget kit lives in uikit (ARCH-003). What stays here is the App's
// side of it: binding the kit to the App, the keys the App routes before
// the kit sees them, the screen table the kit's frame dispatches to, and
// the click path's hit list.

// uiSt is the App's widget kit, bound to the App on first use.
func (a *App) uiSt() *uiState {
	if !a.ui.Bound() {
		a.ui.Bind(uiHost{a})
	}
	return &a.ui
}

// uiPreInput runs before the frame is resolved against the bindings: it
// counts held keys (keyRepeat) and takes the keys the kit owns out of the
// frame so no binding also fires: PageUp/PageDown over a scroll view, Esc
// closes the top modal, Tab moves focus.
func (a *App) uiPreInput(raw *rawInput) {
	u := a.uiSt()
	u.CountHeld(raw)
	if a.textActive() {
		return
	}
	mx, my := a.toLogical(raw.Mx, raw.My)
	if a.cursor != nil {
		mx, my = a.cursor[0], a.cursor[1]
	}
	u.TakePages(raw, mx, my)
	if len(u.Modals) > 0 && uikit.RawPressed(raw, "Escape") {
		a.uiCloseTop()
		uikit.DropPressed(raw, "Escape")
	}
	u.TakeTab(raw, raw.Mods&input.ModShift != 0)
}

// uiBegin starts the kit's frame (a screen change first forgets focus,
// modals and edits), then the modals and the screen's widgets are
// declared.
func (a *App) uiBegin() {
	u := a.uiSt()
	last, _ := u.LastScreen.(screenID) // unset: the zero screen
	if scr := a.nav.screen(); scr != last {
		u.LastScreen = scr
		u.Reset()
	}
	u.Begin()
	a.syncDealings() // trade_ui.go: open/close the trade and comms dialogs
	u.RunModals(a.uiCloseModal)
	if a.nav.overlay() != overlayNone || a.nav.screen() == screenBoot {
		return
	}
	sw, sh := a.size()
	switch {
	case u.Hook != nil:
		u.Hook(sw, sh)
	case a.shell.screens[a.nav.screen()] != nil:
		a.shell.screens[a.nav.screen()](a, sw, sh)
	}
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
