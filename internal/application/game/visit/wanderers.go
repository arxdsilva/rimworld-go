package visit

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/sim/clock"
)

// New wanderers after a game over (MAP-19). Wiki, Endings: the option opens
// eight in-game hours after everyone is gone, and after seven groups it is
// no longer available.
const (
	// WanderersWait is how long everyone must have been gone.
	WanderersWait = 8 * clock.TicksPerHour
	// WandererGroups is how many groups a game gets.
	WandererGroups = 7
)

// WanderersOpen is the option's state: in, the ticks until it opens (only
// meaningful while everyone is gone); left, the groups left; ok, one may come
// now. goneAt is the tick everyone went (0: not noted yet, so the wait is
// whole), groups the groups made.
func WanderersOpen(gone bool, goneAt, now uint64, groups int) (in, left int, ok bool) {
	left = max(WandererGroups-groups, 0)
	if !gone {
		return 0, left, false
	}
	if goneAt == 0 {
		return WanderersWait, left, false
	}
	if due := goneAt + WanderersWait; due > now {
		in = int(due - now)
	}
	return in, left, in == 0 && left > 0
}

// ParseSeeds reads a group's pawn seeds, comma-separated decimal, 1..limit
// and no two alike; why says what is wrong with name ("" fine).
func ParseSeeds(name string, limit int) (seeds []uint64, why string) {
	if name == "" {
		return nil, "nobody was chosen"
	}
	parts := strings.Split(name, ",")
	if len(parts) > limit {
		return nil, fmt.Sprintf("at most %d can set out together", limit)
	}
	for _, p := range parts {
		s, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nil, "that is nobody we know of"
		}
		if slices.Contains(seeds, s) {
			return nil, "the same person was chosen twice"
		}
		seeds = append(seeds, s)
	}
	return seeds, ""
}
