package ebiten

import "rimworld-go/internal/adapters/inbound/ebiten/uikit"

// The modal stack lives in uikit (ARCH-003). The App declares its modals
// with a body that takes the App, and owns the one modeless main-tab
// window (hud_window.go) that sits under the stack: a modeless modal
// opens there, and Esc, the X and a footer close reach it when the stack
// is empty.

// modal is one dialog as the App declares it (uikit.Modal with the App
// passed to Draw and Anchor).
type modal struct {
	ID, Title string
	W, H      float32
	// Draw declares the body's widgets (and draws its text) in body.
	Draw    func(a *App, body rect)
	Buttons []modalButton // footer, laid out right to left (first = rightmost)
	// OnClose runs when Esc or the X closes it.
	OnClose func()
	// Anchor returns the rectangle the frame sits on (nil: centred on the
	// screen). The frame is that rectangle cut to the screen.
	Anchor func(a *App) rect
	// Modeless makes the modal the main-tab window: no scrim, the HUD
	// stays live, the modal input context is not taken; it is closed by
	// Escape, a tab click or its X.
	Modeless bool
}

// kitModal is m bound to the App.
func (a *App) kitModal(m modal) uikit.Modal {
	km := uikit.Modal{ID: m.ID, Title: m.Title, W: m.W, H: m.H, Buttons: m.Buttons, OnClose: m.OnClose}
	if m.Draw != nil {
		km.Draw = func(body rect) { m.Draw(a, body) }
	}
	if m.Anchor != nil {
		km.Anchor = func() rect { return m.Anchor(a) }
	}
	return km
}

// modalGeometry places a w x h modal centred on the screen, shrunk to fit.
func modalGeometry(sw, sh int, m modal) (frame, title, body, footer rect) {
	return uikit.ModalGeometry(sw, sh, uikit.Modal{W: m.W, H: m.H, Buttons: m.Buttons})
}

// uiPushModal opens m on top of the stack (a modal with the same id is
// already open: nothing happens). A modeless modal opens as the main-tab
// window instead and replaces any other.
func (a *App) uiPushModal(m modal) {
	if m.Modeless {
		a.win.OpenKit(a.kitModal(m))
		return
	}
	a.uiSt().Push(a.kitModal(m))
}

// uiCloseTop closes the top modal (Esc, the X), running its onClose; with
// the stack empty it closes the main-tab window.
func (a *App) uiCloseTop() {
	if !a.uiSt().CloseTop() {
		a.win.CloseKit()
	}
}

// uiCloseModal removes the modal id (the main-tab window, or wherever it
// is in the stack) without running onClose.
func (a *App) uiCloseModal(id string) { a.win.CloseModal(id) }

// uiModalTop is the id of the top modal: the top of the stack, else the
// main-tab window (which sits under every stacked modal), else "".
func (a *App) uiModalTop() string {
	if a.uiSt().ModalOpen() {
		return a.uiSt().Top()
	}
	return a.win.KitID()
}
