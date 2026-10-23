package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Toasts (UI-016): short notices such as "Screenshot saved" or a reserved
// shortcut's "not implemented yet", shown above the colonist bar for a
// few seconds of Update frames. Presentation only.

type toast struct {
	text string
	at   uint64 // App.updates when posted
}

const (
	toastFrames = 150 // about 2.5 s at 60 TPS
	toastMax    = 3
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

// drawToasts stacks the live notices centred above the colonist bar.
func (a *App) drawToasts(screen *ebiten.Image, sw, sh int) {
	ts := a.liveToasts()
	y := int(hudBarY(sh)) - 34
	if a.nav.screen() != screenColony {
		y = sh - 110
	}
	for i := len(ts) - 1; i >= 0; i-- {
		w := int(textW(ts[i].text, tsBody)) + 28
		x := (sw - w) / 2
		drawPanel(screen, float32(x), float32(y), float32(w), 26)
		drawText(screen, ts[i].text, float64(x+14), float64(y)+(26-lineH(tsBody))/2, tsBody)
		y -= 30
	}
}
