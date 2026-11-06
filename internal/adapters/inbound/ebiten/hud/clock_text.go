package hud

import (
	"fmt"
	"time"

	"rimworld-go/internal/domain/sim/clock"
)

// ClockText is a time of day as "08:05", or "8:05 AM" with the 12-hour
// option (OPT-008).
func ClockText(hour, minute int, h12 bool) string {
	if !h12 {
		return fmt.Sprintf("%02d:%02d", hour, minute)
	}
	suffix := "AM"
	if hour >= 12 {
		suffix = "PM"
	}
	h := hour % 12
	if h == 0 {
		h = 12
	}
	return fmt.Sprintf("%d:%02d %s", h, minute, suffix)
}

// WallText is the player's wall clock, in the same format (OPT-008). The zero
// time (no clock handed in) reads "".
func WallText(t time.Time, h12 bool) string {
	if t.IsZero() {
		return ""
	}
	return ClockText(t.Hour(), t.Minute(), h12)
}

// hourLine is the in-game time, the wall clock beside it when asked for, the
// period and the activity.
func hourLine(d clock.Date, wall time.Time, realtime, h12 bool, activity string) string {
	t := ClockText(d.Hour, d.Minute, h12)
	if w := WallText(wall, h12); realtime && w != "" {
		t += " (" + w + ")"
	}
	return fmt.Sprintf("%s  %s · %s", t, d.Period(), activity)
}
