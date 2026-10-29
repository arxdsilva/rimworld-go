package hud

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/scenario"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Conditions (NG-1 §2.11, §2.12; PLAY-018): the heat wave or cold snap in
// progress, the solar flare (PLAY-021) and the scenario's permanent
// conditions (Ashfall sky, ...), one line each in the Global controls,
// above the temperature, with the same hover cards the chips had. They read
// WorldView.Extreme, SolarFlare and Conditions; the sky's own tint is the
// light pass's.

// StatusChip is one condition: its label, its hover card and its colour.
type StatusChip struct {
	Label, Tip string
	Edge       color.RGBA
}

// StatusChips lists the conditions for a world view: the extreme weather
// first, then the solar flare, then the permanent conditions in the
// scenario's order.
func StatusChips(w game.WorldView) []StatusChip {
	var out []StatusChip
	switch w.Extreme {
	case "heat_wave":
		out = append(out, StatusChip{"Heat wave", "Heat wave\nA spell of extreme heat: about 12 C hotter than the season, for a few days. Keep everyone out of the midday sun.", uikit.Warning})
	case "cold_snap":
		out = append(out, StatusChip{"Cold snap", "Cold snap\nA spell of extreme cold: about 14 C colder than the season, for a few days. Keep the fires fed.", uikit.LetterInfo})
	}
	if w.SolarFlare {
		out = append(out, StatusChip{"Solar flare", "Solar flare\nA burst from the sun has knocked out every electrical device for a few hours. Batteries keep what they hold; torches and campfires still burn.", uikit.Warning})
	}
	for _, key := range w.Conditions {
		for _, c := range scenario.Conditions() {
			if c.Key() != key {
				continue
			}
			out = append(out, StatusChip{c.Label(), c.Label() + "\nThis land's permanent condition: " + c.Desc() + ".", uikit.TextDim})
		}
	}
	return out
}

// ChipRects are the condition lines' rects in the Global controls, the
// first on top, each as wide as its label and flush with Global's right
// edge (global.go lays them out).
func (c Global) ChipRects(chips []StatusChip, sw int) []gfx.Rect {
	if !c.live() {
		return nil
	}
	p := c.Plan()
	p.Chips = chips
	return LayoutGlobal(c.sh.Layout().Global, p, len(GlobalToggles)).Conds
}

// DeclareChips declares the conditions' hover cards.
func (c Global) DeclareChips(sw int) {
	chips := StatusChips(c.sh.World())
	for i, r := range c.ChipRects(chips, sw) {
		c.kit().Tip("hud.chip."+uikit.Num(i), r, chips[i].Tip)
	}
}

// drawConditionLines paints the conditions: each label right-aligned in its
// rect, in the condition's colour.
func drawConditionLines(screen *ebiten.Image, chips []StatusChip, rs []gfx.Rect) {
	for i, r := range rs {
		if i >= len(chips) {
			break
		}
		rightText(screen, chips[i].Label, r.Right()-GlobalPad, r, gfx.WithColor(gfx.Body, chips[i].Edge))
	}
}

// DrawChips paints the conditions.
func (c Global) DrawChips(screen *ebiten.Image, sw int) {
	chips := StatusChips(c.sh.World())
	drawConditionLines(screen, chips, c.ChipRects(chips, sw))
}
