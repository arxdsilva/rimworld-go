package game

import (
	"fmt"

	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// climateInterval is how often (ticks) rot and frost are applied.
const climateInterval = 250

// maxMessages is how many recent messages the read model keeps.
const maxMessages = 12

// MessageView is a one-line event notice ("12 meat rotted away").
type MessageView struct {
	Tick uint64
	Text string
	X, Y int // where it happened, for jumping the camera
	// Danger marks a threat arriving (raids, predator packs): the UI may
	// pause the game for it.
	Danger bool
	// Focus is the creature the message is about (0: none), for a
	// jump-to link that follows it.
	Focus entity.EntityID
	// Letter: show it once in a letter window titled Title ("Continue"):
	// the scenario's intro, extreme weather (NG-1).
	Letter bool
	Title  string
	// Kind (PLAY-018) colours a letter in the right-side stack: "threat",
	// "caution", "info" or "good" ("" reads as info, or threat if Danger).
	Kind string
}

// temperatureAt is the air temperature on a tile: enclosed rooms hold the
// day's mean plus a little shelter warmth, everywhere else is outdoors.
func (g *Game) temperatureAt(x, y int) temperature.Temp {
	if r := g.roomAt(x, y); r >= 0 && r < len(g.roomTemps) {
		return g.roomTemps[r]
	}
	if t, ok := g.residual[y*g.grid.Width()+x]; ok {
		return t // a freshly breached room still holds some warmth
	}
	return g.climate.Outdoor(g.clock.Now()) + g.weatherTempDelta(x, y) + g.ventWarmth(x, y) + g.fireWarmth(x, y) // vents.go, fire.go
}

// Sowing temperatures (PLAY-023, original values), tenths of a degree C:
// growers sow only while the growing temperature is above sowMinTemp and
// below sowMaxTemp. Seedlings would freeze below and scorch above.
const (
	sowMinTemp = 0
	sowMaxTemp = 500
)

// growingSeason reports whether the growing temperature on (x, y) lets
// growers sow there: the day's mean outdoors, or the room's temperature,
// between sowMinTemp and sowMaxTemp.
func (g *Game) growingSeason(x, y int) bool {
	mean := g.climate.DailyMean(g.clock.Now())
	if r := g.roomAt(x, y); r >= 0 && r < len(g.roomTemps) {
		mean = g.roomTemps[r]
	}
	return mean > sowMinTemp && mean < sowMaxTemp
}

// TemperatureAt is the air temperature on (x, y), tenths of a degree C.
func (g *Game) TemperatureAt(x, y int) int { return int(g.temperatureAt(x, y)) }

func (g *Game) message(x, y int, format string, args ...any) {
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: fmt.Sprintf(format, args...), X: x, Y: y})
}

// threat posts a danger message: a threat has arrived.
func (g *Game) threat(title string, x, y int, format string, args ...any) {
	// a major threat is a red letter in the stack too (PLAY-018), and a
	// danger message the UI may pause for
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: fmt.Sprintf(format, args...), X: x, Y: y, Danger: true,
		Letter: true, Title: title, Kind: letterThreat})
}

func (g *Game) post(m MessageView) {
	if m.Letter {
		m.Kind = g.addLetter(m) // letters.go: a letter is also kept in the stack
	}
	g.messages = append(g.messages, m)
	if n := len(g.messages); n > maxMessages {
		g.messages = append(g.messages[:0:0], g.messages[n-maxMessages:]...)
	}
}

// phaseClimate rots perishable stacks, moves room temperatures and
// exposure on (frostbitten crops die with the plant pass: plant_growth.go).
func (g *Game) phaseClimate() {
	if uint64(g.clock.Now())%climateInterval != 0 {
		return
	}
	g.checkRoofSupport() // roof.go: unsupported roofs fall first
	g.heatRooms()
	g.expose()
	g.craft.Condition.Rot(climateInterval)           // craft/condition.go
	g.craft.Condition.Weather(uint64(g.clock.Now())) // PRD-2: deterioration and worn wear, hourly
	g.craft.Drying.Advance(climateInterval)          // craft/drying.go; frost: per chunk with the plants (plant_growth.go)
}
