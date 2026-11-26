// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package sense

import (
	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// PeerSearch supplies current map and dietary facts, not pack ownership.
// Gear owns pack contents and extraction provenance.
type PeerSearch interface {
	Standing(id entity.EntityID) bool
	Donor(id entity.EntityID) bool
	Position(id entity.EntityID) (int, int, bool)
	ClaimedByOther(id, donor entity.EntityID) bool
	Visible(x, y int) bool
	BesideSpot(id entity.EntityID, x, y int) (int, int, bool)
	InArea(id entity.EntityID, x, y int) bool
	Picker(id entity.EntityID) diet.Picker
}

// PeerChoice is derived, per-map same-tick scratch, never persisted.
type PeerChoice struct {
	Eater, Donor entity.EntityID
	Entry        uint32
	X, Y         int
	At           uint64
}

func edible(pick diet.Picker, st *item.Stack) bool {
	return st.Qty > 0 && st.Kind.IsFood() && !st.Rotten() && pick.Takes(st)
}

func eligibleDonor(g Gear, h PeerSearch, id, other entity.EntityID) bool {
	return other != id && h.Donor(other) && g.s.FromPack[other] == 0 && !h.ClaimedByOther(id, other)
}

// FindPeerMeal searches only food-bearing packs; tie order stays canonical.
func FindPeerMeal(g Gear, h PeerSearch, id, eater entity.EntityID) (donor entity.EntityID, entry uint32, x, y int, ok bool) {
	if !h.Standing(id) {
		return
	}
	px, py, here := h.Position(id)
	if !here {
		return
	}
	var pick diet.Picker
	picked := false
	bestScore := 0
	for _, other := range g.FoodDonors() {
		if !eligibleDonor(g, h, id, other) {
			continue
		}
		ox, oy, present := h.Position(other)
		if !present || !h.Visible(ox, oy) {
			continue
		}
		if !picked {
			pick = h.Picker(eater)
			picked = true
		}
		pack := g.Pack(other)
		eligible := false
		for _, e := range pack.Items {
			if edible(pick, &e.Stack) {
				eligible = true
				break
			}
		}
		if !eligible {
			continue
		}
		bx, by, possible := h.BesideSpot(id, ox, oy)
		if !possible || !h.InArea(id, bx, by) {
			continue
		}
		distance := pathing.Dist(px, py, bx, by)
		for _, e := range pack.Items {
			if !edible(pick, &e.Stack) {
				continue
			}
			score := pick.Score(&e.Stack, distance)
			if !ok || score > bestScore || score == bestScore && (other < donor || other == donor && e.ID < entry) {
				bestScore, donor, entry, x, y, ok = score, other, e.ID, bx, by, true
			}
		}
	}
	return
}

func peerEntry(g Gear, h PeerSearch, eater, donor entity.EntityID, entry uint32) bool {
	pick := h.Picker(eater)
	for _, e := range g.Pack(donor).Items {
		if e.ID == entry && edible(pick, &e.Stack) {
			return true
		}
	}
	return false
}

// AssignPeerMeal consumes a speculative choice only when all live
// eligibility facts remain true, otherwise repeats the native search.
func AssignPeerMeal(g Gear, h PeerSearch, id entity.EntityID, now uint64, c PeerChoice) (entity.EntityID, uint32, int, int, bool) {
	if c.Eater == id && c.At == now && c.Donor != 0 && eligibleDonor(g, h, id, c.Donor) {
		x, y, ok := h.Position(c.Donor)
		if ok && h.Visible(x, y) && max(x-c.X, c.X-x, y-c.Y, c.Y-y) <= 1 && h.InArea(id, c.X, c.Y) &&
			peerEntry(g, h, id, c.Donor, c.Entry) {
			return c.Donor, c.Entry, c.X, c.Y, true
		}
	}
	return FindPeerMeal(g, h, id, id)
}

// PeerTransfer is the native physical stack/claim transition. Failed
// handoffs restore the extracted item instead of replacing pack provenance.
type PeerTransfer interface {
	Stack(entity.EntityID) (item.Stack, bool)
	ClaimPortion(eater, stack entity.EntityID) bool
	ReleaseDonor(eater, donor entity.EntityID)
	PutStack(entity.EntityID, item.Stack)
	SetJob(entity.EntityID, job.Record)
	FailMeal(entity.EntityID, job.Record)
}

func EatFromPeerPack(g Gear, h interface {
	PeerSearch
	PeerTransfer
}, id entity.EntityID, j job.Record) {
	donor := j.Target
	x, y, own := h.Position(id)
	dx, dy, exists := h.Position(donor)
	if !own || !exists || !eligibleDonor(g, h, id, donor) || max(x-dx, dx-x, y-dy, dy-y) > 1 ||
		!peerEntry(g, h, id, donor, uint32(j.BillIndex)) {
		h.FailMeal(id, j)
		return
	}
	stack, ok := g.TakeOut(donor, uint32(j.BillIndex), 1)
	if !ok {
		h.FailMeal(id, j)
		return
	}
	st, found := h.Stack(stack)
	if !found || st.Qty != 1 || !h.ClaimPortion(id, stack) {
		if !g.BackToPack(donor, stack) {
			g.r.Lay(donor, stack)
		}
		h.FailMeal(id, j)
		return
	}
	delete(g.s.FromPack, donor)
	h.ReleaseDonor(id, donor)
	st.Carrier, st.X, st.Y = id, x, y
	h.PutStack(stack, st)
	h.SetJob(id, job.Record{Kind: job.Eat, State: job.Do, Target: stack, Carry: stack, TX: x, TY: y, WorkLeft: needs.EatWorkTicks})
}
