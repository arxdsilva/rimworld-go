package mood

import "rimworld-go/internal/domain/sim/clock"

// Kind is a thought type.
type Kind uint8

const (
	SleptOutside Kind = iota
	SleptOnGround
	SleptInBed
	SleptInAwfulRoom
	SleptInNiceRoom
	AteMeal
	AteRaw
	AteAtTable
	AteWithoutTable
	Hungry
	Tired
	Cold
	Freezing
	Hot
	Sweltering
	numKinds
)

// Def is a thought's label, mood effect and duration (0 = state thought,
// recomputed every tick instead of remembered).
type Def struct {
	Label    string
	Value    int
	Duration uint64
	Group    uint8 // thoughts sharing a non-zero group replace each other
}

const (
	groupNone uint8 = iota
	groupSleepPlace
	groupSleepRoom
	groupMealKind
	groupMealTable
)

// day: remembered thoughts last one in-game day.
const day = uint64(clock.TicksPerDay)

var defs = [numKinds]Def{
	SleptOutside:     {"slept outside", -6, day, groupSleepPlace},
	SleptOnGround:    {"slept on the ground", -4, day, groupSleepPlace},
	SleptInBed:       {"slept in a bed", 2, day, groupSleepPlace},
	SleptInAwfulRoom: {"slept in an awful room", -4, day, groupSleepRoom},
	SleptInNiceRoom:  {"slept in a nice room", 5, day, groupSleepRoom},
	AteMeal:          {"ate a cooked meal", 4, day, groupMealKind},
	AteRaw:           {"ate raw food", -5, day, groupMealKind},
	AteAtTable:       {"ate at a table", 3, day, groupMealTable},
	AteWithoutTable:  {"ate without a table", -3, day, groupMealTable},
	Hungry:           {"hungry", -8, 0, groupNone},
	Tired:            {"tired", -5, 0, groupNone},
	Cold:             {"cold", -4, 0, groupNone},
	Freezing:         {"freezing", -10, 0, groupNone},
	Hot:              {"hot", -4, 0, groupNone},
	Sweltering:       {"sweltering", -10, 0, groupNone},
}

// DefOf returns the thought definition.
func DefOf(k Kind) Def {
	if k >= numKinds {
		return Def{}
	}
	return defs[k]
}

// Base is mood with no thoughts.
const Base = 50

// LowMood is the threshold below which work slows down.
const LowMood = 30

// Memory is a remembered thought that expires.
type Memory struct {
	Kind    Kind
	Expires uint64
}

// Memories is a pawn's remembered thoughts (one per kind, newest wins).
type Memories []Memory

// Add remembers k at tick now, replacing an existing one of the same kind
// and any other thought of the same group.
func (m Memories) Add(k Kind, now uint64) Memories {
	if g := DefOf(k).Group; g != groupNone {
		out := m[:0]
		for _, x := range m {
			if x.Kind == k || DefOf(x.Kind).Group != g {
				out = append(out, x)
			}
		}
		m = out
	}
	exp := now + DefOf(k).Duration
	for i := range m {
		if m[i].Kind == k {
			m[i].Expires = exp
			return m
		}
	}
	return append(m, Memory{Kind: k, Expires: exp})
}

// ForgetGroupOf drops memories in k's group (e.g. room quality after a
// night in an ordinary room).
func (m Memories) ForgetGroupOf(k Kind) Memories {
	g := DefOf(k).Group
	out := m[:0]
	for _, x := range m {
		if g == groupNone || DefOf(x.Kind).Group != g {
			out = append(out, x)
		}
	}
	return out
}

// Prune drops expired memories.
func (m Memories) Prune(now uint64) Memories {
	out := m[:0]
	for _, x := range m {
		if x.Expires > now {
			out = append(out, x)
		}
	}
	return out
}

// Thought is one active line with its effect.
type Thought struct {
	Kind  Kind
	Label string
	Value int
}

// Active lists memories plus current state thoughts.
func Active(m Memories, states ...Kind) []Thought {
	var out []Thought
	for _, x := range m {
		d := DefOf(x.Kind)
		out = append(out, Thought{x.Kind, d.Label, d.Value})
	}
	for _, k := range states {
		d := DefOf(k)
		out = append(out, Thought{k, d.Label, d.Value})
	}
	return out
}

// Level sums thoughts onto Base and clamps to 0..100.
func Level(ts []Thought) int {
	v := Base
	for _, t := range ts {
		v += t.Value
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// Label is a player-facing word for a mood level.
func Label(v int) string {
	switch {
	case v < 20:
		return "miserable"
	case v < LowMood:
		return "unhappy"
	case v < 55:
		return "content"
	case v < 75:
		return "happy"
	}
	return "joyful"
}
