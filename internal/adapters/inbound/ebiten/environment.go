package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Environment presentation (ART §27.3, §16) is render/env's (ARCH-004):
// wind, plants, contact springs, weather, ground cover, grass, crops,
// water, puddles, snow, tree canopies, cave plants and map features
// (env_compat.go forwards the renderer's old entry points). The root keeps
// the renderer's half: envState, the layer with the states kept beside it
// until their passes move, the roof-collapse dust and the autumn leaves
// (particles are the renderer's), and the HUD's weather readout.

// envState is the renderer's environment state: the env layer (its fields
// and methods promoted) and the shadow pass and overlay states kept with
// it.
type envState struct {
	env.Layer
	Shadow    shadowState        // the shadow pass (shadow.go)
	Roof      roofDrawState      // eaves and the Roof overlay (roof_draw.go)
	Home      homeDrawState      // the Home overlay's cached outline (home_draw.go)
	SnowClear snowClearDrawState // the snow clearing area overlay (snow_clear_draw.go)
	// collapses are the roof-collapse stamps already shown as dust.
	collapses map[game.RoofCollapseView]bool
	// paint is the sprite painter the layer draws with, refreshed from
	// Renderer.painter by envLayer (the renderer builds it per call).
	paint sprite.Painter
}

// stepCollapses puffs dust and debris once for every roof collapse stamp
// not seen before (SIM-018); stamps the view no longer lists are forgotten.
func (r *Renderer) stepCollapses(w game.WorldView) {
	e := &r.env
	if len(w.RoofCollapses) == 0 {
		e.collapses = nil
		return
	}
	seen := make(map[game.RoofCollapseView]bool, len(w.RoofCollapses))
	for _, c := range w.RoofCollapses {
		if !e.collapses[c] {
			seed := mix64(c.Tick ^ uint64(envTileKey(c.X, c.Y))<<20 ^ 0xc011)
			r.burst(partDust, float64(c.X)+0.5, float64(c.Y)+0.5, 12, seed)
			r.burst(partChip, float64(c.X)+0.5, float64(c.Y)+0.4, 4, seed^0x5a)
		}
		seen[c] = true
	}
	e.collapses = seen
}

var autumnLeaves = []color.RGBA{{214, 140, 52, 255}, {178, 74, 44, 255}, {226, 184, 74, 255}, {160, 104, 48, 255}}

// dropLeaves lets one or two autumn leaves fall from a brushed tree
// (env.Layer.Leaves lists them after a Step).
func (r *Renderer) dropLeaves(x, y int) {
	seed := tileHash(x, y) ^ r.present*0x9E3779B97F4A7C15
	n := 1 + int(hash01(seed, 7)*2)
	from := len(r.parts)
	r.burst(partLeaf, float64(x)+0.5, float64(y)-0.55, n, seed)
	for i := from; i < len(r.parts); i++ {
		r.parts[i].Clr = autumnLeaves[int(hash01(seed, uint64(i), 9)*float64(len(autumnLeaves)))%len(autumnLeaves)]
		r.parts[i].Life += 30 // leaves flutter down slowly
		r.parts[i].G *= 0.5
	}
}

// --- HUD ----------------------------------------------------------------------------

var windArrows = [8]string{"→", "↘", "↓", "↙", "←", "↖", "↑", "↗"}

// weatherLabel is the HUD weather readout, e.g. "Rain ↗ 2.1": the kind,
// an arrow the way the wind blows and its speed in tiles per second.
func weatherLabel(w game.WorldView) string {
	if w.Weather == "" {
		return ""
	}
	name := strings.ToUpper(w.Weather[:1]) + w.Weather[1:]
	s := math.Hypot(w.WindX, w.WindY)
	if s < 0.05 {
		return name + ", calm"
	}
	oct := int(math.Round(math.Atan2(w.WindY, w.WindX)/(math.Pi/4))) & 7
	return fmt.Sprintf("%s %s %.1f", name, windArrows[oct], s)
}

// weatherLabelW is the logical width drawWeatherLabel takes: the name, the
// arrow's room and the speed; a calm label is plain text.
func weatherLabelW(w game.WorldView) float32 {
	lbl := weatherLabel(w)
	if lbl == "" {
		return 0
	}
	if strings.HasSuffix(lbl, ", calm") {
		return textW(lbl, tsBody)
	}
	name, rest, _ := strings.Cut(lbl, " ")
	_, speed, _ := strings.Cut(rest, " ")
	return textW(name, tsBody) + 20 + textW(speed, tsBody)
}

// drawWeatherLabelRight is drawWeatherLabel with its right edge at x.
func drawWeatherLabelRight(screen *ebiten.Image, x, y int, w game.WorldView) {
	drawWeatherLabel(screen, x-int(weatherLabelW(w)+0.5), y, w)
}

// drawWeatherLabel draws weatherLabel at the logical position (x, y). The
// wind arrow is drawn as lines (the font has no arrows), pointing the true
// way, between the measured name and the speed. It is HUD, so every stroke
// goes through the canvas (uiLine): raw vector calls here ignored the UI
// scale and left the arrow behind at 1.5x and 2x (PLAY-017).
func drawWeatherLabel(screen *ebiten.Image, x, y int, w game.WorldView) {
	lbl := weatherLabel(w)
	if lbl == "" {
		return
	}
	if strings.HasSuffix(lbl, ", calm") {
		drawLabel(screen, x, y, lbl)
		return
	}
	name, rest, _ := strings.Cut(lbl, " ")
	_, speed, _ := strings.Cut(rest, " ")
	drawLabel(screen, x, y, name)
	nameW := textW(name, tsBody)
	cx, cy := float32(x)+nameW+10, float32(y+8)
	ux, uy := unit(w.WindX, w.WindY)
	fx, fy := float32(ux), float32(uy)
	clr := color.RGBA{230, 230, 220, 255}
	tipX, tipY := cx+fx*6, cy+fy*6
	uiLine(screen, cx-fx*6, cy-fy*6, tipX, tipY, 1.5, clr, true)
	for _, s := range [2]float32{1, -1} { // arrow head
		hx := tipX - fx*4 + s*-fy*3
		hy := tipY - fy*4 + s*fx*3
		uiLine(screen, tipX, tipY, hx, hy, 1.5, clr, true)
	}
	drawLabel(screen, x+int(nameW)+20, y, speed)
}
