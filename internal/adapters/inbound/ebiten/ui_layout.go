package ebiten

import (
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
	dialogPad     = 16
	footerBtnH    = 36
	footerBackW   = 140
	footerNextW   = 160
	// Minimum window of the setup screens.
	minSetupW = 1280
	minSetupH = 720
)

// dialogFrame is the centred frame of the setup screens: width
// min(sw-80, 1240), height min(sh-80, 760), a 44 px title bar, a 56 px
// footer bar and the content between them with 16 px padding.
func dialogFrame(sw, sh int) (frame, title, content, footer rect) {
	w := min(float32(sw-dialogMargin), dialogMaxW)
	h := min(float32(sh-dialogMargin), dialogMaxH)
	w, h = max(w, 0), max(h, 0)
	frame = winRect(sw, sh).Centered(w, h)
	title, rest := frame.SplitV(dialogTitleH)
	body, footer := rest.SplitV(-dialogFooterH)
	return frame, title, body.Inset(dialogPad), footer
}

// footerBack is the Back button's rectangle (bottom-left, 140x36).
func footerBack(footer rect) rect {
	return rect{X: footer.X + dialogPad, Y: footer.Y + (footer.H-footerBtnH)/2, W: footerBackW, H: footerBtnH}
}

// footerNext is the forward button's rectangle (bottom-right, 160x36).
func footerNext(footer rect) rect {
	return rect{X: footer.Right() - dialogPad - footerNextW, Y: footer.Y + (footer.H-footerBtnH)/2, W: footerNextW, H: footerBtnH}
}

// tooSmall reports a window below the setup screens' minimum (1280x720).
func tooSmall(sw, sh int) bool { return sw < minSetupW || sh < minSetupH }

// drawTooSmall dims the window and says it is too small for the screen.
func drawTooSmall(dst *ebiten.Image, sw, sh int) {
	uiFillRect(dst, 0, 0, float32(sw), float32(sh), uiDim, false)
	st := tsLarge
	st.Align = alignCenter
	cx, cy := float32(sw)/2, float32(sh)/2
	drawText(dst, "Window too small", float64(cx), float64(cy)-lineH(st), st)
	sub := tsBody
	sub.Align = alignCenter
	sub.Color = uiTextDim
	drawText(dst, "Enlarge the window to at least 1280 x 720 or lower the UI scale.", float64(cx), float64(cy)+4, sub)
}

// asButton is the rect as a button with id and label (the kit's type).
func asButton(r rect, id, label string) button {
	return button{id: id, label: label, x: r.X, y: r.Y, w: r.W, h: r.H}
}
