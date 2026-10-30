package raid

import (
	"fmt"

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
	ThreatRolls  = 8
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
		t.WolfPack(2 + t.h.Roll(2))
	case ThreatManhunters:
		t.s.ManhunterPack(3 + t.h.Roll(3))
	case ThreatHive:
		t.hives.Swarm()
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

// Big-threat weights. wiki: Events, among the big threats a raid weighs
// 7.4 and each of the two crashed ship parts (defoliator and droner) 2.0:
// the one ship part here takes both, 4.0. Every hostile human faction
// weighs 1 as a raider and the mechanoids 1 from MechRaidMinPoints (0
// below). The ship part's own 400-point gate is the incident's.
const (
	raidWeight     = 74
	shipPartWeight = 40
)

// Major splits a raid face for a raid of points with humans hostile human
// factions (a legacy colony: one): a crashed ship part (raidWeight :
// shipPartWeight), then mechanoids against the factions, else people.
func (t *Threats) Major(points, humans int) Major {
	if t.h.Roll(raidWeight+shipPartWeight) < shipPartWeight {
		return MajorShipPart
	}
	if points >= MechRaidMinPoints && t.h.Roll(max(humans, 1)+1) == 0 {
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
	switch t.Major(pts, 1) {
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

// WolfPack sends n hungry pack predators of one season-fitting species
// in from an edge.
func (t *Threats) WolfPack(n int) {
	s, ok := t.w.PickSpecies(func(s animal.Species) bool { return animal.DefOf(s).Pack })
	if !ok {
		return
	}
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
		t.h.Threat("Starving predators", ex, ey, fmt.Sprintf("A pack of %d starving %s is prowling near the colony.", spawned, t.w.Plural(s)))
	}
}
