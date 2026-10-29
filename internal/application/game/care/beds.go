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
	// surgery). The safe-bed ranking of SIM-063 (bedQuery).
	RescueBed BedNeed = iota
	// RestBed: a colonist going to bed rest. Medical beds only (from any
	// area): one it already holds first, then the nearest by tile
	// distance (pathing.Dist).
	RestBed
)

// SafeBedFor is the bed patient at (x, y) should lie in for need, 0 when
// none. For a rescue (RescueBed) it is the safe bed of SIM-063: a
// sleeping place the patient may claim, held by no one else for another
// patient, not burning nor at a deadly temperature (Reach.BedHazard); a
// prison bed exactly when the patient is a prisoner; for a colonist, one
// it can reach (a carrier checks its own way when it sets off). Ranked:
// a medical bed (a hospital bed first) when the patient needs tending or
// bed rest, or is a guest (guests use medical beds, wiki: Guest); else
// the patient's own bed; else any free bed; the nearest by Manhattan
// distance within a rank, ties to the lowest id.
func (c *Care) SafeBedFor(patient entity.EntityID, x, y int, need BedNeed) entity.EntityID {
	if need == RestBed {
		return restBedFor(c, patient, x, y)
	}
	q := rescueQuery(c, patient, x, y)
	if c.kindOf(patient) == targetColonist {
		q.reacher = patient
	}
	return q.best(c)
}

// PrisonBedFor is the safe prison bed for target at (x, y) (0: none):
// SafeBedFor's ranking over prison beds only, whether or not target is a
// prisoner yet (a captive is carried to one). The game's Prison role
// serves capture, arrest and the warden's carry with it.
func PrisonBedFor(c *Care, target entity.EntityID, x, y int) entity.EntityID {
	q := rescueQuery(c, target, x, y)
	q.prison = true
	return q.best(c)
}

// bedQuery is one safe-bed search (SIM-063): for patient at (x, y),
// prison beds only or never, medical beds first (care), and when reacher
// is set only beds reacher can get beside.
type bedQuery struct {
	patient, reacher entity.EntityID
	x, y             int
	prison, care     bool
}

// rescueQuery is the search for patient at (x, y) without a reach check:
// prison beds for a prisoner, medical first for a guest or a patient that
// needs tending or bed rest.
func rescueQuery(c *Care, patient entity.EntityID, x, y int) bedQuery {
	k := c.kindOf(patient)
	needs := k == targetGuest || k == targetStranger || c.h.NeedsTending(patient) || c.Rest.NeedsBedRest(patient)
	return bedQuery{patient: patient, x: x, y: y, prison: k == targetPrisoner, care: needs}
}

// rank is bed b's rank for q (higher first): a medical bed (5 a hospital
// bed, 4 any other) or a plain hospital bed (3) when q.care, else the
// patient's own bed (2), else 0.
func (q bedQuery) rank(b construction.Building) int {
	switch {
	case q.care && IsMedicalBed(b) && b.Kind == construction.HospitalBed:
		return 5
	case q.care && IsMedicalBed(b):
		return 4
	case q.care && b.Kind == construction.HospitalBed:
		return 3
	case b.SlotOf(uint64(q.patient)) >= 0:
		return 2
	}
	return 0
}

// best is the first bed by rank, then distance, then id, that q admits.
// The hazard and reach checks run only for a bed that would win, so a
// search costs one check per new leader, not one per bed.
func (q bedQuery) best(c *Care) entity.EntityID {
	h := c.h
	best, bestD, bestRank := entity.EntityID(0), 0, -1
	for _, bid := range h.BuildingIDs() {
		b, ok := h.Building(bid)
		if !ok || !b.Kind.Sleeps() || b.ForPrisoners != q.prison || !b.CanClaim(uint64(q.patient)) {
			continue
		}
		if owner, taken := h.Owner(bid); taken && owner != q.patient {
			continue
		}
		rank, d := q.rank(b), manhattan(q.x, q.y, b.X, b.Y)
		if rank < bestRank || rank == bestRank && (d > bestD || d == bestD && bid > best) {
			continue
		}
		if h.BedHazard(bid) {
			continue
		}
		if q.reacher != 0 {
			if _, _, ok := h.BesideSpot(q.reacher, b.X, b.Y); !ok {
				continue
			}
		}
		best, bestD, bestRank = bid, d, rank
	}
	return best
}

// restBedFor is the bed-rest choice (PLAY-020, unchanged by SIM-063):
// medical beds the patient may claim and no one else holds, one it
// already holds first, then the nearest by tile distance, ties to the
// lowest id.
func restBedFor(c *Care, patient entity.EntityID, x, y int) entity.EntityID {
	best, bestD := entity.EntityID(0), 1<<30
	for _, bid := range c.h.BuildingIDs() {
		b, ok := c.h.Building(bid)
		if !ok || !IsMedicalBed(b) || !b.CanClaim(uint64(patient)) || b.ForPrisoners != c.h.IsPrisoner(patient) {
			continue
		}
		if owner, taken := c.h.Owner(bid); taken && owner != patient {
			continue
		}
		d := pathing.Dist(x, y, b.X, b.Y)
		if b.SlotOf(uint64(patient)) >= 0 {
			d = -1
		}
		if best == 0 || d < bestD || d == bestD && bid < best {
			best, bestD = bid, d
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
