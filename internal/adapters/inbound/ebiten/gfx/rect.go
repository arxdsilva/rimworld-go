package gfx

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// Rect is a screen rectangle in pixels.
type Rect struct{ X, Y, W, H float32 }

// screenRect is the whole window.
func WinRect(sw, sh int) Rect { return Rect{0, 0, float32(sw), float32(sh)} }

func (r Rect) Right() float32 { return r.X + r.W }

func (r Rect) Bottom() float32 { return r.Y + r.H }

// Inset shrinks r by p on every side (never below zero size).
func (r Rect) Inset(p float32) Rect { return r.InsetXY(p, p) }

// InsetXY shrinks r by px left and right and py top and bottom.
func (r Rect) InsetXY(px, py float32) Rect {
	return Rect{r.X + px, r.Y + py, max(0, r.W-2*px), max(0, r.H-2*py)}
}

// SplitH cuts r into a left part of width at and the rest. A negative at
// cuts a right part of width -at instead (left is the rest).
func (r Rect) SplitH(at float32) (left, right Rect) {
	if at < 0 {
		at = r.W + at
	}
	at = max(0, min(at, r.W))
	return Rect{r.X, r.Y, at, r.H}, Rect{r.X + at, r.Y, r.W - at, r.H}
}

// SplitV cuts r into a top part of height at and the rest. A negative at
// cuts a bottom part of height -at instead.
func (r Rect) SplitV(at float32) (top, bottom Rect) {
	if at < 0 {
		at = r.H + at
	}
	at = max(0, min(at, r.H))
	return Rect{r.X, r.Y, r.W, at}, Rect{r.X, r.Y + at, r.W, r.H - at}
}

// Columns divides r into n equal Columns with gap between them.
func (r Rect) Columns(n int, gap float32) []Rect {
	if n < 1 {
		return nil
	}
	w := (r.W - gap*float32(n-1)) / float32(n)
	out := make([]Rect, n)
	for i := range out {
		out[i] = Rect{r.X + float32(i)*(w+gap), r.Y, max(0, w), r.H}
	}
	return out
}

// Rows stacks Rows of height h separated by gap, as many as fit in r.
func (r Rect) Rows(h, gap float32) []Rect {
	if h <= 0 {
		return nil
	}
	var out []Rect
	for y := r.Y; y+h <= r.Y+r.H+0.001; y += h + gap {
		out = append(out, Rect{r.X, y, r.W, h})
	}
	return out
}

// RowsN is exactly n rows of height h separated by gap, overflowing r if
// they do not fit (a scroll view clips them).
func (r Rect) RowsN(n int, h, gap float32) []Rect {
	out := make([]Rect, max(0, n))
	for i := range out {
		out[i] = Rect{r.X, r.Y + float32(i)*(h+gap), r.W, h}
	}
	return out
}

// Contains reports the point is inside r.
func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Hit is contains for an integer cursor.
func (r Rect) Hit(mx, my int) bool { return r.Contains(float32(mx), float32(my)) }

// Center is the middle point.
func (r Rect) Center() (x, y float32) { return r.X + r.W/2, r.Y + r.H/2 }

// Centered is a w x h rectangle centred in r.
func (r Rect) Centered(w, h float32) Rect {
	return Rect{r.X + (r.W-w)/2, r.Y + (r.H-h)/2, w, h}
}

// Intersect is the overlap of r and o (zero size when disjoint).
func (r Rect) Intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.Right(), o.Right()), min(r.Bottom(), o.Bottom())
	if x1 <= x0 || y1 <= y0 {
		return Rect{x0, y0, 0, 0}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Empty reports r has no area.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Pixels is r as an integer image rectangle (for SubImage clipping).
func (r Rect) Pixels() image.Rectangle {
	return image.Rect(int(r.X), int(r.Y), int(r.X+r.W+0.999), int(r.Y+r.H+0.999))
}

// ClipTo returns dst restricted to r, so drawing with absolute
// coordinates is clipped to it.
// r is logical; the clip is its physical rectangle.
func ClipTo(dst *ebiten.Image, r Rect) *ebiten.Image {
	b := PhysRect(r).Pixels().Intersect(dst.Bounds())
	if b.Empty() {
		return dst.SubImage(image.Rect(0, 0, 0, 0)).(*ebiten.Image)
	}
	return dst.SubImage(b).(*ebiten.Image)
}
