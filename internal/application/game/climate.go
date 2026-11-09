package game

import (
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/pkg/msgfmt"
)

// climateInterval is how often (ticks) rot and frost are applied.
const climateInterval = 250

// maxMessages is how many recent messages the read model keeps.
const maxMessages = 12

// MessageView is a one-line event notice ("12 meat rotted away").
type MessageView struct {
	Tick uint64
	Text string
	Msg  msgfmt.Text // Text as its catalog key and args (OPT-001; zero: show Text)
	X, Y int         // where it happened, for jumping the camera
	Map  MapID       // the map X, Y are on (MAP-0)
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
	// Repeat is how many times more the same notice came in a row (EAI-0
	// skeleton: always 0 yet); the UI shows "x(Repeat+1)".
	Repeat int
}

// temperatureAt is the air temperature on a tile: enclosed rooms hold the
// day's mean plus a little shelter warmth, everywhere else is outdoors.
func (g *Game) temperatureAt(x, y int) temperature.Temp {
	if r := g.roomAt(x, y); r >= 0 && r < len(g.thermo.Temps) {
		return g.thermo.Temps[r]
	}
	if t, ok := g.thermo.Residual[y*g.grid.Width()+x]; ok {
		return t // a freshly breached room still holds some warmth
	}
	return g.climate.Outdoor(g.clock.Now()) + g.weatherTempDelta(x, y) + g.ventWarmth(x, y) + g.fireWarmth(x, y) // vents.go, fire.go
}

// Sowing temperatures (PLAY-023; FOOD-2 wiki: Plants, the 0-58 °C growing range), tenths of a degree C:
// growers sow only while the growing temperature is above sowMinTemp and
// below sowMaxTemp. Seedlings would freeze below and scorch above.
const (
	sowMinTemp = 0
	sowMaxTemp = 580
)

// growingSeason reports whether the growing temperature on (x, y) lets
// growers sow there: the day's mean outdoors, or the room's temperature,
// between sowMinTemp and sowMaxTemp.
func (g *Game) growingSeason(x, y int) bool {
	mean := g.climate.DailyMean(g.clock.Now())
	if r := g.roomAt(x, y); r >= 0 && r < len(g.thermo.Temps) {
		mean = g.thermo.Temps[r]
	}
	return mean > sowMinTemp && mean < sowMaxTemp
}

// TemperatureAt is the air temperature on (x, y), tenths of a degree C.
func (g *Game) TemperatureAt(x, y int) int { return int(g.temperatureAt(x, y)) }

func (g *Game) message(x, y int, format string, args ...any) {
	g.post(MessageView{Tick: uint64(g.clock.Now()), Msg: msgfmt.Of(format, args...), X: x, Y: y})
}

// threat posts a danger message: a threat has arrived.
func (g *Game) threat(title string, x, y int, format string, args ...any) {
	// a major threat is a red letter in the stack too (PLAY-018), and a
	// danger message the UI may pause for
	g.post(MessageView{Tick: uint64(g.clock.Now()), Msg: msgfmt.Of(format, args...), X: x, Y: y, Danger: true,
		Letter: true, Title: title, Kind: letters.Threat})
}

// post posts m and returns its text: a view given only Msg prints it as
// its Text; one given only Text is keyed on that finished text
// (msgfmt.Plain; OPT-001).
func (g *Game) post(m MessageView) string {
	if m.Map = MapID(g.mapState.id); m.Text == "" {
		m.Text = m.Msg.String()
	} else if m.Msg.Format == "" {
		m.Msg = msgfmt.Make(msgfmt.Plain(m.Text), nil)
	}
	if m.Letter {
		m.Kind = g.addLetter(m) // a letter is also kept in the stack
	}
	g.messages = append(g.messages, m)
	if n := len(g.messages); n > maxMessages {
		g.messages = append(g.messages[:0:0], g.messages[n-maxMessages:]...)
	}
	return m.Text
}

// addLetter puts message m on the letter stack and returns the kind it got.
// The letter keeps the message's Tick, Title and Text (the UI matches a
// letter to its message by them); a message with no place (0, 0 or off the
// map) gives the letter X and Y of -1, so the UI offers no jump.
func (g *Game) addLetter(m MessageView) string {
	kind := letters.KindOf(m.Kind, m.Danger)
	x, y := m.X, m.Y
	if x == 0 && y == 0 || !g.grid.InBounds(x, y) {
		x, y = -1, -1
	}
	g.letters.Add(letters.Letter{Tick: m.Tick, Kind: kind, Title: m.Title, Text: m.Text, Msg: m.Msg,
		X: x, Y: y, Map: uint32(m.Map), Focus: m.Focus})
	return kind
}

// dismissLetter removes letter id (an unknown id is ignored). Dismissing a
// join offer declines it: the stranger, still here, goes its way.
func (g *Game) dismissLetter(id uint64) {
	if l, ok := g.letters.Remove(id); ok && l.Join != 0 && g.visitor(l.Join) {
		careGuests{g}.Leave(l.Join) // visitors.go
	} else if ok && l.Wanderer != 0 {
		for _, c := range g.pawnIDs {
			g.remember(c, mood.TurnedAwayWanderer)
		}
	}
}

// acceptLetter accepts letter id's offer (command.AcceptLetter): a rescued
// stranger still on the map joins. A letter with no offer is left as it is.
func (g *Game) acceptLetter(id uint64) {
	l, ok := g.letters.Get(id)
	if !ok || l.Join == 0 && l.Wanderer == 0 {
		return
	}
	if g.letters.Remove(id); l.Wanderer != 0 {
		g.wandererWalksIn(l.Wanderer) // visitors.go
		return
	}
	if v, _ := g.vital.Get(l.Join); !g.visitor(l.Join) || v.Status == health.Dead {
		g.worldNote("%s is no longer here to join.", g.personName(l.Join))
		return
	}
	careGuests{g}.join(l.Join) // visitors.go
}

// letterViews is the stack for the read model, oldest first (a copy).
func (g *Game) letterViews() []LetterView {
	list := g.letters.List()
	if len(list) == 0 {
		return nil
	}
	out := make([]LetterView, len(list))
	for i, l := range list {
		out[i] = LetterView{ID: l.ID, Tick: l.Tick, Kind: l.Kind, Title: l.Title, Text: l.Text, Msg: l.Msg,
			X: l.X, Y: l.Y, Map: MapID(l.Map), Focus: l.Focus, Intro: l.Intro, JoinOffer: l.Join != 0 || l.Wanderer != 0, Demand: l.Demand, Quest: l.Quest, Until: l.Until, Choices: l.Choices}
	}
	return out
}

// importLetters restores the stack from a save. A save from before the
// stack existed has letter messages but never issued an id (next is 0):
// those messages become the letters, oldest first (an old intro letter
// reads as a plain info letter).
func (g *Game) importLetters(list []letters.Letter, next uint64) {
	g.letters = letters.State{}
	g.letters.Import(list, next)
	if next != 0 || len(list) != 0 {
		return
	}
	for _, m := range g.messages {
		if m.Letter {
			g.addLetter(m)
		}
	}
}

// phaseClimate is the climate block every climate interval, after the
// room heat, rot and wear (phaseSystems): roofs, fuel, pumps, exposure
// (frostbitten crops die with the plant pass: plant_growth.go).
func (g *Game) phaseClimate() {
	g.checkRoofSupport() // roof.go: unsupported roofs fall first
	g.burnFuel()         // fuel.go: the burners use their fuel
	g.br.ind.Pumps(industryHost{g}).Advance(climateInterval)
	g.expose()
	senseOf(g).Gear.Rot(climateInterval)    // game/sense (CSE-036)
	g.craft.Drying.Advance(climateInterval) // craft/drying.go; frost: per chunk with the plants (plant_growth.go)
}
