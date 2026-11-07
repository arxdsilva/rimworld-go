package uikit

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Tooltips (NG-1 §1.4): a card that shows after the cursor has rested on
// a widget with a tip for tipDelay frames (0.4 s). TipMaxW (268) px wide,
// wrapped, kept inside the screen. A tip is "Title\nbody": both in Body,
// the title coloured, one blank line between them (UIS-015); a single line
// is a title.

const (
	tipPadX   = 4  // including the 1 px border: the text is TipMaxW-2*tipPadX wide
	tipPadY   = 7  // the height is a line pitch per line plus 2*tipPadY
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
	inner := float64(TipMaxW - 2*tipPadX)
	c := TipCard{Title: gfx.WrapPx(title, gfx.Body, inner)}
	if strings.TrimSpace(body) != "" {
		c.Body = gfx.WrapPx(body, gfx.Body, inner)
	}
	var w, h float64
	for _, l := range c.Title {
		lw, _ := gfx.Measure(l, gfx.Body)
		w = max(w, lw)
		h += gfx.LineH(gfx.Body)
	}
	for _, l := range c.Body {
		lw, _ := gfx.Measure(l, gfx.Body)
		w = max(w, lw)
		h += gfx.LineH(gfx.Body)
	}
	if len(c.Body) > 0 {
		h += gfx.LineH(gfx.Body) // the blank line between title and body
	}
	bw, bh := float32(w)+2*tipPadX, float32(h)+2*tipPadY
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
	return k.tip.text != "" && k.TipFrames >= k.Access.TipFrames()
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
	y := float64(c.Box.Y + tipPadY)
	x := float64(c.Box.X + tipPadX)
	ts := gfx.Body
	ts.Color = Heading
	for _, l := range c.Title {
		gfx.Text(dst, l, x, y, ts)
		y += gfx.LineH(ts)
	}
	if len(c.Body) > 0 {
		y += gfx.LineH(gfx.Body)
		bs := gfx.Body
		bs.Color = Text
		for _, l := range c.Body {
			gfx.Text(dst, l, x, y, bs)
			y += gfx.LineH(bs)
		}
	}
}
