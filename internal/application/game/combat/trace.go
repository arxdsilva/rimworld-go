package combat

import (
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/sim/entity"
)

// TraceView is one recent attack for presentation (the game's
// WorldView.Traces; moved from the game, DEF-3 package budget). Age is
// ticks since it happened (0..projectile.Lifetime).
type TraceView struct {
	FromX, FromY int
	ToX, ToY     int
	Age          int
	Hit          bool
	Melee        bool
	Damage       int  // dealt on a hit
	Building     bool // a blow on a door or wall
	// FromBuilding: a sentry turret fired it (PLAY-024); Attacker is then
	// the turret's building id.
	FromBuilding bool
	// Attacker and Target are the creatures involved (0 for a building
	// target); Weapon is the attacker's weapon name.
	Attacker, Target entity.EntityID
	Weapon           string
	Stage            projectile.Stage // DEF-1: Whole (melee, turret), or a shot's Fired and Landed
	// Dodged (CBT-3): a melee blow the defender dodged (Hit false).
	Dodged bool
}

// TraceViews is the traces younger than projectile.Lifetime at now, oldest
// first (a fresh slice: the world view keeps it).
func TraceViews(traces []projectile.Trace, now uint64) []TraceView {
	out := make([]TraceView, 0, len(traces))
	for _, t := range traces {
		if now-t.Tick >= projectile.Lifetime {
			continue
		}
		out = append(out, TraceView{FromX: t.FromX, FromY: t.FromY, ToX: t.ToX, ToY: t.ToY,
			Age: int(now - t.Tick), Hit: t.Hit, Melee: t.Melee, Damage: t.Damage, Building: t.Building,
			FromBuilding: t.FromBuilding, Attacker: t.Attacker, Target: t.Target, Weapon: t.Weapon, Stage: t.Stage, Dodged: t.Dodged})
	}
	return out
}
