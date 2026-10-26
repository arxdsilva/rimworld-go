package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/scenario"
)

// Conditions (NG-1 §2.11, §2.12; PLAY-018): the heat wave or cold snap in
// progress and the scenario's permanent conditions (Ashfall sky, ...), one
// line each in the Global controls, above the temperature, with the same
// hover cards the chips had. They read WorldView.Extreme and
// WorldView.Conditions; the sky's own tint is the light pass's.

// statusChip is one condition: its label, its hover card and its colour.
type statusChip struct {
	label, tip string
	edge       color.RGBA
}

// statusChips lists the conditions for a world view: the extreme weather
// first, then the permanent conditions in the scenario's order.
func statusChips(w game.WorldView) []statusChip {
	var out []statusChip
	switch w.Extreme {
	case "heat_wave":
		out = append(out, statusChip{"Heat wave", "Heat wave\nA spell of extreme heat: about 12 C hotter than the season, for a few days. Keep everyone out of the midday sun.", uiWarning})
	case "cold_snap":
		out = append(out, statusChip{"Cold snap", "Cold snap\nA spell of extreme cold: about 14 C colder than the season, for a few days. Keep the fires fed.", uiLetterInfo})
	}
	for _, key := range w.Conditions {
		for _, c := range scenario.Conditions() {
			if c.Key() != key {
				continue
			}
			out = append(out, statusChip{c.Label(), c.Label() + "\nThis land's permanent condition: " + c.Desc() + ".", uiTextDim})
		}
	}
	return out
}

// chipRects are the condition lines' rects in the Global controls, the
// first on top, each as wide as its label and flush with Global's right
// edge (hud_global.go lays them out).
func (a *App) chipRects(chips []statusChip, sw int) []rect {
	if a.renderer == nil {
		return nil
	}
	p := a.globalPlan()
	p.chips = chips
	return layoutGlobal(a.hud().Global, p, len(globalToggleList)).Conds
}

// statusChipsUI declares the conditions' hover cards.
func (a *App) statusChipsUI(sw int) {
	chips := statusChips(a.renderer.world())
	for i, r := range a.chipRects(chips, sw) {
		a.uiTip("hud.chip."+uiNum(i), r, chips[i].tip)
	}
}

// drawConditionLines paints the conditions: each label right-aligned in its
// rect, in the condition's colour.
func drawConditionLines(screen *ebiten.Image, chips []statusChip, rs []rect) {
	for i, r := range rs {
		if i >= len(chips) {
			break
		}
		rightText(screen, chips[i].label, r.right()-globalPad, r, withColor(tsBody, chips[i].edge))
	}
}

// drawStatusChips paints the conditions.
func (a *App) drawStatusChips(screen *ebiten.Image, sw int) {
	chips := statusChips(a.renderer.world())
	drawConditionLines(screen, chips, a.chipRects(chips, sw))
}
