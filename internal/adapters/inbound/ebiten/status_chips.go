package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/scenario"
)

// Status chips (NG-1 §2.11, §2.12): the heat wave or cold snap in progress
// and the scenario's permanent conditions (Ashfall sky, ...), each a small
// tag with a hover card, stacked down the right edge under the alerts. They
// read WorldView.Extreme and WorldView.Conditions; the sky's own tint is the
// light pass's. (The top strip has no room left for them.)

const (
	chipH   = 18
	chipPad = 7 // text padding in a chip
	chipGap = 4 // between stacked chips
	chipCh  = 6 // debug font advance
)

// statusChip is one tag.
type statusChip struct {
	label, tip string
	edge       color.RGBA
}

// statusChips lists the chips for a world view: the extreme weather first,
// then the permanent conditions in the scenario's order.
func statusChips(w game.WorldView) []statusChip {
	var out []statusChip
	switch w.Extreme {
	case "heat_wave":
		out = append(out, statusChip{"Heat wave", "Heat wave\nA spell of extreme heat: about 12 C hotter than the season, for a few days. Keep everyone out of the midday sun.", color.RGBA{R: 0xE0, G: 0x70, B: 0x3C, A: 0xFF}})
	case "cold_snap":
		out = append(out, statusChip{"Cold snap", "Cold snap\nA spell of extreme cold: about 14 C colder than the season, for a few days. Keep the fires fed.", color.RGBA{R: 0x6E, G: 0xA4, B: 0xE6, A: 0xFF}})
	}
	for _, key := range w.Conditions {
		for _, c := range scenario.Conditions() {
			if c.Key() != key {
				continue
			}
			out = append(out, statusChip{c.Label(), c.Label() + "\nThis land's permanent condition: " + c.Desc() + ".", uiBtnBorder})
		}
	}
	return out
}

// chipRects stacks the chips down the right edge, below the alerts.
func (a *App) chipRects(chips []statusChip, sw int) []rect {
	y := float32(hudTopH + hudPad)
	if !a.workOpen {
		y += float32(len(a.currentAlerts())) * (alertH + alertGap)
	}
	out := make([]rect, 0, len(chips))
	for _, c := range chips {
		w := float32(len(c.label)*chipCh + 2*chipPad)
		out = append(out, rect{float32(sw) - hudPad - w, y, w, chipH})
		y += chipH + chipGap
	}
	return out
}

// statusChipsUI declares the chips' hover cards.
func (a *App) statusChipsUI(sw int) {
	chips := statusChips(a.renderer.world())
	for i, r := range a.chipRects(chips, sw) {
		a.uiTip("hud.chip."+uiNum(i), r, chips[i].tip)
	}
}

// drawStatusChips paints the chips.
func (a *App) drawStatusChips(screen *ebiten.Image, sw int) {
	if a.workOpen {
		return // the Work tab covers the corner
	}
	chips := statusChips(a.renderer.world())
	for i, r := range a.chipRects(chips, sw) {
		fill := chips[i].edge
		fill.A = 0x38
		uiFillRect(screen, r.X, r.Y, r.W, r.H, fill, false)
		uiStrokeRect(screen, r.X, r.Y, r.W, r.H, 1, chips[i].edge, false)
		drawLabel(screen, int(r.X)+chipPad, int(r.Y)+2, chips[i].label)
	}
}
