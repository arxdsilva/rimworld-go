package ebiten

import (
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
	tipMargin = 4  // screen edge clearance
	tipOffX   = 14 // card offset from the cursor
	tipOffY   = 20
)

// tipCard is a laid-out tooltip.
type tipCard struct {
	Box   rect
	Title []string
	Body  []string
}

// tipLayout wraps tip to tipMaxW and places the card by the cursor
// (mx, my): below and to the right, flipped above when it would run off
// the bottom, and clamped inside the sw x sh screen.
func tipLayout(tip string, sw, sh, mx, my int) tipCard {
	title, body, _ := strings.Cut(tip, "\n")
	inner := float64(tipMaxW - 2*tipPad)
	c := tipCard{Title: wrapPx(title, tsLabel, inner)}
	if strings.TrimSpace(body) != "" {
		c.Body = wrapPx(body, tsSmall, inner)
	}
	var w, h float64
	for _, l := range c.Title {
		lw, _ := measure(l, tsLabel)
		w = max(w, lw)
		h += lineH(tsLabel)
	}
	for _, l := range c.Body {
		lw, _ := measure(l, tsSmall)
		w = max(w, lw)
		h += lineH(tsSmall)
	}
	if len(c.Body) > 0 {
		h += tipGap
	}
	bw, bh := float32(w)+2*tipPad, float32(h)+2*tipPad
	x, y := float32(mx+tipOffX), float32(my+tipOffY)
	if x+bw > float32(sw-tipMargin) {
		x = float32(sw-tipMargin) - bw
	}
	if y+bh > float32(sh-tipMargin) {
		y = float32(my) - bh - 8
	}
	x = max(tipMargin, min(x, float32(sw-tipMargin)-bw))
	y = max(tipMargin, min(y, float32(sh-tipMargin)-bh))
	c.Box = rect{x, y, bw, bh}
	return c
}

// tipVisible reports the pending tooltip has waited long enough.
func (u *uiState) tipVisible() bool {
	return u.tipText != "" && u.tipFrames >= tipDelay
}

// uiTip makes r a hover area with a tooltip and nothing else.
func (a *App) uiTip(id string, r rect, tip string) {
	a.uiSt().add(uiWidget{id: id, r: r, tip: tip})
}

// uiPaintTooltip draws the pending tooltip over everything.
func (a *App) uiPaintTooltip(dst *ebiten.Image) {
	u := a.uiSt()
	if !u.tipVisible() {
		return
	}
	sw, sh := a.size()
	c := tipLayout(u.tipText, sw, sh, u.mx, u.my)
	fillRect(dst, c.Box, uiTooltip)
	strokeRect(dst, c.Box, uiTooltipEdge)
	y := float64(c.Box.Y + tipPad)
	x := float64(c.Box.X + tipPad)
	ts := tsLabel
	ts.Color = uiHeading
	for _, l := range c.Title {
		drawText(dst, l, x, y, ts)
		y += lineH(ts)
	}
	if len(c.Body) > 0 {
		y += tipGap
		bs := tsSmall
		bs.Color = uiTextDim
		for _, l := range c.Body {
			drawText(dst, l, x, y, bs)
			y += lineH(bs)
		}
	}
}
