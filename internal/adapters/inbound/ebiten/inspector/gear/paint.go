package gear

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// paintRow draws one row into rr (dst is clipped to the scroll view): a
// heading with its rule, a line, a charge bar, or an item with its sprite,
// label, facts and tainted mark. textW is the room left of its buttons.
func paintRow(dst *ebiten.Image, r Row, rr gfx.Rect, img *ebiten.Image, textW float32, warn color.RGBA) {
	x, y := float64(rr.X), float64(rr.Y)
	switch {
	case r.kind == rowHead:
		gfx.Text(dst, r.Label, x, y+3, gfx.WithColor(gfx.Label, uikit.Heading))
		uikit.FillRect(dst, gfx.Rect{X: rr.X, Y: rr.Bottom() - 2, W: rr.W, H: 1}, uikit.BtnBorder)
	case r.kind == rowItem:
		paintItem(dst, r, rr, img, textW, warn)
	case r.Bar > 0:
		gfx.Text(dst, fmt.Sprintf("%s  %d%%", r.Label, r.Bar), x, y+2, gfx.Small)
		const lw = 100
		bar := gfx.Rect{X: rr.X + lw, Y: rr.Y + (rr.H-barH)/2, W: rr.W - lw - 4, H: barH}
		uikit.FillRect(dst, bar, uikit.SliderTrack)
		uikit.FillRect(dst, gfx.Rect{X: bar.X, Y: bar.Y, W: bar.W * float32(min(r.Bar, 100)) / 100, H: bar.H}, uikit.SliderFill)
	default:
		st := gfx.Small
		switch {
		case r.Warn:
			st = gfx.WithColor(st, warn)
		case r.Dim:
			st = gfx.WithColor(st, uikit.TextDim)
		}
		gfx.Text(dst, gfx.Ellipsize(r.Label, st, float64(textW)), x, y+1, st)
	}
}

func paintItem(dst *ebiten.Image, r Row, rr gfx.Rect, img *ebiten.Image, textW float32, warn color.RGBA) {
	if img != nil {
		gfx.Icon(dst, img, float64(rr.X)+1, float64(rr.Y+(rr.H-iconSz)/2), iconSz)
	}
	tx := float64(rr.X) + iconSz + 6
	ty := float64(rr.Y) + 2
	gfx.Text(dst, gfx.Ellipsize(r.Label, gfx.Small, float64(textW)), tx, ty, gfx.Small)
	ty += float64(gfx.LineStep(gfx.Small))
	dim := gfx.WithColor(gfx.Small, uikit.TextDim)
	facts := r.Facts
	if r.Forced {
		facts = joinFacts(facts, "locked")
	}
	if facts != "" {
		facts = gfx.Ellipsize(facts, dim, float64(textW))
		gfx.Text(dst, facts, tx, ty, dim)
	}
	if r.Tainted {
		fx := tx
		if facts != "" {
			fx += float64(gfx.TextW(facts+"   ", dim))
		}
		gfx.Text(dst, "tainted", fx, ty, gfx.WithColor(gfx.Small, warn))
	}
}

func joinFacts(a, b string) string {
	if a == "" {
		return b
	}
	return a + "   " + b
}
