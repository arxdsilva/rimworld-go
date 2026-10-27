package config

import "slices"

// PLAY-019: graphics quality from the hardware. At first launch (and once
// for settings files from before it) the game detects a tier from the
// machine's memory and cores and applies its preset; any row the player
// then changes by hand is remembered in Chosen and never overwritten.

// Quality presets.
const (
	QualityHigh   = "high"
	QualityMedium = "medium"
	QualityLow    = "low"
	QualityCustom = "custom" // a preset with rows the player changed
)

// QualityLevels is the Quality row's cycle (Custom is never picked).
var QualityLevels = []string{QualityHigh, QualityMedium, QualityLow}

// Water rendering.
const (
	WaterAnimated = "animated" // the ripple shader
	WaterSimple   = "simple"   // still water, as with reduced motion
)

// Grass detail.
const (
	GrassFull    = "full"
	GrassReduced = "reduced"
	GrassSparse  = "sparse"
)

// GrassLevels is the Grass row's cycle.
var GrassLevels = []string{GrassFull, GrassReduced, GrassSparse}

// RenderScales are the World detail row's cycle (PERF-002): the share of
// the physical pixels the map draws at. 1 is every pixel.
var RenderScales = []float64{1, 0.75, 0.5}

// GraphicsRows are the rows a preset sets, by their Chosen key.
var GraphicsRows = []string{"effects", "shadows", "lighting", "water", "grass", "hidpi", "vsync"}

// Hardware tier thresholds, from measured use (PLAY-019): a grown 500x500
// colony and the editor and OS together need about 8 GiB; High also needs
// cores for the simulation and the renderer side by side.
const (
	tierHighRAM   = 12 << 30
	tierHighCores = 6
	tierMidRAM    = 8 << 30
)

// DetectTier picks a quality tier from total memory (bytes; 0 unknown) and
// core count.
func DetectTier(total uint64, cores int) string {
	switch {
	case total == 0:
		return QualityMedium
	case total >= tierHighRAM && cores >= tierHighCores:
		return QualityHigh
	case total >= tierMidRAM:
		return QualityMedium
	}
	return QualityLow
}

// ApplyTier sets every row of preset tier that the player has not chosen
// by hand; devScale is the screen's device scale (HiDPI is dropped on
// Medium above 2x). Quality reads tier, or Custom when a chosen row kept
// another value.
func (g *Graphics) ApplyTier(tier string, devScale float64) {
	p := presetFor(tier, devScale)
	set := func(key string, apply func()) {
		if !g.chosen(key) {
			apply()
		}
	}
	set("effects", func() { g.Effects = p.Effects })
	set("shadows", func() { g.Shadows = p.Shadows })
	set("lighting", func() { g.Lighting = p.Lighting })
	set("water", func() { g.Water = p.Water })
	set("grass", func() { g.Grass = p.Grass })
	set("hidpi", func() { g.HiDPI = p.HiDPI })
	set("vsync", func() { g.VSync = p.VSync })
	g.Quality = tier
	if !g.matches(p) {
		g.Quality = QualityCustom
	}
}

// Choose records that the player set row key by hand (an Options change);
// the preset then reads Custom.
func (g *Graphics) Choose(key string) {
	if !g.chosen(key) {
		g.Chosen = append(g.Chosen, key)
	}
	g.Quality = QualityCustom
}

// MarkChosenFromOldDefaults treats every row a settings file from before
// PLAY-019 had changed from that version's defaults (full effects, vsync
// on, HiDPI on) as the player's choice.
func (g *Graphics) MarkChosenFromOldDefaults() {
	if g.Effects != EffectsFull {
		g.Chosen = appendNew(g.Chosen, "effects")
	}
	if !g.VSync {
		g.Chosen = appendNew(g.Chosen, "vsync")
	}
	if !g.HiDPI {
		g.Chosen = appendNew(g.Chosen, "hidpi")
	}
}

func (g Graphics) chosen(key string) bool { return slices.Contains(g.Chosen, key) }

func (g Graphics) matches(p Graphics) bool {
	return g.Effects == p.Effects && g.Shadows == p.Shadows && g.Lighting == p.Lighting &&
		g.Water == p.Water && g.Grass == p.Grass && g.HiDPI == p.HiDPI && g.VSync == p.VSync
}

// presetFor is tier's settings.
func presetFor(tier string, devScale float64) Graphics {
	switch tier {
	case QualityLow:
		return Graphics{Effects: EffectsMinimal, Water: WaterSimple, Grass: GrassSparse, VSync: true}
	case QualityMedium:
		return Graphics{Effects: EffectsReduced, Shadows: true, Lighting: true, Water: WaterSimple, Grass: GrassReduced,
			HiDPI: devScale <= 2, VSync: true}
	}
	return Graphics{Effects: EffectsFull, Shadows: true, Lighting: true, Water: WaterAnimated, Grass: GrassFull, HiDPI: true, VSync: true}
}

func appendNew(xs []string, x string) []string {
	if slices.Contains(xs, x) {
		return xs
	}
	return append(xs, x)
}

// normalizeGraphics resets unknown choices and keeps only known, distinct
// Chosen keys.
func (g Graphics) normalizeGraphics(d Graphics) Graphics {
	if !slices.Contains(QualityLevels, g.Quality) && g.Quality != QualityCustom {
		g.Quality = d.Quality
	}
	if g.Water != WaterAnimated && g.Water != WaterSimple {
		g.Water = d.Water
	}
	if !slices.Contains(GrassLevels, g.Grass) {
		g.Grass = d.Grass
	}
	if !slices.Contains(RenderScales, g.RenderScale) { // 0: a file from before PERF-002
		g.RenderScale = d.RenderScale
	}
	var chosen []string
	for _, k := range g.Chosen {
		if slices.Contains(GraphicsRows, k) && !slices.Contains(chosen, k) {
			chosen = append(chosen, k)
		}
	}
	g.Chosen = chosen
	return g
}
