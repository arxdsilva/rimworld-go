package infocard

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

const (
	// ID is the window's modal id; CloseID its close button; ScrollID its
	// scroll view.
	ID       = "colony.infocard"
	CloseID  = "infocard.close"
	ScrollID = "infocard.scroll"

	winW, winH = 440, 520
	labelW     = 130 // the label column
	gap        = 4
)

// Open shows the card for q over the map. A card already open is replaced,
// so a second request shows its own item. Esc and the close button close it
// (the kit's modal); nothing else on the screen is touched.
func (w *Window) Open(q ports.OpenInfoCard) {
	k := w.sh.Kit()
	k.CloseModal(ID)
	k.Push(uikit.Modal{
		ID: ID, Title: Build(w.sh, q).Title, W: winW, H: winH,
		Draw: func(body gfx.Rect) { w.body(q, body) },
		Buttons: []uikit.ModalButton{
			{ID: CloseID, Label: w.sh.T("Close"), Closes: true},
		},
	})
}

// Opened reports the card is on screen.
func (w *Window) Opened() bool {
	for _, m := range w.sh.Kit().Modals {
		if m.ID == ID {
			return true
		}
	}
	return false
}

// body declares the scroll view and draws the card in it, live, so the
// hit points and the pack follow the colony.
func (w *Window) body(q ports.OpenInfoCard, body gfx.Rect) {
	k := w.sh.Kit()
	c := Build(w.sh, q)
	ls, h := Layout(c, body.W-uikit.ScrollBarW)
	area := k.ScrollArea(ScrollID, body, h, uikit.ScrollOpts{})
	for _, l := range ls {
		y := l.Y - area.Off
		if y < 0 || y+l.H > body.H {
			continue // partly out of the view: the wheel brings it in
		}
		k.Label(body.X+l.X, body.Y+y, l.Text, l.style())
	}
}

// Line is one drawn line of the card, relative to the body's top left.
type Line struct {
	X, Y, H float32
	Text    string
	Kind    LineKind
}

// LineKind picks a line's style.
type LineKind uint8

const (
	KindTitle LineKind = iota
	KindDim
	KindBody
	KindHeading
	KindLabel
	KindValue
)

func (l Line) style() gfx.TextStyle {
	switch l.Kind {
	case KindTitle, KindHeading:
		return gfx.WithColor(gfx.Label, uikit.Heading)
	case KindDim, KindLabel:
		return uikit.DimText()
	}
	return gfx.Body
}

// Layout places the card's lines in width w and returns them with the
// content height. Long text wraps and rows stack; nothing is wider than w.
func Layout(c Card, w float32) ([]Line, float32) {
	var out []Line
	var y float32
	put := func(x float32, text string, kind LineKind, st gfx.TextStyle, wrap float32) {
		for _, t := range gfx.WrapPx(text, st, float64(wrap)) {
			h := float32(gfx.LineH(st))
			out = append(out, Line{X: x, Y: y, H: h, Text: t, Kind: kind})
			y += h
		}
	}
	put(0, c.Sub, KindDim, uikit.DimText(), w)
	y += gap * 2
	put(0, c.Desc, KindBody, gfx.Body, w)
	y += gap * 2
	for _, r := range c.Rows {
		if r.Value == "" {
			y += gap
			put(0, r.Label, KindHeading, gfx.Label, w)
			continue
		}
		top := y
		put(labelW, r.Value, KindValue, gfx.Body, w-labelW)
		bottom := y
		y = top
		put(0, gfx.Ellipsize(r.Label, uikit.DimText(), float64(labelW-gap)), KindLabel, uikit.DimText(), labelW)
		y = max(y, bottom) + 2
	}
	return out, y
}
