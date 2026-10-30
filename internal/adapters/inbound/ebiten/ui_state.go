package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	uikitpkg "rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/input"
)

// The widget kit lives in uikit (ARCH-003). What stays here is the App's
// side of it: binding the kit to the App, the keys the App routes before
// the kit sees them, the screen table the kit's frame dispatches to, and
// the click path's hit list.

// uiSt is the App's widget kit, bound to the App on first use.
func (a *App) uiSt() *uikitpkg.Kit {
	if !a.ui.Bound() {
		a.ui.Bind(uiHost{a})
	}
	return &a.ui
}

// uiPreInput runs before the frame is resolved against the bindings: it
// counts held keys (keyRepeat) and takes the keys the kit owns out of the
// frame so no binding also fires: PageUp/PageDown over a scroll view, Esc
// closes the top modal, Tab moves focus.
func (a *App) uiPreInput(raw *inputframe.Raw) {
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
	last, _ := u.LastScreen.(ports.Screen) // unset: the zero screen
	if scr := a.nav.screen(); scr != last {
		u.LastScreen = scr
		u.Reset()
	}
	u.Begin()
	a.deal.SyncDealings() // trade_ui.go: open/close the trade and comms dialogs
	floatOverModal(a)
	u.RunModals(a.uiCloseModal)
	if a.nav.overlay() != overlayNone || a.nav.screen() == ports.ScreenBoot {
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
func (a *App) layoutHits(sw, sh int) []uikitpkg.Button {
	u := a.uiSt()
	var out []uikitpkg.Button
	for i := len(u.Cur) - 1; i >= 0; i-- {
		w := u.Cur[i]
		if w.Click == nil {
			continue
		}
		b := uikitpkg.AsButton(w.R, w.ID, w.Label)
		b.Disabled, b.Reason = w.Disabled, w.Reason
		out = append(out, b)
	}
	if len(u.Modals) == 0 {
		out = append(out, a.layoutButtons(sw, sh)...)
	}
	return out
}

// floatOverModal lets a pick menu opened from a dialog (the bill's "Take
// from") take a click on its rows before the modal swallows the press.
func floatOverModal(a *App) {
	u := a.uiSt()
	if len(u.Modals) == 0 || !a.colonyLive() || !a.ord.Float.Open || !a.in.Clicked(inputframe.Left) {
		return
	}
	mx, my := a.currentCursor()
	if b, ok := uikitpkg.HitButton(a.ord.FloatButtons(), mx, my); ok {
		a.in.Take(inputframe.Left)
		if !b.Disabled {
			a.applyClick(b.ID)
		}
	}
}
