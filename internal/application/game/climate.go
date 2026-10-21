package game

import (
	"fmt"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
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
	return temperature.Outdoor(g.clock.Now()) + g.weatherTempDelta(x, y)
}

// sowMinMean is the lowest daily mean temperature (tenths of a degree C)
// at which growers still sow: below it seedlings would only freeze.
const sowMinMean = 30

// growingSeason reports whether it is warm enough to sow on (x, y): the
// day's mean there (outdoors, or the room's temperature) is above
// sowMinMean.
func (g *Game) growingSeason(x, y int) bool {
	mean := temperature.DailyMean(g.clock.Now())
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

// phaseClimate rots perishable stacks and kills frostbitten crops.
func (g *Game) phaseClimate() {
	if uint64(g.clock.Now())%climateInterval != 0 {
		return
	}
	g.checkRoofSupport() // roof.go: unsupported roofs fall first
	g.heatRooms()
	g.expose()
	g.rotItems()
	g.frostCrops()
	g.dryRacks()
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
	for _, id := range append([]entity.EntityID(nil), g.itemIDs...) {
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
		g.deleteItem(id)
		if lost[st.Kind] == nil {
			lost[st.Kind] = &loss{x: x, y: y}
			order = append(order, st.Kind)
		}
		lost[st.Kind].qty += st.Qty
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

// frostCrops kills crops standing in air colder than they can bear.
func (g *Game) frostCrops() {
	dead := map[plant.Species]int{}
	var order []plant.Species
	var fx, fy int
	for _, id := range append([]entity.EntityID(nil), g.plantIDs...) {
		p, ok := g.plants.Get(id)
		if !ok || !plant.Frostbitten(p.Species, int(g.temperatureAt(p.X, p.Y))) {
			continue
		}
		g.removePlant(id)
		if dead[p.Species] == 0 {
			order = append(order, p.Species)
			fx, fy = p.X, p.Y
		}
		dead[p.Species]++
	}
	for _, s := range order {
		if n := dead[s]; n == 1 {
			g.message(fx, fy, "Frost killed a %s plant", s)
		} else {
			g.message(fx, fy, "Frost killed the %s (%d plants)", plant.DefOf(s).Label, n)
		}
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
