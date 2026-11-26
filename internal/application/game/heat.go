// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"

	"rimworld-go/internal/application/game/bodytemp"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Campfire fuel and heat.
const (
	// CampfireCapacity is how much wood a campfire holds.
	CampfireCapacity = 20
	// TicksPerWood is how long one log burns (about ten logs a day).
	TicksPerWood = 6000
	// campfireBank: a fire banks down (burning no fuel, adding no heat)
	// once the air around it reaches 28 C (wiki: Campfire, it heats up to
	// 28 C; game/thermo), and campfireRelight: it flares up again when the
	// air cools below 18 C (original tuning). The wide gap, and banking
	// only on the campfireBankEvery hour marks (4 h, a multiple of the
	// climate interval), keep the wiki-strength fire from banking and
	// flaring every hour even in a cold snap: at most 6 banks a day.
	campfireBank      = 280
	campfireRelight   = 180
	campfireBankEvery = 10000
	// warmWait is the longest a cold colonist waits in a warm spot.
	warmWait = 2500
)

// --- room temperature ----------------------------------------------------

// heatRooms moves every room's air one equalisation step (game/thermo,
// TEMP-1: walls, roofs, open doors and vents, heat sources).
func (g *Game) heatRooms() {
	g.ensureRooms()
	g.thermo.Step()
	scorch(g) // fire.go (TEMP-6): ignition and burns in air past 235 C or 150 C over the comfortable range
}

// --- exposure ----------------------------------------------------------------

// expose steps hypothermia and heatstroke (TEMP-3, the wiki's per-60-tick
// rates: domain/pawn/exposure) for every colonist, prisoner, guest and
// raider on the map, once a climate interval, and sets them on the body
// (body.SetExposure), whose stages move the capacities: the extreme stage
// downs through consciousness, and full severity kills. Hypothermia past
// 37 % in air at or below 0 °C may frostbite an extremity (body stream).
// Animals: game/bodytemp.
func (g *Game) expose() {
	now := uint64(g.clock.Now())
	for _, id := range slices.Concat(g.pawnIDs, g.raiderIDs) {
		v, ok := g.vital.Get(id)
		if !ok || v.Status == health.Dead || !g.onMap(id) {
			continue
		}
		pos, _ := g.positions.Get(id)
		t, e := int(g.temperatureAt(pos.X, pos.Y)), g.exposure[id]
		e.Hypo, e.Heat = exposure.Step(e.Hypo, e.Heat, t, climateInterval, g.bandsOf(id))
		g.exposure.Set(id, e)
		if e.Hypo >= exposure.Full || e.Heat >= exposure.Full {
			cause, how := health.CauseHypothermia, "%s froze to death"
			if e.Heat >= exposure.Full {
				cause, how = health.CauseHeatstroke, "%s died of heatstroke"
			}
			delete(g.exposure, id)
			g.die(id, cause)
			g.message(pos.X, pos.Y, how, g.nameOf(id))
			continue
		}
		if g.bodies[id] == nil && e == (bodytemp.Sev{}) {
			continue
		}
		b := g.ensureBody(id, &v)
		if b == nil {
			continue
		}
		changed := body.SetExposure(b, body.CondHypothermia, exposure.Permille(e.Hypo))
		changed = body.SetExposure(b, body.CondHeatstroke, exposure.Permille(e.Heat)) || changed
		if p := exposure.FrostbiteChance(e.Hypo, bodytemp.FrostbiteFrom); p > 0 && t <= body.FrostbiteMaxTemp {
			if rng := g.stream(streamBody); int32(rng.IntN(1_000_000)) < p*climateInterval/60 {
				if part, _, ok := body.StrikeFrostbite(b, rng.IntN, now); ok {
					g.woundOpened(id)                       // infection.go: its infection roll
					if !g.isRaider(id) || g.prison.Is(id) { // a hostile's frostbite is no news
						g.message(pos.X, pos.Y, "%s's %s is frostbitten.", g.nameOf(id), b.PartLabel(part))
					}
					changed = true
				}
			}
		}
		if changed {
			g.bodyChanged(id, b, &v) // body.go: stats, patients
		}
		if changed || v.Status == health.Downed { // AIL-002: lying, it tries to stand every interval
			exposureDowned(g, id, b, v, e.Heat > e.Hypo)
		}
	}
}

// exposureDowned downs a standing pawn its exposure incapacitated (the
// capacity rule, bleed.go), or stands one lying from exposure once the
// capacities recover past the band.
func exposureDowned(g *Game, id entity.EntityID, b *body.Body, v health.Record, heat bool) {
	cause, kind, how := health.CauseHypothermia, evlog.Hypothermia, "%s collapsed from hypothermia"
	if heat {
		cause, kind, how = health.CauseHeatstroke, evlog.Heatstroke, "%s collapsed from heatstroke"
	}
	switch {
	case v.Status == health.Alive && g.downsNow(id, v, b):
		g.logEvent(id, logEntry{Kind: kind})
		g.down(id, cause)
		pos, _ := g.positions.Get(id)
		g.message(pos.X, pos.Y, how, g.nameOf(id))
	case v.Status == health.Downed && (v.Cause == health.CauseHypothermia || v.Cause == health.CauseHeatstroke) && g.standsUp(id, v):
		v.Status, v.Cause = health.Alive, health.CauseNone
		g.vital.Set(id, v)
	}
}

// exposureView is a pawn's hypothermia and heatstroke (permille) and its
// frostbitten parts, for the pawn views.
func exposureView(g *Game, id entity.EntityID) (int, int, int) {
	e := g.exposure[id]
	return int(exposure.Permille(e.Hypo)), int(exposure.Permille(e.Heat)), body.FrostbiteParts(g.bodies[id])
}

// exposureNoted is exposureView plus a downed pawn's ExposureNote (AIL-004).
func exposureNoted(g *Game, id entity.EntityID, downed bool) (hypo, heat, frost int, note string) {
	if hypo, heat, frost = exposureView(g, id); downed {
		note = g.exposure[id].Note(chillHost{g}.AirAt(id), g.bandsOf(id))
	}
	return hypo, heat, frost, note
}

// careReach's care.Climate (AIL-1); its ExposureWait is noComfort.
func (h careReach) AirHeals(id entity.EntityID, x, y, margin int) bool {
	return !h.g.exposure[id].Held(exposure.Trivial, int(h.g.temperatureAt(x, y)), h.g.bandsOf(id).Inset(margin))
}
func (h careReach) HealingSpot(id entity.EntityID) (int, int, bool) { return h.g.warmSpot(id) }
func (h careReach) HealingSpotFor(patient, carrier entity.EntityID) (int, int, bool) {
	return thermalSpot(h.g, patient, carrier)
}
func (h careReach) GroundSafe(patient, carrier entity.EntityID, x, y int) bool {
	return thermalGround(h.g, patient, carrier, x, y)
}
func (h careReach) ExposureWait(id entity.EntityID) uint64           { return h.g.noComfort[id] }
func (h careReach) SetExposureWait(id entity.EntityID, until uint64) { h.g.noComfort[id] = until }

// chillHost is bodytemp.AnimalHost: animal exposure's reach into the game.
type chillHost struct{ g *Game }

func (h chillHost) AirAt(id entity.EntityID) int {
	pos, _ := h.g.positions.Get(id)
	return int(h.g.temperatureAt(pos.X, pos.Y))
}

func (h chillHost) Roll(n int) int { return h.g.stream(streamBody).IntN(n) }

func (g *Game) nameOf(id entity.EntityID) string {
	n, _ := g.names.Get(id)
	return n
}

// needsComfort includes serious injury even after entering healing air.
func (g *Game) needsComfort(id entity.EntityID) bool {
	return g.exposureDanger(id) || g.exposure[id].Held(exposure.Mild, chillHost{g}.AirAt(id), g.bandsOf(id))
}

func safeThermalCell(g *Game, id entity.EntityID, x, y int) bool {
	return g.bandsOf(id).Comfortable(int(g.temperatureAt(x, y))) && !g.fireNear(x, y, 1)
}

// warmSpot finds reachable comfortable air, not merely the right side of
// one temperature limit. Fire and a blocked/reserved cell remain unsafe.
func (g *Game) warmSpot(id entity.EntityID) (int, int, bool) {
	return thermalSpot(g, id, id)
}

// thermalGround is shared by discovery and final clinical admission. The
// patient's or carrier's own tile reservation does not obstruct the trip.
func thermalGround(g *Game, patient, carrier entity.EntityID, x, y int) bool {
	owner, taken := g.reserve.TileOwner(x, y)
	return g.grid.Walkable(x, y) && !g.bedCovering(x, y) && (!taken || owner == patient || owner == carrier) && safeThermalCell(g, patient, x, y)
}

func thermalSpot(g *Game, patient, reacher entity.EntityID) (int, int, bool) {
	g.ensureRooms()
	pos, _ := g.positions.Get(reacher)
	bands, out := g.bandsOf(patient), g.climate.Outdoor(g.clock.Now())
	return g.thermo.Spot(pos.X, pos.Y, func(r int) bool {
		t, ok := g.thermo.Air(r, out)
		return ok && bands.Comfortable(int(t))
	}, func(x, y int) bool {
		return thermalGround(g, patient, reacher, x, y)
	}, func(x, y int) bool { return g.reachable(reacher, x, y) })
}

// thermalRecovery reuses Warm; no additional saved job or priority state.
// Explicit orders and emergency responses have priority, and safe sleeping
// or self-tending already provides recovery without another interruption.
func thermalRecovery(g *Game, id entity.EntityID, j job.Record) bool {
	if j.Kind.Eating() {
		return false
	}
	if j.Kind == job.Warm && j.Active() {
		if safeThermalCell(g, id, j.TX, j.TY) {
			return false
		}
		g.interrupt(id, job.Interrupted)
		j = job.Record{}
		delete(g.noComfort, id)
	}
	if !g.needsComfort(id) {
		return false
	}
	pos, _ := g.positions.Get(id)
	safe := safeThermalCell(g, id, pos.X, pos.Y)
	if j.Kind == job.Sleep && (!g.exposureDanger(id) || safe && j.State == job.Do) || safe && j.Kind == job.Tend && j.Target == id && j.State == job.Do {
		return false
	}
	if n, _ := g.need.Get(id); n.Hunger <= needs.RavenousAt && !(j.Forced && j.Active()) && g.seekFood(id) {
		return false
	}
	x, y := pos.X, pos.Y
	if !safe {
		if g.noComfortUntil(id) {
			return false
		}
		var ok bool
		x, y, ok = g.warmSpot(id)
		if !ok {
			g.noComfort[id] = uint64(g.clock.Now()) + warmRetry
			return false
		}
	}
	if j.Forced && j.Active() {
		return false
	}
	if j.Active() {
		g.interrupt(id, job.Interrupted)
	}
	if g.startJob(id, job.Record{Kind: job.Warm, TX: x, TY: y, WorkLeft: warmWait}) {
		return true
	}
	g.noComfort[id] = uint64(g.clock.Now()) + warmRetry
	return false
}

// exposureDanger: hypothermia or heatstroke bad enough to get a sleeper
// out of bed (the serious stage; a little chill only matters once awake).
func (g *Game) exposureDanger(id entity.EntityID) bool {
	e := g.exposure[id]
	return e.Hypo >= exposure.Danger || e.Heat >= exposure.Danger
}

// warmRetry is how long a colonist that found no bearable spot waits
// before looking again.
const warmRetry = 1000

func (g *Game) noComfortUntil(id entity.EntityID) bool {
	return uint64(g.clock.Now()) < g.noComfort[id]
}

// doWarm waits in the warm spot until the exposure has faded.
func (g *Game) doWarm(id entity.EntityID, j job.Record) {
	e := g.exposure[id]
	j.WorkLeft--
	pos, _ := g.positions.Get(id)
	if j.WorkLeft <= 0 || e == (bodytemp.Sev{}) || !safeThermalCell(g, id, pos.X, pos.Y) {
		j.State = job.Complete
	}
	g.jobs.Set(id, j)
}

// SkipTo jumps the calendar forward to day (0-based) at hour without
// simulating the time in between (dev scenarios and fixtures).
func (g *Game) SkipTo(day, hour int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	target := clock.Tick(day*clock.TicksPerDay + hour*clock.TicksPerHour)
	if target > g.clock.Now() {
		g.clock.Advance(int(target - g.clock.Now()))
	}
}

// ClearPlants removes every plant in the rectangle (dev scenarios).
func (g *Game) ClearPlants(x0, y0, x1, y1 int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	for _, id := range append([]entity.EntityID(nil), g.plantIDs...) {
		if p, ok := g.plants.Get(id); ok && p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
			g.removePlant(id)
		}
	}
}
