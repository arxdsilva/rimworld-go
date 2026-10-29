package rest

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/entity"
)

// Stats is the comfort, rest effectiveness and beauty of beds and bed
// furniture (BED-1, ENG-003 wiki numbers; the tables are in
// construction/furniture.go). An end table beside the bed's head and a
// dresser within construction.DresserReach tiles in the same room each
// add construction.FacilityComfort; one of each counts. Facilities change
// comfort only, never rest effectiveness (wiki: End table, Dresser).
type Stats struct{ h Host }

// BedView is one piece's stats, permille (Beauty in thousandths of a
// point), with the facilities linked to a bed (0: none).
type BedView struct {
	Comfort, RestEffect, Beauty int
	EndTable, Dresser           entity.EntityID
}

// Linked facility bits (Links), the read model's BuildingView.Facilities.
const (
	LinkEndTable uint8 = 1 << iota
	LinkDresser
)

// Links is the linked facilities as LinkEndTable | LinkDresser bits.
func (v BedView) Links() uint8 {
	var l uint8
	if v.EndTable != 0 {
		l |= LinkEndTable
	}
	if v.Dresser != 0 {
		l |= LinkDresser
	}
	return l
}

// qualityOf is b's craft quality: Normal for every building until
// quality lands on buildings (PRD-2).
func qualityOf(construction.Building) quality.Level { return quality.Default }

// RestEffect is how well a sleeper rests in bed (permille): the bed's
// rest effectiveness, or the ground's when bed is 0 or no sleeping place.
func (s *Stats) RestEffect(bed entity.EntityID) int {
	if bed != 0 {
		if b, ok := s.h.Building(bed); ok && (b.Kind.Sleeps() || b.Kind.AnimalSleeps()) {
			return construction.FurnitureOf(b.Kind, b.Stuff, qualityOf(b)).RestEffectiveness
		}
	}
	return construction.GroundRestEffectiveness
}

// Of is building id's stats; a bed's comfort includes its facilities.
// Pieces with no stats read all zero.
func (s *Stats) Of(id entity.EntityID) BedView {
	b, ok := s.h.Building(id)
	if !ok {
		return BedView{}
	}
	q := qualityOf(b)
	f := construction.FurnitureOf(b.Kind, b.Stuff, q)
	v := BedView{Comfort: f.Comfort, RestEffect: f.RestEffectiveness, Beauty: f.Beauty}
	if !b.Kind.Sleeps() {
		return v
	}
	cells, n := construction.SmallFootprint(b.Kind, b.Facing, b.X, b.Y) // a slot's head is cells[slot]; no allocation (the read model fills this per bed)
	v.EndTable, v.Dresser = s.endTable(cells[:n], b.Kind.Sleepers()), s.dresser(cells[0])
	if v.EndTable != 0 {
		v.Comfort += construction.FacilityComfort(q)
	}
	if v.Dresser != 0 {
		v.Comfort += construction.FacilityComfort(q)
	}
	return v
}

// endTable is an end table cardinally beside one of the head cells of
// the bed covering cells (0: none).
func (s *Stats) endTable(cells [][2]int, sleepers int) entity.EntityID {
	for _, h := range cells[:min(sleepers, len(cells))] {
		for _, d := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			c := [2]int{h[0] + d[0], h[1] + d[1]}
			if slices.Contains(cells, c) {
				continue
			}
			x, y := c[0], c[1]
			if id := s.kindAt(x, y, construction.EndTable); id != 0 {
				return id
			}
		}
	}
	return 0
}

// dresser is a dresser within DresserReach of a bed's head cell, in its
// room (0: none), scanned row by row from the top.
func (s *Stats) dresser(head [2]int) entity.EntityID {
	hx, hy := head[0], head[1]
	room := s.h.RoomAt(hx, hy)
	const r = construction.DresserReach
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy > r*r {
				continue
			}
			if id := s.kindAt(hx+dx, hy+dy, construction.Dresser); id != 0 && s.h.RoomAt(hx+dx, hy+dy) == room {
				return id
			}
		}
	}
	return 0
}

// kindAt is the k building covering (x, y) (0: none).
func (s *Stats) kindAt(x, y int, k construction.Kind) entity.EntityID {
	id := s.h.ObjectAt(x, y)
	if id == 0 {
		return 0
	}
	if b, ok := s.h.Building(id); ok && b.Kind == k {
		return id
	}
	return 0
}
