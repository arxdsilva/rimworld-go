package rest

import (
	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Beds chooses where a pawn sleeps and who owns which bed (ENG-003, M5).
//
// SleepIn asks partnerBed first: a partnered colonist takes the free slot
// of the double bed its partner holds, else keeps its own double bed,
// else claims a free double bed for the two (the partner joins it on its
// next sleep); otherwise the usual choice (its own bed, the nearest free
// one) stands. Each rule keeps what the couple already holds, so the
// choice never flips between nights. Medical beds are left to patients
// and prison beds to prisoners; a forbidden bed is nobody's (BED-1).
type Beds struct {
	h       Host
	holders map[entity.EntityID]bool // Bedded's reused set
}

// Bedded is which of ids has a bed (BED-3, the read model's
// PawnView.HasBed, behind the "colonists need beds" alert): a pawn holding
// a slot of a bed, spot or bedroll (prison and medical beds included, so
// patients and prisoners count), or a colonist (MayOwnBed) that a free
// slot of a bed it could claim (not forbidden, not for prisoners, not
// medical) is left for, one slot per colonist in ids order: two bedless
// colonists and one free slot leave the second without. One pass over the
// buildings and one over ids; the set is reused: read it before the next
// call.
func (r *Beds) Bedded(ids []entity.EntityID) map[entity.EntityID]bool {
	if r.holders == nil {
		r.holders = make(map[entity.EntityID]bool)
	}
	clear(r.holders)
	free := 0
	for _, bid := range r.h.BuildingIDs() {
		b, ok := r.h.Building(bid)
		if !ok || !b.Kind.Sleeps() {
			continue
		}
		claimable := !b.Forbidden && !b.ForPrisoners && !care.IsMedicalBed(b)
		for slot := range b.Kind.Sleepers() {
			switch c := slotOwner(b, slot); {
			case c != 0:
				r.holders[entity.EntityID(c)] = true
			case claimable:
				free++
			}
		}
	}
	for _, id := range ids {
		if free > 0 && !r.holders[id] && r.h.MayOwnBed(id) {
			r.holders[id] = true
			free--
		}
	}
	return r.holders
}

// slotOwner is who holds slot of bed b (0: free).
func slotOwner(b construction.Building, slot int) uint64 {
	if slot == 1 {
		return b.Claimant2
	}
	return b.Claimant
}

// withSlot is b with slot held by id (0 frees it).
func withSlot(b construction.Building, slot int, id uint64) construction.Building {
	if slot == 1 {
		b.Claimant2 = id
	} else {
		b.Claimant = id
	}
	return b
}

// SleepIn sends pawn id to its partner's or its own bed, else the
// nearest free bed it may claim (giving up the old one); false, with no
// bed held, when there is none or it can't be reached. Medical beds are
// left to patients (care/rest.go: restInMedicalBed holds them for a stay).
func (r *Beds) SleepIn(id entity.EntityID) bool {
	if bid, slot, ok := r.partnerBed(id); ok && r.sleepInPartnerBed(id, bid, slot) {
		return true
	}
	x, y := r.h.Pos(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, bid := range r.h.BuildingIDs() {
		b, ok := r.h.Building(bid)
		if !ok || !b.Kind.Sleeps() || !b.CanClaim(uint64(id)) || b.ForPrisoners || b.Forbidden || care.IsMedicalBed(b) { // prison beds: prisoners only
			continue
		}
		if !r.h.AreaAllows(id, b.X, b.Y) {
			continue // a bed outside the allowed area
		}
		d := pathing.Dist(x, y, b.X, b.Y)
		if b.SlotOf(uint64(id)) >= 0 {
			d = -1 // its own bed first
		}
		if d < bestD || (d == bestD && bid < best) {
			best, bestD = bid, d
		}
	}
	if best == 0 {
		return false
	}
	b, _ := r.h.Building(best)
	if b.SlotOf(uint64(id)) < 0 {
		r.h.ReleaseBed(id) // moving to a new bed gives up the old one
	}
	b = b.Claim(uint64(id))
	r.h.SetBuilding(best, b)
	r.h.SetSleptAt(id, best)
	sx, sy := r.h.SleepCell(id, b)
	if !r.h.StartJob(id, job.Record{Kind: job.Sleep, Target: best, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
		r.h.SetBuilding(best, b.Release(uint64(id)))
		return false // the bed is out of reach
	}
	return true
}

// partnerBed is the bed and slot pawn id should sleep in for its partner:
// the partner's double bed, its own double bed, or a free one (nearest).
func (r *Beds) partnerBed(id entity.EntityID) (entity.EntityID, int, bool) {
	p := r.h.Partner(id)
	if p == 0 {
		return 0, 0, false
	}
	x, y := r.h.Pos(id)
	var own, free entity.EntityID
	ownSlot, freeD := 0, 1<<30
	for _, bid := range r.h.BuildingIDs() {
		b, ok := r.h.Building(bid)
		if !ok || b.Kind.Sleepers() != 2 || b.ForPrisoners || b.Forbidden || care.IsMedicalBed(b) {
			continue // medical beds are for patients (care/rest.go)
		}
		if ps := b.SlotOf(uint64(p)); ps >= 0 {
			if o := slotOwner(b, 1-ps); o == 0 || o == uint64(id) {
				return bid, 1 - ps, true
			}
			continue
		}
		if s := b.SlotOf(uint64(id)); s >= 0 && own == 0 {
			own, ownSlot = bid, s
			continue
		}
		if b.Claimant == 0 && b.Claimant2 == 0 {
			if d := pathing.Dist(x, y, b.X, b.Y); d < freeD {
				free, freeD = bid, d
			}
		}
	}
	switch {
	case own != 0:
		return own, ownSlot, true
	case free != 0:
		return free, 0, true
	}
	return 0, 0, false
}

// sleepInPartnerBed sends pawn id to slot of bed bid, giving up any other
// bed; false (nothing changed) leaves SleepIn to choose as usual.
func (r *Beds) sleepInPartnerBed(id, bid entity.EntityID, slot int) bool {
	b, ok := r.h.Building(bid)
	if !ok || slot < 0 || slot >= b.Kind.Sleepers() {
		return false
	}
	if o := slotOwner(b, slot); o != 0 && o != uint64(id) {
		return false
	}
	orig := b
	prev, hadPrev := r.h.SleptAt(id)
	b = withSlot(b.Release(uint64(id)), slot, uint64(id))
	r.h.SetBuilding(bid, b)
	r.h.SetSleptAt(id, bid)
	sx, sy := construction.SleepCell(b.Kind, b.Facing, b.X, b.Y, slot)
	if !r.h.StartJob(id, job.Record{Kind: job.Sleep, Target: bid, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
		r.h.SetBuilding(bid, orig)
		if hadPrev {
			r.h.SetSleptAt(id, prev)
		} else {
			r.h.ForgetSleptAt(id)
		}
		return false
	}
	for _, other := range r.h.BuildingIDs() {
		if other == bid {
			continue
		}
		if ob, ok := r.h.Building(other); ok && ob.SlotOf(uint64(id)) >= 0 {
			r.h.SetBuilding(other, ob.Release(uint64(id)))
		}
	}
	return true
}

// AssignOwner makes owner (0: nobody) hold slot of bed: the owner gives
// up any other bed, and whoever held the slot loses it. A medical bed has
// no owner (care/rest.go). The command.AssignBedOwner handler.
func (r *Beds) AssignOwner(bed, owner entity.EntityID, slot int) {
	b, ok := r.h.Building(bed)
	if !ok || !b.Kind.Sleeps() || slot < 0 || slot >= b.Kind.Sleepers() {
		return
	}
	if owner != 0 {
		if care.IsMedicalBed(b) || !r.h.MayOwnBed(owner) {
			return
		}
		if slotOwner(b, slot) == uint64(owner) {
			return
		}
		r.h.ReleaseBed(owner)
		b, _ = r.h.Building(bed)
	}
	r.h.SetBuilding(bed, withSlot(b, slot, uint64(owner)))
}
