package learn

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// Row is a listed lesson and its rect.
type Row struct {
	L *Lesson
	R gfx.Rect
}

// Layout is where the helper's parts are this frame.
type Layout struct {
	Panel, Header, Footer gfx.Rect
	Rows                  []Row
	Open                  *Lesson // the lesson whose card is up (nil: none)
	Card, GotIt           gfx.Rect
	Body, Goal            []string // the card's wrapped text
}

// Height is the helper's height the layout reserves: 0 hidden, the header
// alone collapsed, else the header, up to three lessons and the footer. It
// must not call Host.Layout.
func (h *Helper) Height() float32 {
	if !h.visible() {
		return 0
	}
	if h.sh.Settings().Gameplay.LearnCollapsed {
		return learnHeaderH
	}
	n := min(learnShown, h.count())
	return learnHeaderH + learnPanelPad + float32(n)*(learnRowH+learnRowGap) + learnFooterH
}

// Folded reports the helper shows its header alone: the player folded it,
// or the layout's Learn rect p has no room for the lessons (a short screen
// with the minimap up).
func (h *Helper) Folded(p gfx.Rect) bool {
	return h.sh.Settings().Gameplay.LearnCollapsed || p.H < h.Height()-0.01
}

// Layout places the panel's parts inside the layout's Learn rect and the
// detail card left of it.
func (h *Helper) Layout() Layout {
	var lay Layout
	if colony, _ := h.sh.Colony(); colony == nil {
		return lay
	}
	hl := h.sh.Layout()
	p := hl.Learn
	if p.Empty() {
		return lay
	}
	lay.Panel = p
	lay.Header = gfx.Rect{X: p.X, Y: p.Y, W: p.W, H: min(learnHeaderH, p.H)}
	if h.Folded(p) {
		return lay
	}
	lay.Footer = gfx.Rect{X: p.X, Y: p.Bottom() - learnFooterH, W: p.W, H: learnFooterH}
	left := h.left()
	var openRow gfx.Rect
	for i := 0; i < min(learnShown, len(left)); i++ {
		r := gfx.Rect{X: p.X + learnPanelPad, Y: p.Y + learnHeaderH + learnPanelPad + float32(i)*(learnRowH+learnRowGap), W: learnRowW, H: learnRowH}
		lay.Rows = append(lay.Rows, Row{left[i], r})
		if left[i].ID == h.st.open {
			lay.Open, openRow = left[i], r
		}
	}
	if lay.Open != nil {
		h.CardLayout(&lay, hl, openRow)
	}
	return lay
}

// CardLayout places the detail card of lay.Open left of the panel, level
// with its row, clear of the colonist bar and the tab strip, and wraps its
// text.
func (h *Helper) CardLayout(lay *Layout, hl hudlayout.Layout, row gfx.Rect) {
	p := lay.Panel
	w := min(float32(learnCardW), p.X-hudlayout.HudGap-hudlayout.HudMargin)
	if w < 2*learnCardPad+80 {
		lay.Open = nil // no room beside the panel
		return
	}
	inner := float64(w - 2*learnCardPad)
	lay.Body = gfx.WrapPx(lay.Open.Body(h.sh.KeyName), gfx.Body, inner)
	lay.Goal = gfx.WrapPx("Try it: "+lay.Open.Goal, gfx.Small, inner)
	hgt := cardHeight(lay)
	maxH := hl.Tabs.Y - 2*hudlayout.HudMargin
	for hgt > maxH && len(lay.Body) > 1 { // a very short screen: cut the text
		lay.Body = lay.Body[:len(lay.Body)-1]
		last := &lay.Body[len(lay.Body)-1]
		*last = strings.TrimRight(*last, " .,;:") + "…"
		hgt = cardHeight(lay)
	}
	x := p.X - hudlayout.HudGap - w
	y := row.Y
	if !hl.Bar.Empty() && x < hl.Bar.Right() && x+w > hl.Bar.X {
		y = max(y, hl.Bar.Bottom()+hudlayout.HudGap)
	}
	y = max(min(y, hl.Tabs.Y-hudlayout.HudMargin-hgt), hudlayout.HudMargin)
	lay.Card = gfx.Rect{X: x, Y: y, W: w, H: hgt}
	lay.GotIt = gfx.Rect{X: x + w - learnCardPad - learnGotItW, Y: y + hgt - learnCardPad - learnGotItH, W: learnGotItW, H: learnGotItH}
}

// cardHeight is the card's height for its wrapped text.
func cardHeight(lay *Layout) float32 {
	return learnCardPad + gfx.LineStep(gfx.Label) + 6 + float32(len(lay.Body))*gfx.LineStep(gfx.Body) +
		8 + float32(len(lay.Goal))*gfx.LineStep(gfx.Small) + 10 + learnGotItH + learnCardPad
}

// Buttons are the helper's buttons, topmost first: the card's Got it, the
// lesson rows, the header toggle.
func (h *Helper) Buttons() []uikit.Button {
	lay := h.Layout()
	if lay.Panel.Empty() {
		return nil
	}
	var out []uikit.Button
	if lay.Open != nil && !lay.Card.Empty() {
		b := uikit.AsButton(lay.GotIt, "learn.done."+lay.Open.ID, "Got it")
		b.Tip = "Mark this lesson learned"
		out = append(out, b)
	}
	for _, rw := range lay.Rows {
		b := uikit.AsButton(rw.R, "learn.lesson."+rw.L.ID, rw.L.Title)
		b.Selected = lay.Open == rw.L
		b.Tip = "Read this lesson"
		out = append(out, b)
	}
	t := uikit.AsButton(lay.Header, "learn.toggle", "Learning helper")
	t.Tip = "Fold or unfold the lessons (Options › Gameplay turns them off)"
	return append(out, t)
}

// Chrome is the helper's area beyond the layout's Learn rect: the detail
// card, so a click on it never reaches the map.
func (h *Helper) Chrome() []gfx.Rect {
	lay := h.Layout()
	if lay.Open == nil || lay.Card.Empty() {
		return nil
	}
	return []gfx.Rect{lay.Card}
}
