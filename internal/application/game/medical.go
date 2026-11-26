// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/entity"
)

// Medical care (Doctor work) lives in the care package (care/rescue.go,
// care/tend.go): rescue to a bed and tending. What stays here is the bed
// and spot lookups the rest of the game shares. Animals have no wounds
// yet: a tend mends some of their injury at once (animalTendHeal).

const (
	// ReasonMedicalBed: a healthy colonist can't be ordered into a
	// medical bed.
	ReasonMedicalBed = care.ReasonMedicalBed
	// TendWork is the ticks a doctor spends tending one patient.
	TendWork = care.TendWork
	// TendHeal is the injury a plain tend mends on an animal at once.
	TendHeal = 8
	// TendTicks is how long a tend shows as tended (PawnView.Tended) and
	// keeps an animal from being tended again.
	TendTicks = care.TendTicks
)

// animalTendHeal is the injury a tend of quality q (permille) mends on an
// animal at once: TendHeal × (250 + q) / 500, so about 9 for a plain tend
// by a level-8 doctor, 14 with herbal and 20 with industrial medicine
// (original tuning near the old care tiers, until animals get wounds in
// MED-09).
func animalTendHeal(q int32) int { return TendHeal * int(250+q) / 500 }

// bedUnder is the bed covering a pawn's tile that it has claimed.
func (g *Game) bedUnder(id entity.EntityID) entity.EntityID {
	pos, _ := g.positions.Get(id)
	bid := g.bedUnderIndexed(id, pos) // jobindex.go
	if checkIndex(g) {
		if s := g.bedUnderScan(id, pos); s != bid {
			panic(fmt.Sprintf("bedUnder(%d) index %d, scan %d", id, bid, s))
		}
	}
	return bid
}

func (g *Game) inBed(id entity.EntityID) bool { return g.bedUnder(id) != 0 }

func (g *Game) tended(id entity.EntityID) bool {
	return uint64(g.clock.Now()) < g.care.Doctor.TendedUntil(id)
}

// Injure deals damage to a colonist as if from an attack (dev scenarios
// and fixtures).
func (g *Game) Injure(id entity.EntityID, dmg int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.hurt(id, dmg, 0)
}

// InjureCut deals dmg to id as a cut (dev scenarios and fixtures): unlike
// Injure's bruise it bleeds until tended (MED-01).
func (g *Game) InjureCut(id entity.EntityID, dmg int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.hurtAs(id, dmg, 0, body.Cut, health.CauseInjury)
}

// BuildNow places a finished building (dev scenarios and fixtures).
func (g *Game) BuildNow(kind, stuff, facing string, x, y int) bool {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	b, ok := buildableByName(kind)
	if !ok {
		return false
	}
	st, ok := stuffByName(stuff)
	if !ok {
		return false
	}
	f := parseFacingName(facing)
	if ok, _ := g.canPlaceAt(b, f, x, y); !ok {
		return false
	}
	g.placeFrameStuff(b, st, f, x, y)
	if b.Instant() {
		return g.layerBldgAt(x, y, construction.DefOf(b).Layer) != 0
	}
	fid := g.layerFrameAt(x, y, construction.DefOf(b).Layer)
	fr, ok := g.frames.Get(fid)
	if !ok {
		return false
	}
	g.completeFrame(fid, fr)
	return true
}

func buildableByName(name string) (construction.Kind, bool) {
	for _, b := range Buildables() {
		if b.Kind == name {
			return construction.Kind(b.ID), true
		}
	}
	return 0, false
}

// besideSpot is the reachable walkable tile next to (x, y), nearest to
// pawn id, that is not under a bed or in a doorway (a pawn standing in a
// doorway holds the door open).
func (g *Game) besideSpot(id entity.EntityID, x, y int) (int, int, bool) {
	pos, _ := g.positions.Get(id)
	return pathstage.NearestBeside(pos.X, pos.Y, x, y, func(nx, ny int) bool {
		return cellStandable(g, nx, ny) && !g.bedCovering(nx, ny) && !doorway(g, nx, ny)
	}, func(nx, ny int) bool { return g.reachable(id, nx, ny) }, func(nx, ny int) bool { return cellStandable(g, nx, ny) })
}

func (g *Game) bedCovering(x, y int) bool {
	on := g.bedCoveringIndexed(x, y) // jobindex.go
	if checkIndex(g) {
		if s := g.bedCoveringScan(x, y); s != on {
			panic(fmt.Sprintf("bedCovering(%d,%d) index %v, scan %v", x, y, on, s))
		}
	}
	return on
}

func (g *Game) bedCoveringScan(x, y int) bool {
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && (b.Kind.Sleeps() || b.Kind.AnimalSleeps()) &&
			construction.Covers(b.Kind, b.Facing, b.X, b.Y, x, y) {
			return true
		}
	}
	return false
}

// DefaultCare is every group's default care level (medcare.Policy), in
// medcare.Group order (colonists, prisoners, guests, animals). Read-only.
func (g *Game) DefaultCare() [medcare.NumGroups]uint8 {
	return g.care.Policy.Defaults()
}

// Bed rest itself lives in care/rest.go (PLAY-020); what stays here is
// the load-time migration and the shared helpers.

// healingInjury is the part of a pawn's injury (body b, vital v) rest
// can still heal: its open wounds, plus any Injury set past the body from
// outside (all of it with no body yet). Scars and lost parts never heal.
func healingInjury(b *body.Body, v health.Record) int { return body.HealingDamage(b, v.Injury) }

// lyingInBed: id lies in bed bid (b) now: asleep in it, or down on its
// sleeping cell (a check that needs no index, for loading).
func (g *Game) lyingInBed(id, bid entity.EntityID, b construction.Building) bool {
	if j, _ := g.jobs.Get(id); j.Kind == job.Sleep && j.Target == bid && j.Active() {
		return true
	}
	v, _ := g.vital.Get(id)
	slot := construction.BedSlot(b, uint64(id))
	if v.Status != health.Downed || slot < 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	x, y := construction.SleepCell(b.Kind, b.Facing, b.X, b.Y, slot)
	return pos.X == x && pos.Y == y
}

// migrateMedicalBeds: a save from before medical beds (v3 and older)
// gets every hospital bed medical. A colonist lying in one keeps it for
// its stay; any other colonist owner gives it up (a prisoner keeps its
// prison bed).
func (g *Game) migrateMedicalBeds() {
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.HospitalBed {
			continue
		}
		b.Medical = true
		for _, c := range [2]uint64{b.Claimant, b.Claimant2} {
			cid := entity.EntityID(c)
			switch {
			case c == 0 || g.prison.Is(cid):
			case g.lyingInBed(cid, bid, b):
				g.care.Rest.Hold(cid, bid)
			default:
				b = construction.ReleaseBed(b, c)
			}
		}
		g.bldgs.Set(bid, b)
	}
}

// crawlDowned has no persistent path: the existing saved downed tick and
// step timing determine the next retry; a stale bed is never claimed.
func crawlDowned(g *Game, id entity.EntityID, m motionRec, now uint64) {
	v, ok := g.vital.Get(id)
	b := g.bodies[id]
	if !ok || v.Status != health.Downed || now < v.DownedTick+care.CrawlDelay || b == nil || !care.CrawlReady(v, now, b.Capacity(body.Consciousness), b.Capacity(body.Manipulation)) {
		return
	}
	if g.care.Carry.Carrier(id) != 0 || g.inBed(id) {
		return
	}
	if _, held := g.reserve.EntityOwner(id); held {
		return
	}
	from, ok := g.positions.Get(id)
	if !ok {
		return
	}
	q := g.pathQuery(id)
	free := func(x, y int) bool { return q.Walkable(x, y) && cellStandable(g, x, y) && !g.fireNear(x, y, 0) }
	next, move, settled := g.care.CrawlBed(id)
	if !move && !settled && !g.prison.Is(id) {
		kinds := uint8(ckRaider | ckAnimal)
		if g.isRaider(id) {
			kinds = ckPawn
		}
		_, threat, found := g.nearestWhere(from.X, from.Y, 12, kinds, func(o entity.EntityID, _ Position) bool {
			if a, ok := g.animals.Get(o); ok {
				return g.hostile(a)
			}
			if g.isRaider(id) {
				return g.standingPawn(o)
			}
			return g.standingRaider(o)
		})
		if found {
			next, move = care.FleeCrawl(from.X, from.Y, threat.X, threat.Y, free)
		}
	}
	if move && free(next.X, next.Y) {
		g.stepPawn(id, &m, 0, from, Position{X: next.X, Y: next.Y}, now)
		m.stepped(now, care.CrawlStepTicks)
		g.motion.Set(id, m)
	}
}
