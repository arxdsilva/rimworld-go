// Package mentalbreak is what a colonist does when its mood gives way:
// for a while it stops taking orders or doing work and acts out instead.
// Afterwards it feels the relief of catharsis.
package mentalbreak

// Kind is a mental break.
type Kind uint8

const (
	None       Kind = iota
	SadWander       // wanders aimlessly in a daze
	FoodBinge       // eats far past full
	HideInRoom      // shuts itself away in its room
	numKinds
)

// Def is a break's player-facing texts and how long it lasts.
type Def struct {
	Label    string // inspector/alert: "sad wander"
	Message  string // "%s is wandering around in a daze."
	Duration uint64 // ticks
}

const hour = 2500

var defs = [numKinds]Def{
	SadWander:  {"sad wander", "%s is wandering around in a daze.", 8 * hour},
	FoodBinge:  {"food binge", "%s is binge eating.", 4 * hour},
	HideInRoom: {"hiding in room", "%s has locked themselves away.", 10 * hour},
}

// DefOf returns the break definition.
func DefOf(k Kind) Def {
	if k >= numKinds {
		return Def{}
	}
	return defs[k]
}

func (k Kind) String() string { return DefOf(k).Label }

// Kinds lists the breaks a colonist can have.
func Kinds() []Kind { return []Kind{SadWander, FoodBinge, HideInRoom} }

// Threshold is the mood below which breaks become possible.
const Threshold = 20

// RiskPerHour is the chance (permille) of a break starting in an hour at
// mood m: none at or above Threshold, then 2% per point below it (about 4
// in 10 per hour for a colonist at rock bottom).
func RiskPerHour(m int) int {
	if m >= Threshold {
		return 0
	}
	return (Threshold - m) * 20
}
