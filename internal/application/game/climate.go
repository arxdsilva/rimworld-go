package game

import (
	"fmt"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
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
	return g.climate.Outdoor(g.clock.Now()) + g.weatherTempDelta(x, y)
}

// sowMinMean is the lowest daily mean temperature (tenths of a degree C)
// at which growers still sow: below it seedlings would only freeze.
const sowMinMean = 30

// growingSeason reports whether it is warm enough to sow on (x, y): the
// day's mean there (outdoors, or the room's temperature) is above
// sowMinMean.
func (g *Game) growingSeason(x, y int) bool {
	mean := g.climate.DailyMean(g.clock.Now())
	if r := g.roomAt(x, y); r >= 0 && r < len(g.roomTemps) {
		mean = g.roomTemps[r]
	}
	return mean > sowMinMean
}

// TemperatureAt is the air temperature on (x, y), tenths of a degree C.
func (g *Game) TemperatureAt(x, y int) int { return int(g.temperatureAt(x, y)) }

func (g *Game) message(x, y int, format string, args ...any) {
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: fmt.Sprintf(format, args...), X: x, Y: y})
}

// threat posts a danger message: a threat has arrived.
func (g *Game) threat(x, y int, format string, args ...any) {
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: fmt.Sprintf(format, args...), X: x, Y: y, Danger: true})
}

func (g *Game) post(m MessageView) {
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
	g.rotItems()
	g.dryRacks() // frost: per chunk with the plants (plant_growth.go)
}

// rotItems advances spoilage by the local temperature. Spoiled food is
// destroyed; a rotten corpse lingers unbutcherable until it has rotted as
// long again, then decays away, except a human corpse, which becomes the
// deceased's bones (SIM-017). Stacks being carried or reserved for a job
// keep rotting but are only removed once they are free.
func (g *Game) rotItems() {
	type loss struct {
		qty  int
		x, y int
	}
	lost := map[item.Kind]*loss{}
	var order []item.Kind
	var drop map[entity.EntityID]bool // removed from itemIDs once, in order, after the pass
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if !ok || st.Shelf() == 0 {
			continue
		}
		x, y := st.X, st.Y
		if st.Carrier != 0 {
			if p, ok := g.positions.Get(st.Carrier); ok {
				x, y = p.X, p.Y
			}
		}
		st.Rot += climateInterval * item.RotRate(int(g.temperatureAt(x, y))) / 1000
		if st.Kind == item.CorpseHuman && st.Rot >= 2*st.Shelf() {
			st.Kind, st.Rot = item.Bones, 0 // same Of: whose bones they are
		}
		g.stacks.Set(id, st)
		gone := st.Rotten() && !st.Kind.IsCorpse() || st.Kind.IsCorpse() && st.Rot >= 2*st.Shelf()
		if !gone || st.Carrier != 0 {
			continue
		}
		if _, taken := g.reserve.EntityOwner(id); taken {
			continue
		}
		g.stacks.Delete(id) // deleteItem, with the list compacted below
		if drop == nil {
			drop = map[entity.EntityID]bool{}
		}
		drop[id] = true
		if lost[st.Kind] == nil {
			lost[st.Kind] = &loss{x: x, y: y}
			order = append(order, st.Kind)
		}
		lost[st.Kind].qty += st.Qty
	}
	if drop != nil {
		g.itemIDs = dropIDs(g.itemIDs, func(id entity.EntityID) bool { return drop[id] })
	}
	for _, k := range order {
		l := lost[k]
		if k.IsCorpse() {
			g.message(l.x, l.y, "A rotten %s decayed away", k)
			continue
		}
		g.message(l.x, l.y, "%d %s rotted away", l.qty, k)
	}
}

// FreshDays is how many days a perishable stack keeps at full rot speed
// (room temperature); -1 for food that never spoils.
func freshDays(st item.Stack) float64 {
	if st.Shelf() == 0 {
		return -1
	}
	return float64(st.FreshTicks()) / float64(clock.TicksPerDay)
}
