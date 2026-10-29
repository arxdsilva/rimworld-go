package projectile

import "rimworld-go/internal/domain/sim/entity"

// Stage is what moment of an attack a trace records.
type Stage uint8

const (
	// Whole: a blow and its impact at once (melee, a sentry turret's shot,
	// a trap).
	Whole Stage = iota
	// Fired: a projectile left the muzzle. Hit says it was aimed at the
	// target, To is where it is headed, Damage is 0.
	Fired
	// Landed: a projectile arrived. To is where it struck; Hit means it
	// struck a creature or (Building) a wall, door, cover, turret or rock;
	// Damage is what got through armor (Hit with Damage 0 on a creature:
	// armor stopped it).
	Landed
)

// Trace is one attack moment for presentation.
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
	// Stage is the moment it records (Whole for melee and instant shots).
	Stage Stage
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
