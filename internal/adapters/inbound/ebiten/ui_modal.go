package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Modal stack (NG-1 §1.4): dialogs over a modal_dim scrim. The top modal
// takes all input; Esc and the X close it (top only); clicks outside it
// are swallowed; the input context stack gets CtxModal (contextStack).
// A modal's content is declared every Update like any widget.

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
}

// modalGeometry places a w x h modal centred on the screen, shrunk to fit.
func modalGeometry(sw, sh int, m modal) (frame, title, body, footer rect) {
	w := max(120, min(m.w, float32(sw)-2*modalEdge))
	h := max(80, min(m.h, float32(sh)-2*modalEdge))
	frame = winRect(sw, sh).centered(w, h)
	title, rest := frame.splitV(modalTitleH)
	if len(m.buttons) > 0 {
		rest, footer = rest.splitV(-modalFooterH)
	}
	return frame, title, rest.inset(modalPad), footer
}

// uiPushModal opens m on top of the stack (a modal with the same id is
// already open: nothing happens).
func (a *App) uiPushModal(m modal) {
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
	for i, m := range u.modals {
		if m.id == id {
			u.modals = append(u.modals[:i], u.modals[i+1:]...)
			u.focus, u.kbd, u.active = "", false, ""
			u.edit = textEdit{}
			return
		}
	}
}

// uiModalTop is the id of the top modal ("" when none).
func (a *App) uiModalTop() string {
	if m := a.uiSt().modals; len(m) > 0 {
		return m[len(m)-1].id
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
		frame, titleR, body, footer := modalGeometry(sw, sh, m)
		u.queue(func(dst *ebiten.Image) { fillRect(dst, winRect(sw, sh), uiModalDim) })
		u.queue(func(dst *ebiten.Image) {
			drawPanel(dst, frame.X, frame.Y, frame.W, frame.H)
			uiLine(dst, frame.X, titleR.bottom(), frame.right(), titleR.bottom(), 1, uiPanelBorder, false)
			st := tsLarge
			st.Color = uiHeading
			drawText(dst, fitText(m.title, st, titleR.W-modalCloseW-3*modalPad), float64(titleR.X+modalPad), float64(titleR.Y)+(float64(titleR.H)-lineH(st))/2, st)
		})
		n := len(u.modals)
		closeR := rect{titleR.right() - modalCloseW - 8, titleR.Y + (titleR.H-modalCloseW)/2, modalCloseW, modalCloseW}
		if a.uiButton("modal."+m.id+".close", closeR, "X", btnOpts{Tip: "Close"}) {
			a.uiCloseTop()
			return
		}
		if m.draw != nil {
			m.draw(a, body)
		}
		bx := footer.right() - modalPad
		for _, b := range m.buttons {
			bw := max(100, textW(b.label, tsLabel)+32)
			br := rect{bx - bw, footer.Y + (footer.H-footerBtnH)/2, bw, footerBtnH}
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

// textW is the width of s in style st.
func textW(s string, st textStyle) float32 {
	w, _ := measure(s, st)
	return float32(w)
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
