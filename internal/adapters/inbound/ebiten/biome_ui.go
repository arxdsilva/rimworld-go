package ebiten

import (
	"fmt"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/biome"
)

// Biome name and its tooltip (plan 1.5; the date row of the global controls
// shows the card, hud_global.go).

// seasonText is the climate label: "Boreal forest | Spring -2C outside". A
// world with no biome name (a stub) shows the season alone.
func seasonText(w game.WorldView) string {
	s := fmt.Sprintf("%s  %s outside", w.Season, fmtTemp(w.OutdoorTemp))
	if w.Biome != "" {
		s = w.Biome + " | " + s
	}
	return s
}

// biomeTooltip is the biome's hover card: the year's temperature range and
// the growing days.
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
