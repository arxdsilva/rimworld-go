package uikit

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
)

// Modal stack (NG-1 §1.4): dialogs over a modal_dim scrim. The top modal
// takes all input; Esc and the X close it (top only); clicks outside it
// are swallowed; the host's input context stack gives it CtxModal.
// A modal's content is declared every Update like any widget.
//
// A modal's Anchor places the frame on a rectangle instead of centring it
// (PLAY-018). The host's one modeless main-tab window uses the same frame
// through RunWindow: no scrim, declared on the screen layer, not on the
// stack.

const (
	ModalTitleH  = 40
	ModalFooterH = 52
	ModalPad     = 16
	ModalEdge    = 40 // minimum clearance from the screen edge
	ModalCloseW  = 28
)

// ModalButton is a footer button.
type ModalButton struct {
	ID, Label string
	Style     BtnStyle
	// Disabled returns the reason the button cannot be used ("" enables).
	Disabled func() string
	OnClick  func()
	// Closes pops the modal after OnClick.
	Closes bool
}

// Modal is one dialog.
type Modal struct {
	ID, Title string
	W, H      float32
	// Draw declares the body's widgets (and draws its text) in body.
	Draw    func(body gfx.Rect)
	Buttons []ModalButton // footer, laid out right to left (first = rightmost)
	// OnClose runs when Esc or the X closes it.
	OnClose func()
	// Anchor returns the rectangle the frame sits on (nil: centred on the
	// screen). The frame is that rectangle cut to the screen.
	Anchor func() gfx.Rect
}

// ModalGeometry places a w x h modal centred on the screen, shrunk to fit.
func ModalGeometry(sw, sh int, m Modal) (frame, title, body, footer gfx.Rect) {
	w := max(120, min(m.W, float32(sw)-2*ModalEdge))
	h := max(80, min(m.H, float32(sh)-2*ModalEdge))
	return modalParts(gfx.WinRect(sw, sh).Centered(w, h), m)
}

// modalParts cuts a modal's frame into its title bar, body and footer.
func modalParts(frame gfx.Rect, m Modal) (f, title, body, footer gfx.Rect) {
	title, rest := frame.SplitV(ModalTitleH)
	if len(m.Buttons) > 0 {
		rest, footer = rest.SplitV(-ModalFooterH)
	}
	return frame, title, rest.Inset(ModalPad), footer
}

// layout is where m sits this frame: on its anchor when it has one (cut
// to the screen), else centred.
func layout(sw, sh int, m Modal) (frame, title, body, footer gfx.Rect) {
	if m.Anchor != nil {
		if r := m.Anchor().Intersect(gfx.WinRect(sw, sh)); !r.Empty() {
			return modalParts(r, m)
		}
	}
	return ModalGeometry(sw, sh, m)
}

// Push opens m on top of the stack (a modal with the same id is already
// open: nothing happens).
func (k *Kit) Push(m Modal) {
	for _, o := range k.Modals {
		if o.ID == m.ID {
			return
		}
	}
	k.Modals = append(k.Modals, m)
	k.Focus, k.kbd, k.Active = "", false, ""
	k.Edit = textEdit{}
}

// CloseTop closes the top modal (Esc, the X), running its OnClose. It
// reports false when the stack was empty.
func (k *Kit) CloseTop() bool {
	n := len(k.Modals)
	if n == 0 {
		return false
	}
	m := k.Modals[n-1]
	k.Modals = k.Modals[:n-1]
	k.Focus, k.kbd, k.Active = "", false, ""
	k.Edit = textEdit{}
	if m.OnClose != nil {
		m.OnClose()
	}
	return true
}

// CloseModal removes the modal id wherever it is in the stack, without
// running OnClose.
func (k *Kit) CloseModal(id string) {
	for i, m := range k.Modals {
		if m.ID == id {
			k.Modals = append(k.Modals[:i], k.Modals[i+1:]...)
			k.Focus, k.kbd, k.Active = "", false, ""
			k.Edit = textEdit{}
			return
		}
	}
}

// Top is the id of the top modal of the stack, or "".
func (k *Kit) Top() string {
	if n := len(k.Modals); n > 0 {
		return k.Modals[n-1].ID
	}
	return ""
}

// RunModals declares every open modal (the top one live), then swallows
// the mouse presses nothing in a modal used. A footer button that closes
// its modal calls closeModal with the modal's id (the host may route it:
// a window of the same id); nil closes it on the stack. Layer is back on
// the screen (0) afterwards.
func (k *Kit) RunModals(closeModal func(id string)) {
	defer func() { k.Layer = 0 }() // screen widgets sit under every modal
	if closeModal == nil {
		closeModal = k.CloseModal
	}
	sw, sh := k.host.Size()
	for i := 0; i < len(k.Modals); i++ {
		m := k.Modals[i]
		k.Layer = i + 1
		frame, titleR, body, footer := layout(sw, sh, m)
		k.Queue(func(dst *ebiten.Image) { FillRect(dst, gfx.WinRect(sw, sh), ModalDim) })
		k.modalChrome(m, frame, titleR)
		n := len(k.Modals)
		if k.Button("modal."+m.ID+".close", modalCloseRect(titleR), "X", BtnOpts{Tip: "Close"}) {
			k.CloseTop()
			return
		}
		if m.Draw != nil {
			m.Draw(body)
		}
		k.modalButtons(m, footer, func() { closeModal(m.ID) })
		if len(k.Modals) != n {
			break // a callback changed the stack: redeclare next frame
		}
	}
	if len(k.Modals) > 0 { // nothing outside the modal gets a click
		for _, b := range []inputframe.MouseBtn{inputframe.Left, inputframe.Right, inputframe.Middle} {
			if k.in().Clicked(b) {
				k.in().Take(b)
			}
		}
	}
}

// RunWindow declares m as the host's main-tab window on the screen layer:
// its frame sits on its anchor, there is no scrim, and the HUD around it
// stays live. The X calls closeX; a footer button that closes calls
// dismiss. The host calls it once per Update while the window is open.
func (k *Kit) RunWindow(m Modal, closeX, dismiss func()) {
	sw, sh := k.host.Size()
	frame, titleR, body, footer := layout(sw, sh, m)
	k.modalChrome(m, frame, titleR)
	if k.Button("modal."+m.ID+".close", modalCloseRect(titleR), "X", BtnOpts{Tip: "Close"}) {
		closeX()
		return
	}
	if m.Draw != nil {
		m.Draw(body)
	}
	k.modalButtons(m, footer, dismiss)
}

// modalChrome queues a modal's panel and title.
func (k *Kit) modalChrome(m Modal, frame, titleR gfx.Rect) {
	k.Queue(func(dst *ebiten.Image) {
		DrawPanel(dst, frame.X, frame.Y, frame.W, frame.H)
		gfx.Line(dst, frame.X, titleR.Bottom(), frame.Right(), titleR.Bottom(), 1, PanelBorder, false)
		st := gfx.Large
		st.Color = Heading
		gfx.Text(dst, FitText(m.Title, st, titleR.W-ModalCloseW-3*ModalPad), float64(titleR.X+ModalPad), float64(titleR.Y)+(float64(titleR.H)-gfx.LineH(st))/2, st)
	})
}

// modalCloseRect is the X button in a title bar.
func modalCloseRect(titleR gfx.Rect) gfx.Rect {
	return gfx.Rect{X: titleR.Right() - ModalCloseW - 8, Y: titleR.Y + (titleR.H-ModalCloseW)/2, W: ModalCloseW, H: ModalCloseW}
}

// modalButtons declares the footer buttons, right to left; a button that
// closes calls closeFn after its OnClick.
func (k *Kit) modalButtons(m Modal, footer gfx.Rect, closeFn func()) {
	bx := footer.Right() - ModalPad
	for _, b := range m.Buttons {
		bw := max(100, gfx.TextW(b.Label, gfx.Label)+32)
		br := gfx.Rect{X: bx - bw, Y: footer.Y + (footer.H-FooterBtnH)/2, W: bw, H: FooterBtnH}
		bx -= bw + 8
		o := BtnOpts{Style: b.Style}
		if b.Disabled != nil {
			o.Reason = b.Disabled()
			o.Disabled = o.Reason != ""
		}
		if k.Button("modal."+m.ID+"."+b.ID, br, b.Label, o) {
			if b.OnClick != nil {
				b.OnClick()
			}
			if b.Closes {
				closeFn()
			}
		}
	}
}

// Confirm opens a Cancel / OK dialog; onOK runs when OK is pressed.
func (k *Kit) Confirm(id, title, body, okLabel string, danger bool, onOK func()) {
	const w = 460
	lines := gfx.WrapPx(body, gfx.Body, w-2*ModalPad)
	h := float32(ModalTitleH+ModalFooterH+2*ModalPad) + float32(len(lines))*float32(gfx.LineH(gfx.Body))
	style := BtnPrimary
	if danger {
		style = BtnDanger
	}
	k.Push(Modal{
		ID: id, Title: title, W: w, H: h,
		Draw: func(body gfx.Rect) {
			y := body.Y
			for _, l := range lines {
				k.Label(body.X, y, l, gfx.Body)
				y += float32(gfx.LineH(gfx.Body))
			}
		},
		Buttons: []ModalButton{
			{ID: "ok", Label: okLabel, Style: style, Closes: true, OnClick: onOK},
			{ID: "cancel", Label: "Cancel", Closes: true},
		},
	})
}
