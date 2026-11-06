package hud

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/biome"
)

// Biome name and its tooltip (plan 1.5; the date row of the global controls
// shows the card, global.go).

// SeasonText is the climate label: "Boreal forest | Spring -2C outside". A
// world with no biome name (a stub) shows the season alone.
func SeasonText(w game.WorldView, u units.Temp) string { return SeasonTextIn(en, w, u) }

// SeasonTextIn is SeasonText in tr's language.
func SeasonTextIn(tr lang.Tr, w game.WorldView, u units.Temp) string {
	s := lang.Tf(tr, "%s  %s outside", w.Season, FmtTemp(u, w.OutdoorTemp))
	if w.Biome != "" {
		s = w.Biome + " | " + s
	}
	return s
}

// BiomeTooltip is the biome's hover card: the year's temperature range and
// the growing days.
func BiomeTooltip(w game.WorldView) []string { return BiomeTooltipIn(en, w) }

// BiomeTooltipIn is BiomeTooltip in tr's language.
func BiomeTooltipIn(tr lang.Tr, w game.WorldView) []string {
	id, ok := biome.Parse(w.BiomeKey)
	if !ok {
		return nil
	}
	b := game.BiomeInfoOf(id)
	lines := []string{
		b.Name,
		lang.Tf(tr, "Year: %dC winter low, %dC summer high", b.WinterLow, b.SummerHigh),
		lang.Tf(tr, "Growing days: about %d a year", b.GrowingDays),
	}
	if b.Difficulty != "" {
		lines = append(lines, lang.Tf(tr, "Difficulty: %s", b.Difficulty))
	}
	return lines
}
