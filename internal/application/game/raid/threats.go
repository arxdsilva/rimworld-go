// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
)

// Threats. A simple storyteller sends a small raid a few days in, then
// every five to nine days, growing slowly with time; in winter, hungry
// predator packs come too, and manhunter packs and hive swarms (hive.go)
// at times (PickThreat has the odds).

// Threat is what the storyteller sends.
type Threat uint8

const (
	ThreatRaid Threat = iota
	ThreatPack
	ThreatManhunters
	ThreatHive
	ThreatMad // one wild animal near home goes manhunter (SIM-025)
)

// ThreatRolls is the storyteller's die. Odds (SIM-019): in winter a
// predator pack 4/8 and, from day 12, a hive swarm 1/8; otherwise a
// manhunter pack 2/8 from day 8 and a hive swarm 1/8 from day 12; a raid
// the rest (so raids went from 3/4 to 5/8 outside winter and from 1/2 to
// 3/8 in winter, once hives can come). SIM-025 takes one more raid face
// in every season for a mad animal from day 6 (raids 4/8 outside winter,
// 2/8 in it, from day 12); a mad animal that cannot happen (cooling down,
// no wild animal near home) is a raid after all.
const (
	ThreatRolls  = incident.ThreatRolls
	HiveFirstDay = 12
	MadRoll      = ThreatRolls - 2
)

const (
	raidGapMin    = 5 // days
	raidGapSpread = 5 // days, added at random
)

// PickThreat maps a roll of the die (0..ThreatRolls-1) to a threat.
func PickThreat(roll, day int, winter bool) Threat {
	switch {
	case winter && roll < 4:
		return ThreatPack
	case !winter && roll < 2 && day >= 8:
		return ThreatManhunters
	case roll == ThreatRolls-1 && day >= HiveFirstDay:
		return ThreatHive
	case roll == MadRoll && day >= incident.DefOf(incident.MadAnimal).MinDay:
		return ThreatMad
	}
	return ThreatRaid
}

// Threats is the legacy storyteller (a colony without a setup) and the
// raid plans and predator packs every storyteller uses.
type Threats struct {
	h     Host
	w     Wild
	s     Story
	hives *Hives
	mechs *Mechs // mech.go: mechanoid raids
	// picked: Plan chose the strategy of the raid being sent, which
	// Settle completes (the faction's tech and points, the arrival). Set
	// and cleared within one raid's sending: never saved.
	picked bool
}

// Fire sends the legacy storyteller's threat at now and returns the tick
// of the next one.
func (t *Threats) Fire(now uint64) uint64 {
	day := int(now / clock.TicksPerDay)
	winter := temperature.Season(clock.Tick(now)) == "Winter"
	switch PickThreat(t.h.Roll(ThreatRolls), day, winter) {
	case ThreatPack:
		t.WolfPack((2 + t.h.Roll(2)) * 100)
	case ThreatManhunters:
		t.s.ManhunterPack((3 + t.h.Roll(3)) * 100)
	case ThreatHive:
		if !t.hives.Infest() {
			t.s.ScaledRaid(day)
		}
	case ThreatMad:
		if !t.s.MadAnimal(now) { // incidents.go
			t.s.ScaledRaid(day)
		}
	default:
		t.bigRaid(day, now)
	}
	return now + uint64((raidGapMin+t.h.Roll(raidGapSpread))*clock.TicksPerDay+t.h.Roll(clock.TicksPerDay))
}

// Major is what a raid face of the die becomes (DEF-4).
type Major uint8

const (
	MajorRaid     Major = iota // people raid
	MajorMechs                 // a mechanoid raid (mech.go)
	MajorShipPart              // a crashed ship part (shippart.go)
)

// Major splits a raid face for a raid of points with humans hostile human
// factions (a legacy colony: one) by the incident defs def gives (the
// big-threat weights are data, incident.Raid's and the UnderRaid events'):
// a crashed ship part (its Weight against Raid's), then mechanoids from
// their MinPoints (their Weight against each hostile human faction's 1),
// else people. It always draws once, and once more from the mechanoids'
// MinPoints; a weight of 0 never comes.
func (t *Threats) Major(points, humans int, def func(incident.ID) incident.Def) Major {
	ship, mech := def(incident.CrashedShipPart).Weight, def(incident.MechRaid)
	if t.h.Roll(max(def(incident.Raid).Weight+ship, 1)) < ship {
		return MajorShipPart
	}
	if points >= mech.MinPoints && t.h.Roll(max(humans, 1)+mech.Weight) < mech.Weight {
		return MajorMechs
	}
	return MajorRaid
}

// MechRaid sends a mechanoid raid of points: drop pods or walking in by
// the map entry weights (raiddef.PickArrival). It returns how many came.
func (t *Threats) MechRaid(points int, now uint64) int {
	if t.mechs == nil {
		return 0
	}
	return t.mechs.Raid(points, raiddef.PickArrival(t.h.Roll(raiddef.ArrivalRolls), faction.Spacer), now)
}

// bigRaid is the legacy storyteller's raid face: a ship part, mechanoids
// or people (Major); a ship part or mech raid that cannot come is a
// people raid.
func (t *Threats) bigRaid(day int, now uint64) {
	pts := t.s.RaidPoints(day)
	switch t.Major(pts, 1, incident.DefOf) {
	case MajorShipPart:
		if t.s.ShipPart() {
			return
		}
	case MajorMechs:
		if t.MechRaid(pts, now) > 0 {
			return
		}
	}
	t.s.ScaledRaid(day)
}

// Plan chooses the storyteller's raid strategy for colony day day:
// sappers against a shut-in colony, now and then a siege, a staged
// assault, thieves or burners, else an assault (raiddef.PickStrategy).
// Settle completes it when the raid is sent.
func (t *Threats) Plan(day int) Plan {
	t.picked = true
	return raiddef.PickStrategy(t.h.Roll(raiddef.StrategyRolls), day, t.s.Enclosed)
}

// Settle fits a raid being sent as group grp to the faction that sends it
// (tech) and its points: the group's flee share is rolled, and a raid the
// storyteller planned (Plan) gets a strategy the faction can carry out
// and an arrival (raiddef.Fit, raiddef.PickArrival). A raid from a dev
// hook or an event keeps its plan and walks in.
func (t *Threats) Settle(grp *Group, tech faction.Tech, points int) {
	grp.FleeLoss = raiddef.FleeLoss(t.h.Roll(raiddef.FleeLossRolls))
	if !t.picked {
		return
	}
	t.picked = false
	grp.Plan = raiddef.Fit(grp.Plan, tech, points)
	grp.Arrival = raiddef.PickArrival(t.h.Roll(raiddef.ArrivalRolls), tech)
	if grp.Plan == Siege || grp.Plan == Sappers || grp.Plan == Stage {
		grp.Arrival = raiddef.WalkInEdge // they camp, dig or gather from the edge
	}
}

// WolfPack sends a pack of starving predators of one species of the
// map's biome and season, as many as threat points buy (animal.PackSize,
// EAI-005; two at least), in from an edge: pack hunters where the biome
// has them, else any predator it has (a lone hunter's kind forms a pack
// too).
func (t *Threats) WolfPack(points int) {
	s, ok := t.w.PickSpecies(func(s animal.Species) bool { return animal.DefOf(s).Pack })
	if !ok {
		s, ok = t.w.PickSpecies(packHunter)
	}
	if !ok {
		return
	}
	n := max(animal.PackSize(points, s), 2) // a pack: two at least
	ex, ey, ok := t.w.EdgeTile()
	if !ok {
		return
	}
	spawned := 0
	for i := 0; i < n; i++ {
		for try := 0; try < 10; try++ {
			x, y := ex+t.h.Roll(5)-2, ey+t.h.Roll(5)-2
			if !t.h.InBounds(x, y) {
				continue
			}
			if id, err := t.w.SpawnAnimal(s, x, y); err == nil {
				a := t.w.Animal(id)
				a.Hunger = animal.DefOf(s).HungerTicks
				t.w.SetAnimal(id, a)
				spawned++
				break
			}
		}
	}
	if spawned > 0 {
		t.h.Threat("Starving predators", ex, ey, "A pack of %d starving %s is prowling near the colony.", spawned, t.w.Plural(s))
	}
}

// packHunter: a predator that may come as a starving pack (never a
// legendary, a hive insect or a machine).
func packHunter(s animal.Species) bool {
	d := animal.DefOf(s)
	return d.Predator && !d.Legendary && !d.Hive && !d.Mech
}
