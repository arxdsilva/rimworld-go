// Package snow is the per-tile snow layer and the snow clearing area
// (PLAY-015 §4): 0..255 per cell (255 = deepest) on open ground, i.e.
// unroofed, not a wall, not water, not rock. It follows the weather
// model's own rates (weather.State.Step): it builds up while snow falls,
// by intensity, melts above freezing and is halved when a rain or storm
// block begins. Roofed cells stay bare.
//
// It is a consumer sub-package of internal/application/game
// (.agents/specs/SOLID_GO.md §10): it never imports the game package. The
// simulation comes in through the consumer-owned roles in deps.go, which
// the game implements with one adapter (snowHost, game/weather.go).
package snow
