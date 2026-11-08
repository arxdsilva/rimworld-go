package windows

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The pawn reward's chip and hover card (QST-137): the chip leads with a
// 16-px portrait; resting on it shows a card 214 px wide, 1 px under the
// chip with its left edge at the mouse, holding three blocks a line and a
// blank apart: "<name>, <role>", "Best skill: <skill> (level N)" with the
// label highlighted, and the backstory. The card replaces the tooltip.

const (
	pawnChipIcon   = 16 // the portrait in the chip
	pawnChipIconGo = 5  // the portrait -> the label
	pawnCardW      = 214
	pawnCardPadX   = 9
	pawnCardPadY   = 6
	pawnCardEdge   = 4 // screen edge clearance
)

// cardLine is one line of the card, relative to its box.
type cardLine struct {
	s    string
	x, y float32
	c    color.RGBA
}

// layoutPawnCard places card t for a chip at chip (screen) with the mouse
// at x mx, inside a sw x sh screen: under the chip, flipped above it when
// it would run off the bottom. Text wraps inside the card.
func layoutPawnCard(t pawnCardText, chip gfx.Rect, mx, sw, sh float32) (gfx.Rect, []cardLine) {
	st := gfx.Body
	lh := float32(gfx.LineH(st))
	inner := float32(pawnCardW - 2*pawnCardPadX)
	var lines []cardLine
	y := float32(pawnCardPadY)
	block := func(s string, c color.RGBA) {
		for _, l := range gfx.WrapPx(s, st, float64(inner)) {
			lines = append(lines, cardLine{l, pawnCardPadX, y, c})
			y += lh
		}
		y += lh // the blank line between blocks
	}
	block(t.head, uikit.Heading)
	lw := min(gfx.TextW(t.skillLabel, st), inner)
	lines = append(lines, cardLine{uikit.FitText(t.skillLabel, st, inner), pawnCardPadX, y, uikit.Accent})
	if rest := inner - lw - gfx.TextW(" ", st); rest > 0 {
		lines = append(lines, cardLine{uikit.FitText(t.skill, st, rest), pawnCardPadX + lw + gfx.TextW(" ", st), y, uikit.Text})
	}
	y += 2 * lh
	block(t.story, uikit.TextDim)
	h := y - lh + pawnCardPadY // no blank under the last block
	box := gfx.Rect{X: mx, Y: chip.Bottom() + 1, W: pawnCardW, H: h}
	if box.Bottom() > sh-pawnCardEdge {
		box.Y = chip.Y - 1 - h
	}
	box.X = max(pawnCardEdge, min(box.X, sw-pawnCardEdge-box.W))
	box.Y = max(pawnCardEdge, box.Y)
	return box, lines
}

// drawPawnCard paints the card.
func drawPawnCard(dst *ebiten.Image, box gfx.Rect, lines []cardLine) {
	uikit.FillRect(dst, box, uikit.Tooltip)
	uikit.StrokeRect(dst, box, uikit.TooltipEdge)
	for _, l := range lines {
		st := gfx.Body
		st.Color = l.c
		gfx.Text(dst, l.s, float64(box.X+l.x), float64(box.Y+l.y), st)
	}
}
