package uikit

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"

	"github.com/hajimehoshi/ebiten/v2"
)

// Layout system (NG-1 §1.3): a float rectangle with the splitting helpers
// the setup screens lay themselves out with, and the centred dialog frame.

// Setup-screen metrics (NG-1 §1.3).
const (
	dialogMaxW    = 1240
	dialogMaxH    = 760
	dialogMargin  = 80 // total, both sides
	dialogTitleH  = 44
	dialogFooterH = 56
	DialogPad     = 16
	FooterBtnH    = 36
	footerBackW   = 140
	footerNextW   = 160
	// Minimum window of the setup screens.
	MinSetupW = 1280
	MinSetupH = 720
)

// DialogFrame is the centred frame of the setup screens: width
// min(sw-80, 1240), height min(sh-80, 760), a 44 px title bar, a 56 px
// footer bar and the content between them with 16 px padding.
func DialogFrame(sw, sh int) (frame, title, content, footer gfx.Rect) {
	w := min(float32(sw-dialogMargin), dialogMaxW)
	h := min(float32(sh-dialogMargin), dialogMaxH)
	w, h = max(w, 0), max(h, 0)
	frame = gfx.WinRect(sw, sh).Centered(w, h)
	title, rest := frame.SplitV(dialogTitleH)
	body, footer := rest.SplitV(-dialogFooterH)
	return frame, title, body.Inset(DialogPad), footer
}

// FooterBack is the Back button's rectangle (bottom-left, 140x36).
func FooterBack(footer gfx.Rect) gfx.Rect {
	return gfx.Rect{X: footer.X + DialogPad, Y: footer.Y + (footer.H-FooterBtnH)/2, W: footerBackW, H: FooterBtnH}
}

// FooterNext is the forward button's rectangle (bottom-right, 160x36).
func FooterNext(footer gfx.Rect) gfx.Rect {
	return gfx.Rect{X: footer.Right() - DialogPad - footerNextW, Y: footer.Y + (footer.H-FooterBtnH)/2, W: footerNextW, H: FooterBtnH}
}

// TooSmall reports a window below the setup screens' minimum (1280x720).
func TooSmall(sw, sh int) bool { return sw < MinSetupW || sh < MinSetupH }

// DrawTooSmall dims the window and says it is too small for the screen.
func DrawTooSmall(dst *ebiten.Image, sw, sh int) {
	gfx.FillRect(dst, 0, 0, float32(sw), float32(sh), Dim, false)
	st := gfx.Large
	st.Align = gfx.AlignCenter
	cx, cy := float32(sw)/2, float32(sh)/2
	gfx.Text(dst, "Window too small", float64(cx), float64(cy)-gfx.LineH(st), st)
	sub := gfx.Body
	sub.Align = gfx.AlignCenter
	sub.Color = TextDim
	gfx.Text(dst, "Enlarge the window to at least 1280 x 720 or lower the UI scale.", float64(cx), float64(cy)+4, sub)
}

// AsButton is the rect as a button with id and label (the kit's type).
func AsButton(r gfx.Rect, id, label string) Button {
	return Button{ID: id, Label: label, X: r.X, Y: r.Y, W: r.W, H: r.H}
}
