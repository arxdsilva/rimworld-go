// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
)

type peerMealHost struct{ g *Game }

func (h peerMealHost) Standing(id entity.EntityID) bool { return h.g.standingPawn(id) }
func (h peerMealHost) Donor(id entity.EntityID) bool {
	g := h.g
	return g.colonist(id) && g.standingPawn(id) && !g.drafted(id) && !g.breaking(id) && !g.visitor(id)
}
func (h peerMealHost) Position(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h peerMealHost) ClaimedByOther(id, donor entity.EntityID) bool {
	owner, held := h.g.reserve.EntityOwner(donor)
	return held && owner != id
}
func (h peerMealHost) Visible(x, y int) bool { return !h.g.fogged(x, y) }
func (h peerMealHost) BesideSpot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.besideSpot(id, x, y)
}
func (h peerMealHost) InArea(id entity.EntityID, x, y int) bool { return h.g.inArea(id, x, y) }
func (h peerMealHost) Picker(id entity.EntityID) diet.Picker {
	g := h.g
	return g.sk.diet.Picker(thoughtsOf(g), id, body.Malnutrition(g.bodies[id]) > 0, cannibal(g, id)).Under(dietEaters{g})
}

type peerTransferHost struct{ peerMealHost }

func (h peerTransferHost) Stack(id entity.EntityID) (item.Stack, bool) { return h.g.stacks.Get(id) }
func (h peerTransferHost) ClaimPortion(id, stack entity.EntityID) bool {
	return h.g.reserve.TryCount(id, stack, 1, 1)
}
func (h peerTransferHost) ReleaseDonor(id, donor entity.EntityID) {
	h.g.reserve.ReleaseEntity(id, donor)
}
func (h peerTransferHost) PutStack(id entity.EntityID, st item.Stack) { h.g.putStack(id, st) }
func (h peerTransferHost) SetJob(id entity.EntityID, j job.Record)    { h.g.jobs.Set(id, j) }
func (h peerTransferHost) FailMeal(id entity.EntityID, j job.Record) {
	h.g.reserve.ReleaseOwner(id)
	j.State, j.Fail = job.Fail, job.TargetGone
	h.g.jobs.Set(id, j)
}

func (g *Game) peerPackedMeal(id, eater entity.EntityID) (entity.EntityID, uint32, int, int, bool) {
	return sense.FindPeerMeal(senseOf(g).Gear, peerMealHost{g}, id, eater)
}
func doEatFromPeerPack(g *Game, id entity.EntityID, j job.Record) {
	sense.EatFromPeerPack(senseOf(g).Gear, peerTransferHost{peerMealHost{g}}, id, j)
}
func (g *Game) peerMealForAssignment(id entity.EntityID) (entity.EntityID, uint32, int, int, bool) {
	c := g.haul.peerMeal
	g.haul.peerMeal = sense.PeerChoice{}
	return sense.AssignPeerMeal(senseOf(g).Gear, peerMealHost{g}, id, uint64(g.clock.Now()), c)
}
