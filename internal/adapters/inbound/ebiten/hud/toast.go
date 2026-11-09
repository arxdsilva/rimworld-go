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
//
// While a background save is written (Env.Saving, CI-002) a pinned
// "Saving..." row holds the anchor and the toasts stack above it; once up
// it stays SavingMinFrames Updates, so a quick save does not flicker. A
// failure (Alarm) is a toast in the danger colour that stays longer.

// Toast is one notice and when it was posted.
type Toast struct {
	Text   string
	At     uint64 // Env.Updates when posted
	Danger bool   // a failure (Alarm): the danger colour, AlarmFrames
}

const (
	ToastFrames = 150 // about 2.5 s at 60 TPS
	AlarmFrames = 360 // a failure stays about 6 s
	// SavingMinFrames is the least the Saving row stays once shown.
	SavingMinFrames = 45
	toastMax        = 3
	toastH          = 26
	toastStride     = toastH + 4
	toastPadX       = 14
	// menuToastBottom is how far above the screen's bottom a menu toast ends.
	menuToastBottom = 84
)

// Toast posts a notice.
func (c *Chrome) Toast(msg string) { c.post(Toast{Text: msg}) }

// Alarm posts a failure, drawn in the danger colour (a background save
// that could not be written).
func (c *Chrome) Alarm(msg string) { c.post(Toast{Text: msg, Danger: true}) }

func (c *Chrome) post(t Toast) {
	t.At = c.sh.Env().Updates
	c.toasts = append(c.toasts, t)
	if len(c.toasts) > toastMax {
		c.toasts = c.toasts[len(c.toasts)-toastMax:]
	}
}

// LiveToasts are the notices still showing.
func (c *Chrome) LiveToasts() []Toast {
	var out []Toast
	now := c.sh.Env().Updates
	for _, t := range c.toasts {
		life := uint64(ToastFrames)
		if t.Danger {
			life = AlarmFrames
		}
		if now-t.At < life {
			out = append(out, t)
		}
	}
	return out
}

// savingState is when the Saving row appeared.
type savingState struct {
	on    bool
	since uint64 // Env.Updates
}

// SavingShown reports the Saving row is up: while Env.Saving, and for
// SavingMinFrames Updates from when it appeared. It reads only the Env.
func (c *Chrome) SavingShown() bool {
	env := c.sh.Env()
	if env.Saving {
		if !c.saving.on {
			c.saving = savingState{on: true, since: env.Updates}
		}
		return true
	}
	if c.saving.on && env.Updates-c.saving.since < SavingMinFrames {
		return true
	}
	c.saving.on = false
	return false
}

// SavingRect is the Saving row's rect at the toast anchor, when it is up.
func (c *Chrome) SavingRect(sw, sh int) (gfx.Rect, bool) {
	if !c.SavingShown() {
		return gfx.Rect{}, false
	}
	x0, bottom, colony := c.toastAnchor(sh)
	return toastRect(c.sh.T("Saving..."), x0, bottom, colony, sw), true
}

// toastAnchor is where the bottom row ends: the hint anchor in the colony
// (above the hint card when it is up), bottom-centre elsewhere.
func (c *Chrome) toastAnchor(sh int) (x0, bottom float32, colony bool) {
	env := c.sh.Env()
	if !env.Colony {
		return 0, float32(sh - menuToastBottom), false
	}
	l := c.sh.Layout()
	x0, bottom = l.HintX, l.HintBottom
	if env.Hint { // the hint card owns the anchor
		bottom -= gfx.LineStep(gfx.Body) + 4 + hudlayout.HudGap
	}
	return x0, bottom, true
}

// toastRect is one row's rect ending at bottom, kept on screen.
func toastRect(text string, x0, bottom float32, colony bool, sw int) gfx.Rect {
	w := gfx.TextW(text, gfx.Body) + 2*toastPadX
	x := x0
	if !colony {
		x = (float32(sw) - w) / 2
	}
	x = max(min(x, float32(sw)-hudlayout.HudMargin-w), hudlayout.HudMargin)
	return gfx.Rect{X: x, Y: bottom - toastH, W: w, H: toastH}
}

// ToastRects are the live toasts' rects, newest first: the newest ends at
// the anchor and the older ones stack above it.
func (c *Chrome) ToastRects(sw, sh int) []gfx.Rect {
	ts := c.LiveToasts()
	if len(ts) == 0 {
		return nil
	}
	x0, bottom, colony := c.toastAnchor(sh)
	if c.SavingShown() { // the Saving row is pinned at the anchor
		bottom -= toastStride
	}
	out := make([]gfx.Rect, 0, len(ts))
	for i := len(ts) - 1; i >= 0; i-- {
		out = append(out, toastRect(ts[i].Text, x0, bottom, colony, sw))
		bottom -= toastStride
	}
	return out
}

// DrawToasts draws the Saving row and stacks the live notices above it at
// the hint anchor.
func (c *Chrome) DrawToasts(screen *ebiten.Image, sw, sh int) {
	if r, ok := c.SavingRect(sw, sh); ok {
		uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
		gfx.Text(screen, c.sh.T("Saving..."), float64(r.X+toastPadX), gfx.CenterTop(float64(r.Y), toastH, gfx.Body), gfx.WithColor(gfx.Body, uikit.TextDim))
	}
	ts := c.LiveToasts()
	for j, r := range c.ToastRects(sw, sh) {
		uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
		t, st := ts[len(ts)-1-j], gfx.Body
		if t.Danger {
			st = gfx.WithColor(st, uikit.Danger)
		}
		gfx.Text(screen, t.Text, float64(r.X+toastPadX), gfx.CenterTop(float64(r.Y), toastH, gfx.Body), st)
	}
}
