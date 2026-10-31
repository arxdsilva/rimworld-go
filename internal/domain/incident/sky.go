package incident

import (
	"strconv"

	"rimworld-go/internal/domain/sim/clock"
)

// Sky events (FOOD-6, wiki: Events, Eclipse, Toxic fallout). An eclipse
// takes the sunlight to nothing for 0.75 to 1.25 days: outdoor crops stop
// growing and solar panels give nothing. Toxic fallout lasts 2.5 to 10.5
// days: plants under the open sky wither and die, and pawns and animals
// under it gain toxic buildup (body/cond_toxic.go); a roof shelters
// them. The wiki gives fallout no interplay with rain or snow: the
// weather goes on as before.

// The sky events' lengths, hundredths of a day (wiki: Events).
const (
	EclipseMin = 75
	EclipseMax = 125
	FalloutMin = 250
	FalloutMax = 1050
)

// SkyTicks is how long sky event id lasts for roll r in [0, SkySpan(id)]:
// its shortest length plus r ticks (0 for any other event).
func SkyTicks(id ID, r int) uint64 {
	switch id {
	case Eclipse:
		return EclipseMin*clock.TicksPerDay/100 + uint64(max(r, 0))
	case ToxicFallout:
		return FalloutMin*clock.TicksPerDay/100 + uint64(max(r, 0))
	}
	return 0
}

// SkySpan is the spread (ticks) of sky event id's length.
func SkySpan(id ID) int {
	switch id {
	case Eclipse:
		return (EclipseMax - EclipseMin) * clock.TicksPerDay / 100
	case ToxicFallout:
		return (FalloutMax - FalloutMin) * clock.TicksPerDay / 100
	}
	return 0
}

// StartSky starts sky event id at tick now lasting ticks; false for an
// event that is not a sky event, or one already under way.
func (st *State) StartSky(id ID, now, ticks uint64) bool {
	switch {
	case id == Eclipse && !st.Eclipsed(now):
		st.EclipseUntil = now + ticks
	case id == ToxicFallout && !st.Fallout(now):
		st.FalloutUntil = now + ticks
	default:
		return false
	}
	return true
}

// Eclipsed reports an eclipse at tick now.
func (st State) Eclipsed(now uint64) bool { return now < st.EclipseUntil }

// Fallout reports toxic fallout at tick now.
func (st State) Fallout(now uint64) bool { return now < st.FalloutUntil }

// SkyView is the sky event under way at tick now for the views: "toxic
// fallout", "eclipse" or "" (fallout first when both are), and its end
// tick (0: none).
func (st State) SkyView(now uint64) (string, uint64) {
	switch {
	case st.Fallout(now):
		return "toxic fallout", st.FalloutUntil
	case st.Eclipsed(now):
		return "eclipse", st.EclipseUntil
	}
	return "", 0
}

// SkyLetter is the letter's title and text for sky event id lasting
// ticks.
func SkyLetter(id ID, ticks uint64) (title, text string) {
	days := float64(ticks) / clock.TicksPerDay
	switch id {
	case Eclipse:
		return "Eclipse", "The moon has slid across the sun. For about " + dayText(days) +
			" no sunlight reaches the ground: crops outdoors stop growing and solar panels give no power."
	case ToxicFallout:
		return "Toxic fallout", "A poisonous dust is settling out of the sky and will keep falling for about " + dayText(days) +
			". Plants under the open sky will wither and die, and anyone outdoors will build up toxins: keep colonists and animals under a roof."
	}
	return "", ""
}

// dayText is a length in days for a letter ("1 day", "4 days").
func dayText(d float64) string {
	n := int(d + 0.5)
	if n <= 1 {
		return "a day"
	}
	return strconv.Itoa(n) + " days"
}
