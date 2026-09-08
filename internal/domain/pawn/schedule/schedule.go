package schedule

import "rimworld-go/internal/domain/sim/clock"

// Activity is one hour-block on a pawn's daily schedule.
type Activity int

const (
	Anything Activity = iota
	Work
	Recreation
	Sleep
)

func (a Activity) String() string {
	switch a {
	case Work:
		return "work"
	case Recreation:
		return "recreation"
	case Sleep:
		return "sleep"
	default:
		return "anything"
	}
}

// Grid is a 24-hour activity plan. Index 0 is hour 0.
type Grid [clock.HoursPerDay]Activity

// DefaultColony is a starting plan: sleep at night, a short anything
// window at dawn, work through the day, recreation in the evening.
func DefaultColony() Grid {
	var g Grid
	for h := 0; h < clock.HoursPerDay; h++ {
		switch clock.PeriodAt(h) {
		case clock.Night:
			g[h] = Sleep
		case clock.Evening:
			g[h] = Recreation
		default:
			g[h] = Work
		}
	}
	g[6] = Anything
	g[7] = Anything
	return g
}

// At returns the activity for a 0-based hour. Out-of-range hours are night/sleep.
func (g Grid) At(hour int) Activity {
	if hour < 0 || hour >= len(g) {
		return Sleep
	}
	return g[hour]
}
