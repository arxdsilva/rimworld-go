package game

import (
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/weather"
)

// Weather (SIM-016): the sky over open ground, its wind and what it
// leaves behind, plus the light gameplay it causes. Contract stubs; the
// weather model fills them in.

// phaseWeather moves the weather on: a new block every 6 hours, easing
// and ground build-up every climate interval.
func (g *Game) phaseWeather() {}

// weatherSlow is base step ticks plus what rain, snow or a blizzard adds
// on an outdoor tile.
func (g *Game) weatherSlow(x, y int, base int) int { return base }

// weatherTempDelta is the weather's effect on outdoor air at (x, y).
func (g *Game) weatherTempDelta(x, y int) temperature.Temp { return 0 }

// doused reports an outdoor campfire put out by rain or snow for now.
func (g *Game) doused(bid entity.EntityID) bool { return false }

// SetWeather forces the weather (dev scenarios).
func (g *Game) SetWeather(k weather.Kind, intensity float64) {
	g.weather.Kind, g.weather.Target, g.weather.Intensity = k, k, intensity
}

// Weather is the current outdoor weather.
func (g *Game) Weather() weather.State { return g.weather }

// fillWorldWeather copies the weather into the world view.
func (g *Game) fillWorldWeather(w *WorldView) {}

// fillPawnWeather sets PawnView.Soaked.
func (g *Game) fillPawnWeather(pv *PawnView) {}

// fillPlantWeather sets PlantView.Wilted.
func (g *Game) fillPlantWeather(pv *PlantView, p plant.Plant) {}
