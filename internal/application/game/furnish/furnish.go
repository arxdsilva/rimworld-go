// Package furnish is A2's furniture and decor rules that keep state on
// the buildings themselves (AX-2): plant pots and the decorative plants
// growing in them, and the one-off move of older saves' 1x1 tables to
// the vanilla 1x2 table.
//
// It is a consumer sub-package of internal/application/game
// (.agents/specs/SOLID_GO.md §10): it never imports the game package, and
// reaches the simulation only through Host. It holds no state of its own:
// a pot's plant and growth are construction.Building.Plant and Growth,
// saved with the building. It is deterministic: no random numbers, no
// wall clock, buildings walked in the host's order.
package furnish

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/pkg/mathx"
)

// Buildings reads and writes the buildings (the host's own id slice,
// read only).
type Buildings interface {
	BuildingIDs() []entity.EntityID
	Building(id entity.EntityID) (construction.Building, bool)
	SetBuilding(id entity.EntityID, b construction.Building)
}

// Host is what the furnish rules need from the simulation.
type Host interface {
	Buildings
	// GrowRate is how fast (permille of full speed) a pot plant on
	// (x, y) grows now: its light and air (the growing hours are
	// furnish's own rule).
	GrowRate(x, y int) int
	// Air is the temperature on (x, y), tenths of a degree C.
	Air(x, y int) int
	// RoomsChanged recounts the rooms (a pot's beauty moved).
	RoomsChanged()
}

// PotEvery is how often (ticks) pot plants grow: once an hour.
const PotEvery = clock.TicksPerHour

// A pot plant dies below PotFrost (wiki: Plants, the warmest crop cold
// threshold) and a grower sows one only at PotSowAir or warmer (original
// tuning): the gap keeps a cold pot from being sown and frozen hour after
// hour. A sown plant starts at Sprout growth, so a sown pot never reads
// as waiting (growth 0).
const (
	PotFrost  = plant.ColdWarmest
	PotSowAir = 0
	Sprout    = 10
)

// Pots grows and sows plant pots (wiki: Plant pot, Rose, Daylily).
type Pots struct{ h Host }

// NewPots is the pot rules on host h.
func NewPots(h Host) Pots { return Pots{h: h} }

// SetPlant picks pot id's plant p (PotNone empties it); a grower then
// sows it (ToSow, Sow). A pot keeps the plant it already holds; another
// choice replaces it (the grower replants). False when id is no plant pot
// or p is unknown.
func (s Pots) SetPlant(id entity.EntityID, p construction.PotPlant) bool {
	b, ok := s.h.Building(id)
	if !ok || b.Kind != construction.PlantPot || p != construction.PotNone && !p.Valid() {
		return false
	}
	if construction.PotPlant(b.Plant) == p {
		return true
	}
	b.Plant, b.Growth = uint8(p), 0
	s.h.SetBuilding(id, b)
	s.h.RoomsChanged()
	return true
}

// Tick runs hourly: a sown pot's plant dies in air below PotFrost (a
// grower replants it), and in the growing hours (plant.GrowingHour) it
// grows, taking its grow days at full speed in growing time, as a crop
// does (wiki: Plants). Rooms are recounted when a pot's beauty moves a
// tenth (every 10 % of growth) or a plant dies.
func (s Pots) Tick(now uint64) {
	if now%PotEvery != 0 {
		return
	}
	growing := plant.GrowingHour(int(clock.HourOfDay(clock.Tick(now))))
	changed := false
	for _, id := range s.h.BuildingIDs() {
		b, ok := s.h.Building(id)
		if !ok || b.Kind != construction.PlantPot || b.Growth >= 1000 {
			continue
		}
		p := construction.PotPlant(b.Plant)
		if !p.Valid() || b.Growth == 0 {
			continue // empty, or waiting for a grower
		}
		if s.h.Air(b.X, b.Y) < PotFrost { // frozen: it dies, and a grower replants it once it is warm
			b.Growth, changed = 0, true
			s.h.SetBuilding(id, b)
			continue
		}
		if !growing {
			continue
		}
		g := Grow(p, int(b.Growth), s.h.GrowRate(b.X, b.Y))
		if g == int(b.Growth) {
			continue
		}
		changed = changed || g/100 != int(b.Growth)/100
		b.Growth = uint16(g)
		s.h.SetBuilding(id, b)
	}
	if changed {
		s.h.RoomsChanged()
	}
}

// ToSow is the nearest pot to (x, y) waiting for its plant to be sown,
// not forbidden and warm enough (PotSowAir), with the cell a grower works
// it from (the pot's own: it is passed through) and the sowing work. ok
// false when none waits.
func (s Pots) ToSow(x, y int) (id entity.EntityID, px, py, work int, ok bool) {
	best := 1 << 30
	for _, bid := range s.h.BuildingIDs() {
		b, has := s.h.Building(bid)
		if !has || !Waiting(b) || b.Forbidden || s.h.Air(b.X, b.Y) < PotSowAir {
			continue
		}
		if d := max(mathx.Abs(b.X-x), mathx.Abs(b.Y-y)); d < best {
			best, id, px, py, work = d, bid, b.X, b.Y, construction.PotPlant(b.Plant).DefOf().SowWork
		}
	}
	return id, px, py, work, best < 1<<30
}

// Waiting reports a plant pot with a plant chosen and not sown yet.
func Waiting(b construction.Building) bool {
	return b.Kind == construction.PlantPot && construction.PotPlant(b.Plant).Valid() && b.Growth == 0
}

// Sow is a grower's sowing of pot id done: its plant sprouts. False when
// the pot no longer waits (emptied, replaced or sown by another).
func (s Pots) Sow(id entity.EntityID) bool {
	b, ok := s.h.Building(id)
	if !ok || !Waiting(b) {
		return false
	}
	b.Growth = Sprout
	s.h.SetBuilding(id, b)
	s.h.RoomsChanged()
	return true
}

// Grow is plant p's growth (permille) after one PotEvery at rate
// (permille): its grow days at full rate in growing time (one growth
// unit per tick, construction.PotPlantDef.GrowDays days of them), capped
// at grown.
func Grow(p construction.PotPlant, growth, rate int) int {
	days := p.DefOf().GrowDays
	if days <= 0 || rate <= 0 {
		return growth
	}
	step := int(float64(PotEvery) * float64(rate) / (days * float64(clock.TicksPerDay)))
	return min(1000, growth+max(step, 1))
}
