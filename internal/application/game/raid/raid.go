// Package raid runs the colony's threats: the legacy storyteller's die,
// raid plans, the raider AI (arrive, assault, siege camp, flee), predator
// packs, hive swarms and sappers' rock damage (DEF-4; moved from the game's
// raids.go, raid_plans.go and hive.go).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. What it
// needs from the simulation comes through the consumer-owned roles in
// deps.go, which the game implements with small adapter types in
// raid_host.go. The game builds one *Raid and keeps one field.
//
// The raider records and raid groups stay on the game (prisoners,
// visitors, kidnappers and factions write them too), behind the Roster
// role; their types live here. Spawning a raid (people, factions,
// relatives) stays on the game.
//
// Raid is deterministic: every roll comes from the host (the game's main
// stream, as before the extraction), time only from the host's tick, and
// it iterates no maps.
package raid

import (
	"rimworld-go/internal/domain/economy/item"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Raid is the raid cluster's root: the game builds one with New and keeps
// one field. Its components are exported so the game reaches each by name
// (g.raid.Raiders.Think); a feature's entry points go on its component,
// never here.
type Raid struct {
	// Raiders is the raider AI (raiders.go).
	Raiders *Raiders
	// Threats is the legacy storyteller's die and raid plans (threats.go).
	Threats *Threats
	// Hives are hive swarms and the rock face they break out of (hive.go).
	Hives *Hives
	// Rock is the rock sappers are hacking through (rock.go).
	Rock *Rock
	// Pods are hostile drop pods falling and landing (pods.go, DEF-4).
	Pods *Pods
	// Mechs are the mechanoids (mech.go, DEF-4); Ships the crashed ship
	// parts they guard (shippart.go).
	Mechs *Mechs
	Ships *ShipParts
	// Kidnaps are raiders carrying off downed colonists (kidnap.go).
	Kidnaps *Kidnaps
}

// New builds the raid cluster on host h with the extra roles r.
func New(h Host, r Roles) *Raid {
	hv := &Hives{h: h, w: r.Wild, n: r.Nest, m: r.Mech}
	kid := &Kidnaps{h: h, k: r.Kidnap}
	rs := &Raiders{h: h, m: r.Mischief, sg: r.Siege, ch: r.Chronicle, t: r.Tactics, a: r.Arrive, kid: kid}
	ms, ships := newMechs(h, r) // mech.go
	pods := &Pods{h: h, a: r.Arrive, ch: r.Chronicle, mechs: ms}
	ms.hives, ms.drops = hv, pods // nest.go: saved with the mechanoids; their drop pods
	return &Raid{Raiders: rs, Threats: &Threats{h: h, w: r.Wild, s: r.Story, hives: hv, mechs: ms}, Hives: hv,
		Rock: &Rock{h: h, damage: make(map[[2]int]int)}, Pods: pods, Mechs: ms, Ships: ships, Kidnaps: kid}
}

// Mode is what a raider is doing.
type Mode uint8

const (
	Arriving   Mode = iota // walking in towards the colony
	Assaulting             // fighting colonists
	Fleeing                // running for the map edge
)

// Raider is one raider's record (the game's saved Raiders map). Prisoners
// and visitors are on it too.
type Raider struct {
	Group  int
	Mode   Mode
	Downed uint64 // tick it went down (0 = standing)
	Idle   uint64 // tick it last had no one to fight
	Dead   uint64 // tick it died (removed a little later)
	// RetryAt: no way in was found; don't plan again before this tick.
	RetryAt uint64
	// Arrived is when the raider came onto the map: after RaiderGiveUp
	// it gives up and goes home.
	Arrived uint64
	// The colony's prisoners (prisoners.go, M8) stay on this list, out of
	// the raid AI. Leaving: released, walking off the map (since Idle).
	// Hungry: the food band (prison.FoodLow..prison.FoodFed) wants a
	// meal brought, not before FoodRetry. Warden: who last visited.
	Leaving   bool
	Hungry    bool
	FoodRetry uint64
	Warden    entity.EntityID
	// KidnapAt: a kidnap failed; no new one before this tick (kidnap.go).
	KidnapAt uint64
	// DEF-4 (absent in older saves): Loot and LootQty are goods a thief
	// carries off (dropped where it falls); Hurt the injury it last had (a
	// besieger taking damage may set off the storm); Deeds the fires a
	// burner set or the thefts a thief made.
	Loot    item.Kind
	LootQty int
	Hurt    int
	Deeds   int
	// CBT-4 (absent in older saves): CoverAt is the tick before which a
	// raider under fire looks for cover again, walking to (CoverX,
	// CoverY) when it found some (tactics.go); Escaping marks a prisoner
	// breaking out (game/prison/escape.go): it fights its way out, then
	// flees.
	CoverAt        uint64
	CoverX, CoverY int
	Escaping       bool
}

// Group is one raid (the game's saved RaidGroups).
type Group struct {
	Size int
	Plan Plan
	// Siege camp: where it digs in, whether it has, when it started out,
	// when it storms (Until) and whether it has.
	CampX, CampY    int
	Camped, Stormed bool
	Started, Until  uint64
	// Faction is 1 + the index of the faction that sent it (0: an
	// unattributed raid, as every legacy one); Over: it was defeated or
	// left and the faction's goodwill paid for it (factions.go, NG-1).
	Faction int
	Over    bool
	// DEF-4 (absent in older saves): Arrival is how it came; FleeLoss the
	// share lost (percent) at which it flees (0: the old half); Fled that
	// its retreat was logged. A staged raid uses the camp fields as its
	// staging spot (CampX, CampY), Started as when it came, Until as its
	// time limit and Stormed as the assault begun. Pods are its drop pods
	// (PodFaction holds Faction until the first lands, so the faction's
	// raid is not judged over before anyone is down); Mortars the mortars
	// a siege built.
	Arrival    raiddef.Arrival
	FleeLoss   int
	Fled       bool
	Pods       []Pod
	PodFaction int
	Mortars    []entity.EntityID
	// Supply (CBT-4, absent in older saves) is when a camped siege's next
	// resupply pod lands (tactics.go).
	Supply uint64
}

// Plan is how a raid comes at the colony: the domain's strategy (DEF-4;
// the first three keep the saved byte of the old plan). Most raids
// assault head on; later ones may lay siege (dig in behind sandbags out of
// reach, bombard with mortars, then storm in), stage near the edge before
// attacking, come to steal or to burn, or, against a colony shut behind
// walls and rock, send sappers who tunnel straight through and step
// around traps.
type Plan = raiddef.Strategy

const (
	Assault = raiddef.ImmediateAssault
	Siege   = raiddef.Siege
	Sappers = raiddef.Sappers
	Stage   = raiddef.StageThenAssault
	Steal   = raiddef.Steal
	Burn    = raiddef.Burn
)

const (
	// raiderLeaveIdle: raiders with no one left to fight for this long go home.
	raiderLeaveIdle = clock.TicksPerHour
	// SightRange: colonists this close pull arriving raiders into the fight.
	SightRange = 18
	// RaiderRetry: a raider that found no way in waits this long to re-plan.
	RaiderRetry = 120
	// RaiderGiveUp: raiders still on the map after this long go home.
	RaiderGiveUp = clock.TicksPerDay
	// SiegeHoldMax is the longest a siege camp holds before it storms the
	// colony: it storms after 1.5 to 3 days (wiki: Raid, raiddef
	// SiegeAssaultMin and SiegeAssaultRolls), sooner when hurt.
	SiegeHoldMax = (raiddef.SiegeAssaultMin + raiddef.SiegeAssaultRolls - 1) * clock.TicksPerHour
	// siegeSetup: raiders still not at the camp after this long dig in anyway.
	siegeSetup = 2 * clock.TicksPerHour
	// siegeRange: the camp's distance from the colony, just past the
	// mortars' minimum range (wiki: Mortar, 29.9) so they can reach it
	// (original tuning).
	siegeRange = 32
)
