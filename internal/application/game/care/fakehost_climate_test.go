package care

import (
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeAir is fakeHost's Climate role (AIL-1): air per tile (20 °C unless
// set), heatstroke per pawn (Full-based), the everyday comfortable range,
// a healing spot per pawn and the ExposureWait ticks.
type fakeAir struct {
	temp  map[[2]int]int
	heat  map[entity.EntityID]int32
	spot  map[entity.EntityID][2]int
	wait  map[entity.EntityID]uint64
	spots int // HealingSpot calls
}

func (a *fakeAir) ensure() {
	if a.temp == nil {
		a.temp, a.heat = map[[2]int]int{}, map[entity.EntityID]int32{}
		a.spot, a.wait = map[entity.EntityID][2]int{}, map[entity.EntityID]uint64{}
	}
}

func (a *fakeAir) at(x, y int) int {
	if t, ok := a.temp[[2]int{x, y}]; ok {
		return t
	}
	return 200
}

func (h *fakeHost) AirHeals(id entity.EntityID, x, y, margin int) bool {
	h.air.ensure()
	b := exposure.Everyday
	return !exposure.Held(0, h.air.heat[id], exposure.Trivial, h.air.at(x, y), exposure.Bands{Min: b.Min + margin, Max: b.Max - margin})
}

func (h *fakeHost) HealingSpot(id entity.EntityID) (int, int, bool) {
	h.air.ensure()
	h.air.spots++
	s, ok := h.air.spot[id]
	return s[0], s[1], ok
}

func (h *fakeHost) ExposureWait(id entity.EntityID) uint64 { h.air.ensure(); return h.air.wait[id] }

func (h *fakeHost) SetExposureWait(id entity.EntityID, until uint64) {
	h.air.ensure()
	h.air.wait[id] = until
}

// heatstroked downs pawn id from heatstroke at sev (Full-based).
func (h *fakeHost) heatstroked(id entity.EntityID, sev int32) {
	h.air.ensure()
	h.vital[id] = health.Record{Status: health.Downed, Cause: health.CauseHeatstroke}
	h.air.heat[id] = sev
}

// air sets the temperature (tenths of a degree C) of tile (x, y).
func (h *fakeHost) setAir(x, y, t int) {
	h.air.ensure()
	h.air.temp[[2]int{x, y}] = t
}
