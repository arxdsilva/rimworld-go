package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/biome"
)

// Top bar biome name and its tooltip (plan 1.5).

// seasonText is the top bar's climate label: "Boreal forest | Spring -2C
// outside". A world with no biome name (a stub) shows the season alone.
func seasonText(w game.WorldView) string {
	s := fmt.Sprintf("%s  %s outside", w.Season, fmtTemp(w.OutdoorTemp))
	if w.Biome != "" {
		s = w.Biome + " | " + s
	}
	return s
}

// biomeTooltip is the hover card of the top bar's biome label: the
// year's temperature range and the growing days.
func biomeTooltip(w game.WorldView) []string {
	id, ok := biome.Parse(w.BiomeKey)
	if !ok {
		return nil
	}
	b := game.BiomeInfoOf(id)
	lines := []string{
		b.Name,
		fmt.Sprintf("Year: %dC winter low, %dC summer high", b.WinterLow, b.SummerHigh),
		fmt.Sprintf("Growing days: about %d a year", b.GrowingDays),
	}
	if b.Difficulty != "" {
		lines = append(lines, "Difficulty: "+b.Difficulty)
	}
	return lines
}

// drawBiomeTooltip shows the biome card while the cursor is on the
// season label (x..x+width, in the top bar).
func (a *App) drawBiomeTooltip(screen *ebiten.Image, x, width int) {
	mx, my := a.currentCursor()
	if my < 0 || my >= hudTopH || mx < x || mx >= x+width {
		return
	}
	lines := biomeTooltip(a.renderer.world())
	if len(lines) == 0 {
		return
	}
	wid := 0
	for _, l := range lines {
		wid = max(wid, len(l)*6)
	}
	drawPanel(screen, float32(x), hudTopH+4, float32(wid+16), float32(len(lines)*16+12))
	for i, l := range lines {
		drawLabel(screen, x+8, hudTopH+10+i*16, l)
	}
}
