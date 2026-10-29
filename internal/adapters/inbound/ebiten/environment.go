package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	envpkg "rimworld-go/internal/adapters/inbound/ebiten/render/env"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/application/game"
)

// Environment presentation (ART §27.3, §16) is render/env's (ARCH-004):
// wind, plants, contact springs, weather, ground cover, grass, crops,
// water, puddles, snow, tree canopies, cave plants and map features
// (env_host.go steps it and serves it). The root keeps the renderer's half:
// envState, the layer with the states kept beside it until their passes
// move, the roof-collapse dust and the autumn leaves (particles of the
// creature layer), and the HUD's weather readout.

// envState is the renderer's environment state: the env layer (its fields
// and methods promoted) and the shadow pass and overlay states kept with
// it.
type envState struct {
	env.Layer
	Shadow    light.Shadows[creature.FigurePose] // the shadow pass (render/light; its casters: shadow.go)
	Roof      overlaypkg.RoofDrawState           // eaves and the Roof overlay (roof_draw.go)
	Home      overlaypkg.HomeDrawState           // the Home overlay's cached outline (home_draw.go)
	SnowClear overlaypkg.SnowClearDrawState      // the snow clearing area overlay (snow_clear_draw.go)
	// collapses are the roof-collapse stamps already shown as dust.
	collapses map[game.RoofCollapseView]bool
}

// stepCollapses puffs dust and debris once for every roof collapse stamp
// not seen before (SIM-018); stamps the view no longer lists are forgotten.
func (e *envState) stepCollapses(c *creature.Set, v *render.View, w game.WorldView) {
	if len(w.RoofCollapses) == 0 {
		e.collapses = nil
		return
	}
	seen := make(map[game.RoofCollapseView]bool, len(w.RoofCollapses))
	for _, rc := range w.RoofCollapses {
		if !e.collapses[rc] {
			seed := art.Mix64(rc.Tick ^ uint64(envpkg.TileKey(rc.X, rc.Y))<<20 ^ 0xc011)
			c.Burst(v, creature.PartDust, float64(rc.X)+0.5, float64(rc.Y)+0.5, 12, seed)
			c.Burst(v, creature.PartChip, float64(rc.X)+0.5, float64(rc.Y)+0.4, 4, seed^0x5a)
		}
		seen[rc] = true
	}
	e.collapses = seen
}

var autumnLeaves = []color.RGBA{{214, 140, 52, 255}, {178, 74, 44, 255}, {226, 184, 74, 255}, {160, 104, 48, 255}}

// dropLeaves lets one or two autumn leaves fall from a brushed tree at
// (x, y) (env.Layer.Leaves lists them after a Step) as particles of c.
func dropLeaves(c *creature.Set, v *render.View, x, y int) {
	seed := spritepkg.TileHash(x, y) ^ v.Present*0x9E3779B97F4A7C15
	n := 1 + int(motionpkg.Hash01(seed, 7)*2)
	parts := c.Parts
	from := len(*parts)
	c.Burst(v, creature.PartLeaf, float64(x)+0.5, float64(y)-0.55, n, seed)
	for i := from; i < len(*parts); i++ {
		p := &(*parts)[i]
		p.Clr = autumnLeaves[int(motionpkg.Hash01(seed, uint64(i), 9)*float64(len(autumnLeaves)))%len(autumnLeaves)]
		p.Life += 30 // leaves flutter down slowly
		p.G *= 0.5
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
		return gfx.TextW(lbl, gfx.Body)
	}
	name, rest, _ := strings.Cut(lbl, " ")
	_, speed, _ := strings.Cut(rest, " ")
	return gfx.TextW(name, gfx.Body) + 20 + gfx.TextW(speed, gfx.Body)
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
		uikit.DrawLabel(screen, x, y, lbl)
		return
	}
	name, rest, _ := strings.Cut(lbl, " ")
	_, speed, _ := strings.Cut(rest, " ")
	uikit.DrawLabel(screen, x, y, name)
	nameW := gfx.TextW(name, gfx.Body)
	cx, cy := float32(x)+nameW+10, float32(y+8)
	ux, uy := motionpkg.Unit(w.WindX, w.WindY)
	fx, fy := float32(ux), float32(uy)
	clr := color.RGBA{230, 230, 220, 255}
	tipX, tipY := cx+fx*6, cy+fy*6
	gfx.Line(screen, cx-fx*6, cy-fy*6, tipX, tipY, 1.5, clr, true)
	for _, s := range [2]float32{1, -1} { // arrow head
		hx := tipX - fx*4 + s*-fy*3
		hy := tipY - fy*4 + s*fx*3
		gfx.Line(screen, tipX, tipY, hx, hy, 1.5, clr, true)
	}
	uikit.DrawLabel(screen, x+int(nameW)+20, y, speed)
}
