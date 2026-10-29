package care

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// IsMedicalBed: b is a sleeping place kept for the sick and injured.
func IsMedicalBed(b construction.Building) bool {
	return b.Medical && b.Kind.Sleeps()
}

// BedNeed is why a patient wants a bed; it picks which beds SafeBedFor
// considers and how it ranks them.
type BedNeed uint8

const (
	// RescueBed: a downed patient carried to bed (and one waiting for
	// surgery). Any free sleeping place: medical beds first (PLAY-020),
	// then hospital beds (NG-1 §5.6), each the nearest by Manhattan
	// distance.
	RescueBed BedNeed = iota
	// RestBed: a colonist going to bed rest. Medical beds only (from any
	// area): one it already holds first, then the nearest by tile
	// distance (pathing.Dist).
	RestBed
)

// SafeBedFor is the bed patient at (x, y) should lie in for need, 0 when
// none: a sleeping place it may claim, a prison bed exactly when it is a
// prisoner, not held by someone else for another patient; ties go to the
// lowest id.
func (c *Care) SafeBedFor(patient entity.EntityID, x, y int, need BedNeed) entity.EntityID {
	best, bestD, bestRank := entity.EntityID(0), 1<<30, 0
	prisoner := c.h.IsPrisoner(patient)
	for _, bid := range c.h.BuildingIDs() {
		b, ok := c.h.Building(bid)
		if !ok || !b.Kind.Sleeps() || need == RestBed && !b.Medical {
			continue
		}
		if !b.CanClaim(uint64(patient)) || b.ForPrisoners != prisoner {
			continue
		}
		if owner, taken := c.h.Owner(bid); taken && owner != patient {
			continue
		}
		rank, d := 0, 0
		switch need {
		case RescueBed:
			if IsMedicalBed(b) {
				rank += 2
			}
			if b.Kind == construction.HospitalBed {
				rank++
			}
			d = manhattan(x, y, b.X, b.Y)
		case RestBed:
			d = pathing.Dist(x, y, b.X, b.Y)
			if b.SlotOf(uint64(patient)) >= 0 {
				d = -1
			}
		}
		if best == 0 || rank > bestRank || rank == bestRank && (d < bestD || d == bestD && bid < best) {
			best, bestD, bestRank = bid, d, rank
		}
	}
	return best
}

func manhattan(ax, ay, bx, by int) int { return abs(ax-bx) + abs(ay-by) }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
