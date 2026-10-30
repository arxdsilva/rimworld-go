package raid

import "rimworld-go/internal/domain/faction"

// Raid rules (DEF-4). Pure functions of a roll and the raid's faction, so
// the application layer rolls on its own stream and the rules stay here.
// wiki: Raid (map entry weights, flee threshold, siege timings, drop-pod
// restrictions); the strategy weights and the staging delays are not
// published and are original tuning.

// Map entry. wiki: Raid gives walk in at the edge 45 %, walk in as an edge
// group 15 %, drop pods 30 % and haywired pods 10 % (mechanoid clusters
// only). People raids roll over ArrivalRolls: the two walk-ins are one
// arrival here and the drop pods split evenly between centre, edge and
// scattered (original: the split is not published).
const (
	ArrivalRolls = 90
	walkInRolls  = 60 // wiki: 45 + 15
	podRolls     = 10 // each of the three pod arrivals (wiki: 30 together)
)

// PickArrival maps roll (0..ArrivalRolls-1) to a raid's arrival. A tribe
// never uses pods; dropping on top of the colony is for the industrial
// factions and above (wiki: Raid, "Drop on top of you").
func PickArrival(roll int, tech faction.Tech) Arrival {
	if roll < walkInRolls || tech < faction.Industrial {
		return WalkInEdge
	}
	switch (roll - walkInRolls) / podRolls {
	case 0:
		return DropPodCentre
	case 1:
		return DropPodEdge
	}
	return DropPodScatter
}

// Strategy rolls (out of 100, the roll the legacy plan already made): the
// first two bands keep the old plan's odds (sappers under 50 against a
// shut-in colony from day 10, a siege under 30 from day 12); the bands
// past them are original tuning.
const (
	StrategyRolls = 100
	stageFrom     = 50 // 50..64: stage, then assault
	stealFrom     = 65 // 65..74: steal
	burnFrom      = 75 // 75..82: burn
	burnTo        = 83
	// StageFirstDay, StealFirstDay, BurnFirstDay: the new strategies come
	// once the colony has stood a few days (original tuning).
	StageFirstDay = 6
	StealFirstDay = 8
	BurnFirstDay  = 8
)

// PickStrategy maps roll (0..StrategyRolls-1) to a strategy on colony day
// day; enclosed is asked only when the roll could make sappers (it costs
// the caller a path search).
func PickStrategy(roll, day int, enclosed func() bool) Strategy {
	switch {
	case day >= 10 && roll < 50 && enclosed():
		return Sappers
	case day >= 12 && roll < 30:
		return Siege
	case day >= StageFirstDay && roll >= stageFrom && roll < stealFrom:
		return StageThenAssault
	case day >= StealFirstDay && roll >= stealFrom && roll < burnFrom:
		return Steal
	case day >= BurnFirstDay && roll >= burnFrom && roll < burnTo:
		return Burn
	}
	return ImmediateAssault
}

// SiegeMinPoints is the least a siege is sent with: two mortars and a crew
// (original tuning: the threshold is not published).
const SiegeMinPoints = 300

// Fit makes strategy s one the faction can carry out: a siege needs the
// industrial mortars and enough points (a tribe or a small raid stages
// instead).
func Fit(s Strategy, tech faction.Tech, points int) Strategy {
	if s == Siege && (tech < faction.Industrial || points < SiegeMinPoints) {
		return StageThenAssault
	}
	if s >= NumStrategies {
		return ImmediateAssault
	}
	return s
}

// Flee threshold. wiki: Raid, human raiders flee once 40 % to 70 % of
// their group is downed or killed, the share rolled per group.
const (
	FleeLossMin   = 40
	FleeLossRolls = 31 // 40..70
	// FleeLossLegacy is a group from before the roll (the old rule: half).
	FleeLossLegacy = 50
)

// FleeLoss is a group's flee threshold (percent lost) for roll
// (0..FleeLossRolls-1).
func FleeLoss(roll int) int { return FleeLossMin + min(max(roll, 0), FleeLossRolls-1) }

// Broken reports a group of size that has lost lossPct percent or more
// once only standing remain on their feet (pods still falling count as
// standing). A broken group stays broken: fleeing is one way.
func Broken(standing, size, lossPct int) bool {
	if size <= 0 {
		return false
	}
	if lossPct <= 0 {
		lossPct = FleeLossLegacy
	}
	return (size-standing)*100 >= size*lossPct
}

// Siege timings. wiki: Raid, a siege assaults after a random 1.5 to 3
// days, and every time a besieger takes damage there is an 8 % chance it
// assaults at once; it builds two mortars.
const (
	SiegeAssaultMin   = 36 // hours (1.5 days)
	SiegeAssaultRolls = 37 // 36..72 hours
	SiegeStormPct     = 8
	SiegeMortars      = 2
)

// Staging (StageThenAssault). wiki: Raid, the raiders stand around in a
// small group near where they came in and attack when they see a colonist
// nearby or their assault timer of 10 to 15 hours runs out (CBT-4). The
// wander radius and the spot are original tuning.
const (
	StageLimitMin   = 10 // the assault timer, hours: 10..15 (wiki)
	StageLimitRolls = 6  // hours added at random
	StageWander     = 7  // a raider further than this walks back to the spot
	StageSpotIn     = 10 // the spot lies this many tiles in from the edge
)

// Target priority. wiki: Raid, raiders attack a colonist or a powered
// turret they can reach; with none reachable they go for doors and walls.
// The order here is what a raider weighs when it picks a target.
type Target uint8

const (
	TargetColonist Target = iota // a colonist or an ally
	TargetTurret                 // a powered turret in sight, well closer
	TargetDoor                   // the way in when no colonist is reachable
	TargetWall
)

// PodFallTicks is how long a drop pod falls before it lands (original
// tuning: the wiki gives no time; the incoming sound leads by 120 ticks).
// PodStagger spreads one raid's landings.
const (
	PodFallTicks = 240
	PodStagger   = 60
	// PodRadius: centre and edge pods land within this many tiles of their
	// spot (original tuning).
	PodRadius = 6
	// RoofCrush is what a roof broken by a landing pod deals to whoever
	// stands beneath (wiki: Drop pod, pods deal no damage themselves but
	// go through roofs, and the falling roof hurts; the amount is original
	// tuning).
	RoofCrush = 15
)
