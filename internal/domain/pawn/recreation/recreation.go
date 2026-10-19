// Package recreation is the recreation (joy) need: it drains while a
// colonist is awake and fills while it does something it enjoys.
// Values are permille-style 0..Full like the other needs.
package recreation

// Full is a completely entertained colonist.
const Full = 10000

// Start is the level new colonists arrive with.
const Start = 7000

// DrainPeriod: one point lost per this many awake ticks (about 4300 a
// day, so a colonist wants some fun every day).
const DrainPeriod = 14

// Activity is something done for fun.
type Activity uint8

const (
	None Activity = iota
	Wander
	Chat
	Relax
	FireSide
	Horseshoes
)

// gain per GainPeriod ticks of each activity. A three-hour evening of
// horseshoes fills the need from empty; a stroll restores less than a day
// drains, so colonists need proper recreation, not just wandering about.
var gain = [...]int{None: 0, Wander: 4, Chat: 8, Relax: 6, FireSide: 6, Horseshoes: 13}

// GainPeriod is how many ticks one Gain step takes.
const GainPeriod = 10

// Gain is the recreation restored per GainPeriod ticks of activity a.
func Gain(a Activity) int {
	if int(a) >= len(gain) {
		return 0
	}
	return gain[a]
}

// Thresholds for mood thoughts.
const (
	Starved     = 1500 // "recreation-starved"
	Bored       = 3500 // "bored"
	Entertained = 8500 // "entertained"
)

// Clamp keeps v in 0..Full.
func Clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > Full {
		return Full
	}
	return v
}
