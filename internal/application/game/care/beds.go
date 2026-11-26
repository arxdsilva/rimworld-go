// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package care

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// IsMedicalBed: b is a sleeping place kept for the sick and injured.
func IsMedicalBed(b construction.Building) bool {
	return b.Medical && (b.Kind.Sleeps() || b.Kind.AnimalSleeps())
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
// distance within a rank, ties to the lowest id. A rescue takes only a
// bed whose air heals the patient's hypothermia or heatstroke (healBest,
// AIL-1).
func (c *Care) SafeBedFor(patient entity.EntityID, x, y int, need BedNeed) entity.EntityID {
	if need == RestBed {
		return restBedFor(c, patient, x, y)
	}
	q := rescueQuery(c, patient, x, y)
	if c.kindOf(patient) == targetColonist {
		q.reacher = patient
	}
	return q.healBest(c)
}

// PrisonBedFor is the safe prison bed for target at (x, y) (0: none):
// SafeBedFor's ranking over prison beds only, whether or not target is a
// prisoner yet (a captive is carried to one). The game's Prison role
// serves capture, arrest and the warden's carry with it.
func PrisonBedFor(c *Care, target entity.EntityID, x, y int) entity.EntityID {
	q := rescueQuery(c, target, x, y)
	q.prison, q.heal = true, false
	return q.best(c)
}

// bedQuery is one safe-bed search (SIM-063): for patient at (x, y),
// prison beds only or never, medical beds first (care), when reacher is
// set only beds reacher can get beside, and with heal only beds whose air
// heals the patient's exposure (AIL-1).
type bedQuery struct {
	patient, reacher, carrier  entity.EntityID
	x, y                       int
	prison, care, heal, animal bool
}

// healMargin is how far inside the patient's comfortable range (tenths of
// a degree) a bed's air must be for a rescue to take it there: a patient
// is moved once its air leaves the range, so a bed right on the line
// would send it back on the next drift (AIL-1 hysteresis).
const healMargin = 10

// rescueQuery is the search for patient at (x, y) without a reach check:
// prison beds for a prisoner, medical first for a guest or a patient that
// needs tending or bed rest.
func rescueQuery(c *Care, patient entity.EntityID, x, y int) bedQuery {
	k := c.kindOf(patient)
	isAnimal := c.h.CareGroup(patient) == medcare.Animals
	needs := isAnimal || k == targetGuest || k == targetStranger || c.h.NeedsTending(patient) || c.Rest.NeedsBedRest(patient)
	return bedQuery{patient: patient, x: x, y: y, prison: k == targetPrisoner, care: needs, heal: k != targetPrisoner, animal: isAnimal}
}

// healBest is best over the beds whose air heals the patient. With none,
// a patient out of bed in air that won't heal it either, and with no spot
// whose air does (Climate.HealingSpot), still goes to any safe bed: no
// worse than where it lies (AIL-1).
func (q bedQuery) healBest(c *Care) entity.EntityID {
	if bid := q.best(c); bid != 0 || !q.heal {
		return bid
	}
	if !q.fallbackAllowed(c) {
		return 0
	}
	q.heal = false
	return q.best(c)
}

// fallbackAllowed is the shared live no-healing-site exception. A patient
// already in healing air or its bed must not be moved into worse air.
func (q bedQuery) fallbackAllowed(c *Care) bool {
	h := c.h
	px, py, _ := h.Pos(q.patient)
	if h.BedUnder(q.patient) != 0 || h.AirHeals(q.patient, px, py, 0) {
		return false
	}
	_, _, ok := h.HealingSpot(q.patient)
	return !ok
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
	case construction.BedSlot(b, uint64(q.patient)) >= 0:
		return 2
	}
	return 0
}

// best is the first bed by rank, then distance, then id, that q admits.
// The hazard and reach checks run only for a bed that would win, so a
// search costs one check per new leader, not one per bed.
func (q bedQuery) eligible(c *Care, bid entity.EntityID, b construction.Building) bool {
	if (q.animal && !b.Kind.AnimalSleeps()) || (!q.animal && !b.Kind.Sleeps()) ||
		b.ForPrisoners != q.prison || b.Forbidden || b.Deconstruct || b.Uninstall || !construction.CanClaimBed(b, uint64(q.patient)) {
		return false
	}
	owner, taken := c.h.Owner(bid)
	return !taken || owner == q.patient || owner == q.reacher || owner == q.carrier
}
func (q bedQuery) bedWhy(c *Care, bid entity.EntityID, b construction.Building) string {
	if q.prison && !c.h.PrisonRoom(bid) {
		return ReasonPrisonRoom
	}
	if why := c.h.BedWhy(bid, q.carrier); why != "" {
		return why
	}
	if q.heal && !c.h.AirHeals(q.patient, b.X, b.Y, healMargin) {
		return ReasonBedTemperature
	}
	return ""
}
func (q bedQuery) refusal(c *Care) string {
	// A usable but hazardous bed gives a precise refusal rather than
	// claiming no bed exists. Rank deterministically like selection.
	bestRank, bestD := -1, 0
	var best entity.EntityID
	why := ""
	for _, id := range c.h.BuildingIDs() {
		b, ok := c.h.Building(id)
		if !ok || !q.eligible(c, id, b) {
			continue
		}
		r, d := q.rank(b), manhattan(q.x, q.y, b.X, b.Y)
		if r < bestRank || r == bestRank && (d > bestD || d == bestD && id > best) {
			continue
		}
		if reason := q.bedWhy(c, id, b); reason != "" {
			best, bestRank, bestD, why = id, r, d, reason
		}
	}
	return why
}

func (q bedQuery) best(c *Care) entity.EntityID {
	h := c.h
	best, bestD, bestRank := entity.EntityID(0), 0, -1
	for _, bid := range h.BuildingIDs() {
		b, ok := h.Building(bid)
		if !ok || !q.eligible(c, bid, b) {
			continue
		}
		rank, d := q.rank(b), manhattan(q.x, q.y, b.X, b.Y)
		if rank < bestRank || rank == bestRank && (d > bestD || d == bestD && bid > best) {
			continue
		}
		if q.bedWhy(c, bid, b) != "" {
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

// restBedFor is the safe bed-rest choice (PLAY-020):
// medical beds the patient may claim and no one else holds, one it
// already holds first, then the nearest by tile distance, ties to the
// lowest id.
func restBedFor(c *Care, patient entity.EntityID, x, y int) entity.EntityID {
	best, bestD := entity.EntityID(0), 1<<30
	for _, bid := range c.h.BuildingIDs() {
		b, ok := c.h.Building(bid)
		if !ok || !b.Kind.Sleeps() || !IsMedicalBed(b) || b.Forbidden || b.Uninstall || b.Deconstruct || !construction.CanClaimBed(b, uint64(patient)) || b.ForPrisoners != c.h.IsPrisoner(patient) {
			continue
		}
		if owner, taken := c.h.Owner(bid); taken && owner != patient {
			continue
		}
		d := pathing.Dist(x, y, b.X, b.Y)
		if construction.BedSlot(b, uint64(patient)) >= 0 {
			d = -1
		}
		if best != 0 && (d > bestD || d == bestD && bid > best) {
			continue
		}
		sx, sy := c.h.SleepCell(patient, construction.ClaimBed(b, uint64(patient)))
		if !restBedSafe(c.h, patient, bid, sx, sy) {
			continue
		}
		if _, ok := c.h.PathTo(patient, sx, sy); !ok {
			continue
		}
		best, bestD = bid, d
	}
	return best
}

// The same clinical checks protect both a newly chosen bed and ongoing
// bed rest. A bed claim alone does not make the bed safe.
func restBedSafe(h Host, patient, bid entity.EntityID, x, y int) bool {
	b, ok := h.Building(bid)
	return ok && !b.Forbidden && !b.Uninstall && !b.Deconstruct && !h.BedHazard(bid) && h.AirHeals(patient, x, y, 0)
}

func manhattan(ax, ay, bx, by int) int { return mathx.Abs(ax-bx) + mathx.Abs(ay-by) }
