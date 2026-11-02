package industry

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// The moisture pump (wiki: Moisture pump): while powered it dries the
// ground in a circle that grows one cell of radius every 8.7 days, up to
// 6.9 cells (about 60 days): marsh and mud become soil, shallow water
// stony soil (our gravel). Moving river water, deep water and the open
// sea are never dried. A pump keeps its progress while unpowered; one
// taken down loses it.
const (
	// PumpTileTicks is the powered ticks per cell of radius (8.7 days).
	PumpTileTicks = 87 * clock.TicksPerDay / 10
	// PumpRadius10 is the full radius in tenths of a cell (6.9).
	PumpRadius10 = 69
)

// Pumps is the moisture pumps over a State and its host.
type Pumps struct {
	s *State
	h Host
}

// Pumps is the moisture pumps of s.
func (s *State) Pumps(h Host) Pumps { return Pumps{s, h} }

// radius10 is the dried radius of a pump with work powered ticks, in
// tenths of a cell.
func radius10(work int) int { return min(PumpRadius10, work*10/PumpTileTicks) }

// Radius is pump id's dried radius now, in whole cells.
func (p Pumps) Radius(id entity.EntityID) int { return radius10(p.s.work[id]) / 10 }

// Advance runs every powered pump for dt ticks, drying the cells its
// circle reaches whenever it grows by a tenth of a cell.
func (p Pumps) Advance(dt int) {
	for _, id := range p.s.sitesOf(p.h).pumps {
		b, ok := p.h.Building(id)
		if !ok || !p.h.Powered(id) {
			continue
		}
		old := p.s.work[id]
		if radius10(old) >= PumpRadius10 {
			continue
		}
		w := old + dt
		p.s.setWork(id, w)
		if r := radius10(w); r > radius10(old) {
			p.dry(b.X, b.Y, r)
		}
	}
}

// dry turns the wet ground within r10 tenths of a cell of (cx, cy) dry.
func (p Pumps) dry(cx, cy, r10 int) {
	w, h := p.h.Size()
	reach := r10/10 + 1
	for y := max(0, cy-reach); y <= min(h-1, cy+reach); y++ {
		for x := max(0, cx-reach); x <= min(w-1, cx+reach); x++ {
			if (x-cx)*(x-cx)*100+(y-cy)*(y-cy)*100 > r10*r10 {
				continue
			}
			if t, ok := Dried(p.h.Terrain(x, y)); ok && !p.s.Moving(x, y) && !p.h.Ocean(x, y) {
				p.h.SetTerrain(x, y, t)
			}
		}
	}
}

// Dried is what a moisture pump turns ground t into (ok false: it leaves
// t alone).
func Dried(t terrain.Type) (terrain.Type, bool) {
	switch t {
	case terrain.Marsh, terrain.Mud:
		return terrain.Soil, true
	case terrain.ShallowWater:
		return terrain.Gravel, true
	}
	return t, false
}
