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
	ID, Label string
	Style     btnStyle
	// Disabled returns the reason the button cannot be used ("" enables).
	Disabled func() string
	OnClick  func()
	// Closes pops the modal after onClick.
	Closes bool
}

// modal is one dialog.
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
	// Modeless makes the modal a window (see above).
	Modeless bool
}

// modalGeometry places a w x h modal centred on the screen, shrunk to fit.
func modalGeometry(sw, sh int, m modal) (frame, title, body, footer rect) {
	w := max(120, min(m.W, float32(sw)-2*modalEdge))
	h := max(80, min(m.H, float32(sh)-2*modalEdge))
	return modalParts(winRect(sw, sh).Centered(w, h), m)
}

// modalParts cuts a modal's frame into its title bar, body and footer.
func modalParts(frame rect, m modal) (f, title, body, footer rect) {
	title, rest := frame.SplitV(modalTitleH)
	if len(m.Buttons) > 0 {
		rest, footer = rest.SplitV(-modalFooterH)
	}
	return frame, title, rest.Inset(modalPad), footer
}

// modalLayout is where m sits this frame: on its anchor when it has one
// (cut to the screen), else centred.
func (a *App) modalLayout(sw, sh int, m modal) (frame, title, body, footer rect) {
	if m.Anchor != nil {
		if r := m.Anchor(a).Intersect(winRect(sw, sh)); !r.Empty() {
			return modalParts(r, m)
		}
	}
	return modalGeometry(sw, sh, m)
}

// uiPushModal opens m on top of the stack (a modal with the same id is
// already open: nothing happens). A modeless modal opens as the main-tab
// window instead and replaces any other.
func (a *App) uiPushModal(m modal) {
	if m.Modeless {
		a.uiOpenWindow(m)
		return
	}
	u := a.uiSt()
	for _, o := range u.Modals {
		if o.ID == m.ID {
			return
		}
	}
	u.Modals = append(u.Modals, m)
	u.Focus, u.kbd, u.Active = "", false, ""
	u.Edit = textEdit{}
}

// uiCloseTop closes the top modal (Esc, the X), running its onClose.
func (a *App) uiCloseTop() {
	u := a.uiSt()
	n := len(u.Modals)
	if n == 0 {
		a.uiCloseWindow() // only the main-tab window is left
		return
	}
	m := u.Modals[n-1]
	u.Modals = u.Modals[:n-1]
	u.Focus, u.kbd, u.Active = "", false, ""
	u.Edit = textEdit{}
	if m.OnClose != nil {
		m.OnClose()
	}
}

// uiCloseModal removes the modal id wherever it is in the stack, without
// running onClose.
func (a *App) uiCloseModal(id string) {
	u := a.uiSt()
	if k := a.hst.win.kit; k != nil && k.ID == id && a.windowKitOpen() {
		a.closeWindowQuietly()
		return
	}
	for i, m := range u.Modals {
		if m.ID == id {
			u.Modals = append(u.Modals[:i], u.Modals[i+1:]...)
			u.Focus, u.kbd, u.Active = "", false, ""
			u.Edit = textEdit{}
			return
		}
	}
}

// uiModalTop is the id of the top modal: the top of the stack, else the
// main-tab window (which sits under every stacked modal), else "".
func (a *App) uiModalTop() string {
	if m := a.uiSt().Modals; len(m) > 0 {
		return m[len(m)-1].ID
	}
	if a.windowKitOpen() {
		return a.hst.win.kit.ID
	}
	return ""
}

// uiRunModals declares every open modal (the top one live), then swallows
// the mouse presses nothing in a modal used.
func (a *App) uiRunModals() {
	u := a.uiSt()
	sw, sh := a.size()
	for i := 0; i < len(u.Modals); i++ {
		m := u.Modals[i]
		u.Layer = i + 1
		frame, titleR, body, footer := a.modalLayout(sw, sh, m)
		u.Queue(func(dst *ebiten.Image) { fillRect(dst, winRect(sw, sh), uiModalDim) })
		a.uiModalChrome(m, frame, titleR)
		n := len(u.Modals)
		if a.uiButton("modal."+m.ID+".close", modalCloseRect(titleR), "X", btnOpts{Tip: "Close"}) {
			a.uiCloseTop()
			return
		}
		if m.Draw != nil {
			m.Draw(a, body)
		}
		a.uiModalButtons(m, footer)
		if len(u.Modals) != n {
			break // a callback changed the stack: redeclare next frame
		}
	}
	if len(u.Modals) > 0 { // nothing outside the modal gets a click
		for _, b := range []mouseBtn{mbLeft, mbRight, mbMiddle} {
			if a.in.Clicked(b) {
				a.in.Take(b)
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
	if a.uiButton("modal."+m.ID+".close", modalCloseRect(titleR), "X", btnOpts{Tip: "Close"}) {
		a.uiCloseWindow()
		return
	}
	if m.Draw != nil {
		m.Draw(a, body)
	}
	a.uiModalButtons(m, footer)
}

// uiModalChrome queues a modal's panel and title.
func (a *App) uiModalChrome(m modal, frame, titleR rect) {
	a.uiSt().Queue(func(dst *ebiten.Image) {
		drawPanel(dst, frame.X, frame.Y, frame.W, frame.H)
		uiLine(dst, frame.X, titleR.Bottom(), frame.Right(), titleR.Bottom(), 1, uiPanelBorder, false)
		st := tsLarge
		st.Color = uiHeading
		drawText(dst, fitText(m.Title, st, titleR.W-modalCloseW-3*modalPad), float64(titleR.X+modalPad), float64(titleR.Y)+(float64(titleR.H)-lineH(st))/2, st)
	})
}

// modalCloseRect is the X button in a title bar.
func modalCloseRect(titleR rect) rect {
	return rect{X: titleR.Right() - modalCloseW - 8, Y: titleR.Y + (titleR.H-modalCloseW)/2, W: modalCloseW, H: modalCloseW}
}

// uiModalButtons declares the footer buttons, right to left.
func (a *App) uiModalButtons(m modal, footer rect) {
	bx := footer.Right() - modalPad
	for _, b := range m.Buttons {
		bw := max(100, textW(b.Label, tsLabel)+32)
		br := rect{X: bx - bw, Y: footer.Y + (footer.H-footerBtnH)/2, W: bw, H: footerBtnH}
		bx -= bw + 8
		o := btnOpts{Style: b.Style}
		if b.Disabled != nil {
			o.Reason = b.Disabled()
			o.Disabled = o.Reason != ""
		}
		if a.uiButton("modal."+m.ID+"."+b.ID, br, b.Label, o) {
			if b.OnClick != nil {
				b.OnClick()
			}
			if b.Closes {
				a.uiCloseModal(m.ID)
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
		ID: id, Title: title, W: w, H: h,
		Draw: func(a *App, body rect) {
			y := body.Y
			for _, l := range lines {
				a.uiLabel(body.X, y, l, tsBody)
				y += float32(lineH(tsBody))
			}
		},
		Buttons: []modalButton{
			{ID: "ok", Label: okLabel, Style: style, Closes: true, OnClick: onOK},
			{ID: "cancel", Label: "Cancel", Closes: true},
		},
	})
}
