package uikit

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Tooltips (NG-1 §1.4): a card that shows after the cursor has rested on
// a widget with a tip for tipDelay frames (0.4 s). Up to 360 px wide,
// wrapped, kept inside the screen. A tip is "Title\nbody": the title in
// tsLabel, the body in tsSmall; a single line is a title.

const (
	tipPad    = 8
	tipGap    = 2  // between title and body
	TipMargin = 4  // screen edge clearance
	tipOffX   = 14 // card offset from the cursor
	tipOffY   = 20
)

// TipCard is a laid-out tooltip.
type TipCard struct {
	Box   gfx.Rect
	Title []string
	Body  []string
}

// TipLayout wraps tip to TipMaxW and places the card by the cursor
// (mx, my): below and to the right, flipped above when it would run off
// the bottom, and clamped inside the sw x sh screen.
func TipLayout(tip string, sw, sh, mx, my int) TipCard {
	title, body, _ := strings.Cut(tip, "\n")
	inner := float64(TipMaxW - 2*tipPad)
	c := TipCard{Title: gfx.WrapPx(title, gfx.Label, inner)}
	if strings.TrimSpace(body) != "" {
		c.Body = gfx.WrapPx(body, gfx.Small, inner)
	}
	var w, h float64
	for _, l := range c.Title {
		lw, _ := gfx.Measure(l, gfx.Label)
		w = max(w, lw)
		h += gfx.LineH(gfx.Label)
	}
	for _, l := range c.Body {
		lw, _ := gfx.Measure(l, gfx.Small)
		w = max(w, lw)
		h += gfx.LineH(gfx.Small)
	}
	if len(c.Body) > 0 {
		h += tipGap
	}
	bw, bh := float32(w)+2*tipPad, float32(h)+2*tipPad
	x, y := float32(mx+tipOffX), float32(my+tipOffY)
	if x+bw > float32(sw-TipMargin) {
		x = float32(sw-TipMargin) - bw
	}
	if y+bh > float32(sh-TipMargin) {
		y = float32(my) - bh - 8
	}
	x = max(TipMargin, min(x, float32(sw-TipMargin)-bw))
	y = max(TipMargin, min(y, float32(sh-TipMargin)-bh))
	c.Box = gfx.Rect{X: x, Y: y, W: bw, H: bh}
	return c
}

// TipVisible reports the pending tooltip has waited long enough.
func (k *Kit) TipVisible() bool {
	return k.tip.text != "" && k.TipFrames >= TipDelay
}

// Tip makes r a hover area with a tooltip and nothing else.
func (k *Kit) Tip(id string, r gfx.Rect, tip string) {
	k.Add(Widget{ID: id, R: r, Tip: tip})
}

// paintTooltip draws the pending tooltip over everything.
func (k *Kit) paintTooltip(dst *ebiten.Image) {
	if !k.TipVisible() {
		return
	}
	sw, sh := k.host.Size()
	c := TipLayout(k.tip.text, sw, sh, k.mx, k.my)
	FillRect(dst, c.Box, Tooltip)
	StrokeRect(dst, c.Box, TooltipEdge)
	y := float64(c.Box.Y + tipPad)
	x := float64(c.Box.X + tipPad)
	ts := gfx.Label
	ts.Color = Heading
	for _, l := range c.Title {
		gfx.Text(dst, l, x, y, ts)
		y += gfx.LineH(ts)
	}
	if len(c.Body) > 0 {
		y += tipGap
		bs := gfx.Small
		bs.Color = TextDim
		for _, l := range c.Body {
			gfx.Text(dst, l, x, y, bs)
			y += gfx.LineH(bs)
		}
	}
}
