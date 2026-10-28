package ebiten

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// Modal stack (NG-1 §1.4): dialogs over a modal_dim scrim. The top modal
// takes all input; Esc and the X close it (top only); clicks outside it
// are swallowed; the input context stack gets CtxModal (contextStack).
// A modal's content is declared every Update like any widget.
//
// Two options (PLAY-018) let a main-tab window use the same frame:
//   - anchor places the frame on a rectangle instead of centring it;
//   - modeless makes it a window: no scrim, the HUD stays live, the modal
//     input context is not taken. A modeless modal is not on the stack: it
//     is the one open main-tab window (hud_window.go), declared on the
//     screen layer, closed by Escape, a tab click or its X.

const (
	modalTitleH  = 40
	modalFooterH = 52
	modalPad     = 16
	modalEdge    = 40 // minimum clearance from the screen edge
	modalCloseW  = 28
)

// modalButton is a footer button.
type modalButton struct {
	id, label string
	style     btnStyle
	// disabled returns the reason the button cannot be used ("" enables).
	disabled func() string
	onClick  func()
	// closes pops the modal after onClick.
	closes bool
}

// modal is one dialog.
type modal struct {
	id, title string
	w, h      float32
	// draw declares the body's widgets (and draws its text) in body.
	draw    func(a *App, body rect)
	buttons []modalButton // footer, laid out right to left (first = rightmost)
	// onClose runs when Esc or the X closes it.
	onClose func()
	// anchor returns the rectangle the frame sits on (nil: centred on the
	// screen). The frame is that rectangle cut to the screen.
	anchor func(a *App) rect
	// modeless makes the modal a window (see above).
	modeless bool
}

// modalGeometry places a w x h modal centred on the screen, shrunk to fit.
func modalGeometry(sw, sh int, m modal) (frame, title, body, footer rect) {
	w := max(120, min(m.w, float32(sw)-2*modalEdge))
	h := max(80, min(m.h, float32(sh)-2*modalEdge))
	return modalParts(winRect(sw, sh).Centered(w, h), m)
}

// modalParts cuts a modal's frame into its title bar, body and footer.
func modalParts(frame rect, m modal) (f, title, body, footer rect) {
	title, rest := frame.SplitV(modalTitleH)
	if len(m.buttons) > 0 {
		rest, footer = rest.SplitV(-modalFooterH)
	}
	return frame, title, rest.Inset(modalPad), footer
}

// modalLayout is where m sits this frame: on its anchor when it has one
// (cut to the screen), else centred.
func (a *App) modalLayout(sw, sh int, m modal) (frame, title, body, footer rect) {
	if m.anchor != nil {
		if r := m.anchor(a).Intersect(winRect(sw, sh)); !r.Empty() {
			return modalParts(r, m)
		}
	}
	return modalGeometry(sw, sh, m)
}

// uiPushModal opens m on top of the stack (a modal with the same id is
// already open: nothing happens). A modeless modal opens as the main-tab
// window instead and replaces any other.
func (a *App) uiPushModal(m modal) {
	if m.modeless {
		a.uiOpenWindow(m)
		return
	}
	u := a.uiSt()
	for _, o := range u.modals {
		if o.id == m.id {
			return
		}
	}
	u.modals = append(u.modals, m)
	u.focus, u.kbd, u.active = "", false, ""
	u.edit = textEdit{}
}

// uiCloseTop closes the top modal (Esc, the X), running its onClose.
func (a *App) uiCloseTop() {
	u := a.uiSt()
	n := len(u.modals)
	if n == 0 {
		a.uiCloseWindow() // only the main-tab window is left
		return
	}
	m := u.modals[n-1]
	u.modals = u.modals[:n-1]
	u.focus, u.kbd, u.active = "", false, ""
	u.edit = textEdit{}
	if m.onClose != nil {
		m.onClose()
	}
}

// uiCloseModal removes the modal id wherever it is in the stack, without
// running onClose.
func (a *App) uiCloseModal(id string) {
	u := a.uiSt()
	if k := a.hst.win.kit; k != nil && k.id == id && a.windowKitOpen() {
		a.closeWindowQuietly()
		return
	}
	for i, m := range u.modals {
		if m.id == id {
			u.modals = append(u.modals[:i], u.modals[i+1:]...)
			u.focus, u.kbd, u.active = "", false, ""
			u.edit = textEdit{}
			return
		}
	}
}

// uiModalTop is the id of the top modal: the top of the stack, else the
// main-tab window (which sits under every stacked modal), else "".
func (a *App) uiModalTop() string {
	if m := a.uiSt().modals; len(m) > 0 {
		return m[len(m)-1].id
	}
	if a.windowKitOpen() {
		return a.hst.win.kit.id
	}
	return ""
}

// uiRunModals declares every open modal (the top one live), then swallows
// the mouse presses nothing in a modal used.
func (a *App) uiRunModals() {
	u := a.uiSt()
	sw, sh := a.size()
	for i := 0; i < len(u.modals); i++ {
		m := u.modals[i]
		u.layer = i + 1
		frame, titleR, body, footer := a.modalLayout(sw, sh, m)
		u.queue(func(dst *ebiten.Image) { fillRect(dst, winRect(sw, sh), uiModalDim) })
		a.uiModalChrome(m, frame, titleR)
		n := len(u.modals)
		if a.uiButton("modal."+m.id+".close", modalCloseRect(titleR), "X", btnOpts{Tip: "Close"}) {
			a.uiCloseTop()
			return
		}
		if m.draw != nil {
			m.draw(a, body)
		}
		a.uiModalButtons(m, footer)
		if len(u.modals) != n {
			break // a callback changed the stack: redeclare next frame
		}
	}
	if len(u.modals) > 0 { // nothing outside the modal gets a click
		for _, b := range []mouseBtn{mbLeft, mbRight, mbMiddle} {
			if a.in.clicked(b) {
				a.in.take(b)
			}
		}
	}
}

// uiRunWindow declares the main-tab window m on the screen layer: its
// frame sits on its anchor, there is no scrim, and the HUD around it stays
// live. hud_window.go calls it once per Update while the window is open.
func (a *App) uiRunWindow(m modal) {
	sw, sh := a.size()
	frame, titleR, body, footer := a.modalLayout(sw, sh, m)
	a.uiModalChrome(m, frame, titleR)
	if a.uiButton("modal."+m.id+".close", modalCloseRect(titleR), "X", btnOpts{Tip: "Close"}) {
		a.uiCloseWindow()
		return
	}
	if m.draw != nil {
		m.draw(a, body)
	}
	a.uiModalButtons(m, footer)
}

// uiModalChrome queues a modal's panel and title.
func (a *App) uiModalChrome(m modal, frame, titleR rect) {
	a.uiSt().queue(func(dst *ebiten.Image) {
		drawPanel(dst, frame.X, frame.Y, frame.W, frame.H)
		uiLine(dst, frame.X, titleR.Bottom(), frame.Right(), titleR.Bottom(), 1, uiPanelBorder, false)
		st := tsLarge
		st.Color = uiHeading
		drawText(dst, fitText(m.title, st, titleR.W-modalCloseW-3*modalPad), float64(titleR.X+modalPad), float64(titleR.Y)+(float64(titleR.H)-lineH(st))/2, st)
	})
}

// modalCloseRect is the X button in a title bar.
func modalCloseRect(titleR rect) rect {
	return rect{X: titleR.Right() - modalCloseW - 8, Y: titleR.Y + (titleR.H-modalCloseW)/2, W: modalCloseW, H: modalCloseW}
}

// uiModalButtons declares the footer buttons, right to left.
func (a *App) uiModalButtons(m modal, footer rect) {
	bx := footer.Right() - modalPad
	for _, b := range m.buttons {
		bw := max(100, textW(b.label, tsLabel)+32)
		br := rect{X: bx - bw, Y: footer.Y + (footer.H-footerBtnH)/2, W: bw, H: footerBtnH}
		bx -= bw + 8
		o := btnOpts{Style: b.style}
		if b.disabled != nil {
			o.Reason = b.disabled()
			o.Disabled = o.Reason != ""
		}
		if a.uiButton("modal."+m.id+"."+b.id, br, b.label, o) {
			if b.onClick != nil {
				b.onClick()
			}
			if b.closes {
				a.uiCloseModal(m.id)
			}
		}
	}
}

// uiConfirm opens a Cancel / OK dialog; onOK runs when OK is pressed.
func (a *App) uiConfirm(id, title, body, okLabel string, danger bool, onOK func()) {
	const w = 460
	lines := wrapPx(body, tsBody, w-2*modalPad)
	h := float32(modalTitleH+modalFooterH+2*modalPad) + float32(len(lines))*float32(lineH(tsBody))
	style := btnPrimary
	if danger {
		style = btnDanger
	}
	a.uiPushModal(modal{
		id: id, title: title, w: w, h: h,
		draw: func(a *App, body rect) {
			y := body.Y
			for _, l := range lines {
				a.uiLabel(body.X, y, l, tsBody)
				y += float32(lineH(tsBody))
			}
		},
		buttons: []modalButton{
			{id: "ok", label: okLabel, style: style, closes: true, onClick: onOK},
			{id: "cancel", label: "Cancel", closes: true},
		},
	})
}
