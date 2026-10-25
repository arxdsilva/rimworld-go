package ebiten

import (
	"image"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Layout system (NG-1 §1.3): a float rectangle with the splitting helpers
// the setup screens lay themselves out with, and the centred dialog frame.

// rect is a screen rectangle in pixels.
type rect struct{ X, Y, W, H float32 }

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

// screenRect is the whole window.
func winRect(sw, sh int) rect { return rect{0, 0, float32(sw), float32(sh)} }

func (r rect) right() float32  { return r.X + r.W }
func (r rect) bottom() float32 { return r.Y + r.H }

// inset shrinks r by p on every side (never below zero size).
func (r rect) inset(p float32) rect { return r.insetXY(p, p) }

// insetXY shrinks r by px left and right and py top and bottom.
func (r rect) insetXY(px, py float32) rect {
	return rect{r.X + px, r.Y + py, max(0, r.W-2*px), max(0, r.H-2*py)}
}

// splitH cuts r into a left part of width at and the rest. A negative at
// cuts a right part of width -at instead (left is the rest).
func (r rect) splitH(at float32) (left, right rect) {
	if at < 0 {
		at = r.W + at
	}
	at = max(0, min(at, r.W))
	return rect{r.X, r.Y, at, r.H}, rect{r.X + at, r.Y, r.W - at, r.H}
}

// splitV cuts r into a top part of height at and the rest. A negative at
// cuts a bottom part of height -at instead.
func (r rect) splitV(at float32) (top, bottom rect) {
	if at < 0 {
		at = r.H + at
	}
	at = max(0, min(at, r.H))
	return rect{r.X, r.Y, r.W, at}, rect{r.X, r.Y + at, r.W, r.H - at}
}

// columns divides r into n equal columns with gap between them.
func (r rect) columns(n int, gap float32) []rect {
	if n < 1 {
		return nil
	}
	w := (r.W - gap*float32(n-1)) / float32(n)
	out := make([]rect, n)
	for i := range out {
		out[i] = rect{r.X + float32(i)*(w+gap), r.Y, max(0, w), r.H}
	}
	return out
}

// rows stacks rows of height h separated by gap, as many as fit in r.
func (r rect) rows(h, gap float32) []rect {
	if h <= 0 {
		return nil
	}
	var out []rect
	for y := r.Y; y+h <= r.Y+r.H+0.001; y += h + gap {
		out = append(out, rect{r.X, y, r.W, h})
	}
	return out
}

// rowsN is exactly n rows of height h separated by gap, overflowing r if
// they do not fit (a scroll view clips them).
func (r rect) rowsN(n int, h, gap float32) []rect {
	out := make([]rect, max(0, n))
	for i := range out {
		out[i] = rect{r.X, r.Y + float32(i)*(h+gap), r.W, h}
	}
	return out
}

// contains reports the point is inside r.
func (r rect) contains(x, y float32) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// hit is contains for an integer cursor.
func (r rect) hit(mx, my int) bool { return r.contains(float32(mx), float32(my)) }

// center is the middle point.
func (r rect) center() (x, y float32) { return r.X + r.W/2, r.Y + r.H/2 }

// centered is a w x h rectangle centred in r.
func (r rect) centered(w, h float32) rect {
	return rect{r.X + (r.W-w)/2, r.Y + (r.H-h)/2, w, h}
}

// intersect is the overlap of r and o (zero size when disjoint).
func (r rect) intersect(o rect) rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.right(), o.right()), min(r.bottom(), o.bottom())
	if x1 <= x0 || y1 <= y0 {
		return rect{x0, y0, 0, 0}
	}
	return rect{x0, y0, x1 - x0, y1 - y0}
}

// empty reports r has no area.
func (r rect) empty() bool { return r.W <= 0 || r.H <= 0 }

// pixels is r as an integer image rectangle (for SubImage clipping).
func (r rect) pixels() image.Rectangle {
	return image.Rect(int(r.X), int(r.Y), int(r.X+r.W+0.999), int(r.Y+r.H+0.999))
}

// asButton is r as a hit-list button.
func (r rect) asButton(id, label string) button {
	return button{id: id, label: label, x: r.X, y: r.Y, w: r.W, h: r.H}
}

// clipTo returns dst restricted to r, so drawing with absolute
// coordinates is clipped to it.
// r is logical; the clip is its physical rectangle.
func clipTo(dst *ebiten.Image, r rect) *ebiten.Image {
	b := uiPhysRect(r).pixels().Intersect(dst.Bounds())
	if b.Empty() {
		return dst.SubImage(image.Rect(0, 0, 0, 0)).(*ebiten.Image)
	}
	return dst.SubImage(b).(*ebiten.Image)
}

// dialogFrame is the centred frame of the setup screens: width
// min(sw-80, 1240), height min(sh-80, 760), a 44 px title bar, a 56 px
// footer bar and the content between them with 16 px padding.
func dialogFrame(sw, sh int) (frame, title, content, footer rect) {
	w := min(float32(sw-dialogMargin), dialogMaxW)
	h := min(float32(sh-dialogMargin), dialogMaxH)
	w, h = max(w, 0), max(h, 0)
	frame = winRect(sw, sh).centered(w, h)
	title, rest := frame.splitV(dialogTitleH)
	body, footer := rest.splitV(-dialogFooterH)
	return frame, title, body.inset(dialogPad), footer
}

// footerBack is the Back button's rectangle (bottom-left, 140x36).
func footerBack(footer rect) rect {
	return rect{footer.X + dialogPad, footer.Y + (footer.H-footerBtnH)/2, footerBackW, footerBtnH}
}

// footerNext is the forward button's rectangle (bottom-right, 160x36).
func footerNext(footer rect) rect {
	return rect{footer.right() - dialogPad - footerNextW, footer.Y + (footer.H-footerBtnH)/2, footerNextW, footerBtnH}
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
