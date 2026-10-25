package ebiten

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Cave plants (MAPGEN-3 §1): duskmoss gives off a faint cool glow. It is
// purely presentational (the simulation's own light is game/light.go) and
// must never light a cave up: two soft discs, peak alpha duskGlowMax.
// Plants never get outlines.

const (
	duskmossSpecies = "duskmoss"
	duskGlowMax     = 0.10 // peak alpha of the inner disc at full growth
	duskGlowRadius  = 0.9  // outer disc, in tiles
)

var duskGlowClr = color.RGBA{R: 110, G: 160, B: 235, A: 255}

// duskGlowStrength is the glow of plant p, 0..1: none for other species
// and for a withering or dormant plant, growing with the plant.
func duskGlowStrength(p game.PlantView) float64 {
	if p.Species != duskmossSpecies || p.Wilted || p.Withering {
		return 0
	}
	return math.Min(math.Max(float64(p.Growth), 0), 100) / 100
}

// plantLightLines are the inspector lines about a plant's light needs
// (the game's PlantView flags NeedsDark, NeedsLight and Withering).
func plantLightLines(p game.PlantView) []string {
	var out []string
	switch {
	case p.NeedsDark:
		out = append(out, "Needs darkness")
	case p.NeedsLight:
		out = append(out, "Needs light to grow")
	}
	if p.Withering {
		out = append(out, "Withering in the light")
	}
	return out
}

// drawDuskGlow draws the glow of the duskmoss in view, after objects.
func (r *Renderer) drawDuskGlow(screen *ebiten.Image, w game.WorldView, tile float64) {
	if r.lod.tier != lodLive {
		return
	}
	pulse := 0.85 + 0.15*math.Sin(float64(r.present)*0.03)
	for _, cv := range r.lod.views {
		for _, i := range cv.Plants {
			if int(i) >= len(w.Plants) {
				continue
			}
			p := &w.Plants[i]
			k := duskGlowStrength(*p)
			if k <= 0 {
				continue
			}
			sx, sy := r.worldToScreenF(float64(p.X)+0.5, float64(p.Y)+0.5)
			vector.FillCircle(screen, sx, sy, float32(tile*duskGlowRadius), fade(duskGlowClr, duskGlowMax*0.4*k*pulse), true)
			vector.FillCircle(screen, sx, sy, float32(tile*duskGlowRadius*0.45), fade(duskGlowClr, duskGlowMax*k*pulse), true)
		}
	}
}
