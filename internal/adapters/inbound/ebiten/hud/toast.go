package hud

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Toasts (UI-016): short notices such as "Screenshot saved" or a reserved
// shortcut's "not implemented yet", shown for a few seconds of Update
// frames. In the colony they stack upward from the hint anchor (PLAY-018:
// the layout's HintX / HintBottom, just above the command buttons or the
// pane), above a disabled-reason hint when one is up; off the colony they
// sit bottom-centre. Presentation only.

// Toast is one notice and when it was posted.
type Toast struct {
	Text string
	At   uint64 // Env.Updates when posted
}

const (
	ToastFrames = 150 // about 2.5 s at 60 TPS
	toastMax    = 3
	toastH      = 26
	toastStride = toastH + 4
	toastPadX   = 14
	// menuToastBottom is how far above the screen's bottom a menu toast ends.
	menuToastBottom = 84
)

// Toast posts a notice.
func (c *Chrome) Toast(msg string) {
	c.toasts = append(c.toasts, Toast{Text: msg, At: c.sh.Env().Updates})
	if len(c.toasts) > toastMax {
		c.toasts = c.toasts[len(c.toasts)-toastMax:]
	}
}

// LiveToasts are the notices still showing.
func (c *Chrome) LiveToasts() []Toast {
	var out []Toast
	now := c.sh.Env().Updates
	for _, t := range c.toasts {
		if now-t.At < ToastFrames {
			out = append(out, t)
		}
	}
	return out
}

// ToastRects are the live toasts' rects, newest first: the newest ends at
// the anchor and the older ones stack above it.
func (c *Chrome) ToastRects(sw, sh int) []gfx.Rect {
	ts := c.LiveToasts()
	if len(ts) == 0 {
		return nil
	}
	env := c.sh.Env()
	colony := env.Colony
	var x0, bottom float32
	if colony {
		l := c.sh.Layout()
		x0, bottom = l.HintX, l.HintBottom
		if env.Hint { // the hint card owns the anchor
			bottom -= gfx.LineStep(gfx.Body) + 4 + hudlayout.HudGap
		}
	} else {
		bottom = float32(sh - menuToastBottom)
	}
	out := make([]gfx.Rect, 0, len(ts))
	for i := len(ts) - 1; i >= 0; i-- {
		w := gfx.TextW(ts[i].Text, gfx.Body) + 2*toastPadX
		x := x0
		if !colony {
			x = (float32(sw) - w) / 2
		}
		x = max(min(x, float32(sw)-hudlayout.HudMargin-w), hudlayout.HudMargin)
		out = append(out, gfx.Rect{X: x, Y: bottom - toastH, W: w, H: toastH})
		bottom -= toastStride
	}
	return out
}

// DrawToasts stacks the live notices at the hint anchor.
func (c *Chrome) DrawToasts(screen *ebiten.Image, sw, sh int) {
	ts := c.LiveToasts()
	for j, r := range c.ToastRects(sw, sh) {
		uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
		gfx.Text(screen, ts[len(ts)-1-j].Text, float64(r.X+toastPadX), float64(r.Y)+(toastH-gfx.LineH(gfx.Body))/2, gfx.Body)
	}
}
