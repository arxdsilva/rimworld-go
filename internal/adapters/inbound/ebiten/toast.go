package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Toasts (UI-016): short notices such as "Screenshot saved" or a reserved
// shortcut's "not implemented yet", shown for a few seconds of Update
// frames. In the colony they stack upward from the hint anchor (PLAY-018:
// the layout's HintX / HintBottom, just above the command buttons or the
// pane), above a disabled-reason hint when one is up; off the colony they
// sit bottom-centre. Presentation only.

type toast struct {
	text string
	at   uint64 // App.updates when posted
}

const (
	toastFrames = 150 // about 2.5 s at 60 TPS
	toastMax    = 3
	toastH      = 26
	toastStride = toastH + 4
	toastPadX   = 14
	// menuToastBottom is how far above the screen's bottom a menu toast ends.
	menuToastBottom = 84
)

// toast posts a notice.
func (a *App) toast(msg string) {
	a.toasts = append(a.toasts, toast{text: msg, at: a.updates})
	if len(a.toasts) > toastMax {
		a.toasts = a.toasts[len(a.toasts)-toastMax:]
	}
}

// liveToasts are the notices still showing.
func (a *App) liveToasts() []toast {
	var out []toast
	for _, t := range a.toasts {
		if a.updates-t.at < toastFrames {
			out = append(out, t)
		}
	}
	return out
}

// toastRects are the live toasts' rects, newest first: the newest ends at
// the anchor and the older ones stack above it.
func (a *App) toastRects(sw, sh int) []rect {
	ts := a.liveToasts()
	if len(ts) == 0 {
		return nil
	}
	colony := a.nav.screen() == screenColony && a.renderer != nil
	var x0, bottom float32
	if colony {
		l := a.hud()
		x0, bottom = l.HintX, l.HintBottom
		if a.hint != "" { // the hint card owns the anchor
			bottom -= lineStep(tsBody) + 4 + hudGap
		}
	} else {
		bottom = float32(sh - menuToastBottom)
	}
	out := make([]rect, 0, len(ts))
	for i := len(ts) - 1; i >= 0; i-- {
		w := textW(ts[i].text, tsBody) + 2*toastPadX
		x := x0
		if !colony {
			x = (float32(sw) - w) / 2
		}
		x = max(min(x, float32(sw)-hudMargin-w), hudMargin)
		out = append(out, rect{X: x, Y: bottom - toastH, W: w, H: toastH})
		bottom -= toastStride
	}
	return out
}

// drawToasts stacks the live notices at the hint anchor.
func (a *App) drawToasts(screen *ebiten.Image, sw, sh int) {
	ts := a.liveToasts()
	for j, r := range a.toastRects(sw, sh) {
		drawPanel(screen, r.X, r.Y, r.W, r.H)
		drawText(screen, ts[len(ts)-1-j].text, float64(r.X+toastPadX), float64(r.Y)+(toastH-lineH(tsBody))/2, tsBody)
	}
}
