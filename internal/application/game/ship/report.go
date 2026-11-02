package ship

import (
	"fmt"

	"rimworld-go/internal/domain/sim/clock"
)

// Wiki numbers (wiki: Ship, Ship reactor).
const (
	// EnginesNeeded is the engines a ship needs to leave.
	EnginesNeeded = 3
	// CountdownTicks is the reactor's startup: 15 days.
	CountdownTicks = 15 * clock.TicksPerDay
)

// Issues is ship sh's launch report at now: what it still lacks, in the
// report's order (none: ready for launch).
func (s *State) Issues(sh Ship, now uint64) []string {
	var out []string
	need := func(have int, what string) {
		if have == 0 {
			out = append(out, "Needs "+what)
		}
	}
	if sh.Engines < EnginesNeeded {
		n := EnginesNeeded - sh.Engines
		out = append(out, fmt.Sprintf("Needs %d more %s", n, plural(n, "engine")))
	}
	need(len(sh.Reactors), "a ship reactor")
	need(sh.Cores, "a ship computer core")
	need(sh.Sensors, "a ship sensor cluster")
	need(sh.Beams, "a ship structural beam")
	need(len(sh.Caskets), "a ship stasis casket joined to a beam")
	if len(sh.Reactors) > 0 {
		switch left := s.left(sh, now); {
		case left < 0:
			out = append(out, "The reactor has not been started")
		case left > 0:
			out = append(out, "The reactor is still waking: "+Duration(left)+" left")
		}
	}
	if len(sh.Caskets) > 0 && s.aboard(sh) == 0 {
		out = append(out, "Nobody sleeps in a ship casket")
	}
	return out
}

// left is ship sh's countdown: the ticks left on its best started
// reactor (0 awake, -1 none started).
func (s *State) left(sh Ship, now uint64) int {
	best := -1
	for _, id := range sh.Reactors {
		r := s.reactor(id)
		if r == nil {
			continue
		}
		if l := r.left(now); best < 0 || l < best {
			best = l
		}
	}
	return best
}

// Left is the countdown ticks left at now for a reactor started at start.
func Left(start, now uint64) int {
	end := start + CountdownTicks
	if now >= end {
		return 0
	}
	return int(end - now)
}

// aboard counts ship sh's occupied caskets.
func (s *State) aboard(sh Ship) int {
	n := 0
	for _, c := range sh.Caskets {
		if s.Sleeper(c) != 0 {
			n++
		}
	}
	return n
}

// Duration is ticks as the panes say it: "4 days 6 hours", "5 hours".
func Duration(ticks int) string {
	d, h := ticks/clock.TicksPerDay, (ticks%clock.TicksPerDay+clock.TicksPerHour-1)/clock.TicksPerHour
	if h == 24 {
		d, h = d+1, 0
	}
	switch {
	case d > 0 && h > 0:
		return fmt.Sprintf("%d %s %d %s", d, plural(d, "day"), h, plural(h, "hour"))
	case d > 0:
		return fmt.Sprintf("%d %s", d, plural(d, "day"))
	}
	return fmt.Sprintf("%d %s", max(h, 1), plural(max(h, 1), "hour"))
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
