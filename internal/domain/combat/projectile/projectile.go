package projectile

import "rimworld-go/internal/domain/sim/entity"

// Trace is one resolved attack.
type Trace struct {
	FromX, FromY int
	ToX, ToY     int
	Tick         uint64
	Hit          bool
	Melee        bool
	// Damage dealt when it hit; Building marks a blow on a structure.
	Damage   int
	Building bool
	// FromBuilding: a sentry turret fired it (PLAY-024); Attacker is then
	// the turret's building id.
	FromBuilding bool
	// Attacker and Target are the creatures involved (Target 0 for a
	// building or a trap); Weapon is the attacker's weapon name.
	Attacker, Target entity.EntityID
	Weapon           string
}

// Lifetime is how many ticks a trace stays visible.
const Lifetime = 18

// Prune drops traces older than Lifetime at now.
func Prune(ts []Trace, now uint64) []Trace {
	out := ts[:0]
	for _, t := range ts {
		if now-t.Tick < Lifetime {
			out = append(out, t)
		}
	}
	return out
}
