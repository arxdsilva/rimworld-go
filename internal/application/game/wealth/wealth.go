package wealth

import (
	"math"
	"slices"

	"rimworld-go/internal/domain/sim/clock"
)

// Source is a map's wealth in parts, the sweep's view of it: WealthOf
// sums parts from..to-1 (clamped to what there is) and reports how many
// parts there are now. The game implements it over its things in a fixed
// order; WealthOf(0, math.MaxInt) is one full count.
type Source interface {
	WealthOf(from, to int) (sum, parts int)
}

const (
	// Pass is how long one full sweep takes: one in-game hour.
	Pass = clock.TicksPerHour
	// Stride is how often the game calls Step: every Stride ticks, a
	// pass in Pass/Stride slices. A pass starts on the hour's first tick.
	Stride = 25
	// steps is the number of slices in a pass.
	steps = Pass / Stride
	// HistoryDays is how many daily samples are kept (the History graph).
	HistoryDays = 60
)

// Sample is the wealth counted at the absolute tick Tick.
type Sample struct {
	Tick  uint64
	Value int
}

// Save is a Tracker as saved: everything, the pass in progress included,
// so a game saved mid-sweep loads into the same state.
type Save struct {
	// History is the daily samples, oldest first (at most HistoryDays);
	// Next the tick from which the next is taken (0: on the first pass).
	History []Sample
	Next    uint64
	// Value is the last full pass's count, once Counted.
	Value   int
	Counted bool
	// Hour is 1 + the hour of the pass in progress (0: none); Cursor the
	// parts it has summed so far, into Partial.
	Hour            uint64
	Cursor, Partial int
}

// Tracker is one map's wealth: a sweep over the map's things, a slice
// each Step, one full pass an hour. Build it with New.
type Tracker struct {
	src Source
	s   Save
}

// New is a tracker that sweeps src.
func New(src Source) *Tracker { return &Tracker{src: src} }

// Value is the wealth: the last full pass's count, or a full count now
// while no pass has finished yet (a new game, an older save).
func (t *Tracker) Value() int {
	if t.s.Counted {
		return t.s.Value
	}
	sum, _ := t.src.WealthOf(0, math.MaxInt)
	return sum
}

// History is the daily samples, oldest first (read only).
func (t *Tracker) History() []Sample {
	if t == nil {
		return nil
	}
	return t.s.History
}

// Step sweeps the slice of the pass that tick now falls in: it sums the
// parts up to now's share of the pass and, in the hour's last slice,
// finishes it (the value, and the day's sample). A pass starts on an
// hour's first tick; until one has, Step does nothing (so a call at any
// other tick, as a load's, leaves the tracker as it was). Calling it twice
// at one tick does the work once, and a missed slice is caught up by the
// next call.
func (t *Tracker) Step(now uint64) {
	hour := now/Pass + 1
	if t.s.Hour != 0 && t.s.Hour != hour {
		t.finish(now) // a pass left from an earlier hour: count the rest now
	}
	slice := int(now % Pass / Stride)
	if t.s.Hour == 0 {
		if now%Pass != 0 {
			return
		}
		t.s.Hour = hour
	}
	_, n := t.src.WealthOf(0, 0)
	if to := n * (slice + 1) / steps; to > t.s.Cursor {
		sum, _ := t.src.WealthOf(t.s.Cursor, to)
		t.s.Partial, t.s.Cursor = t.s.Partial+sum, to
	}
	if slice == steps-1 {
		t.finish(now)
	}
}

// finish ends the pass in progress at tick now: the parts not yet summed
// are counted, the total becomes the value, and the day's first finished
// pass is its sample.
func (t *Tracker) finish(now uint64) {
	rest, _ := t.src.WealthOf(t.s.Cursor, math.MaxInt)
	t.s.Value, t.s.Counted = t.s.Partial+rest, true
	t.s.Hour, t.s.Cursor, t.s.Partial = 0, 0, 0
	if now < t.s.Next {
		return
	}
	t.s.History = append(t.s.History, Sample{Tick: now, Value: t.s.Value})
	if len(t.s.History) > HistoryDays {
		t.s.History = slices.Clone(t.s.History[len(t.s.History)-HistoryDays:])
	}
	t.s.Next = (now/clock.TicksPerDay + 1) * clock.TicksPerDay
}

// Export is the tracker as saved; nil while it holds nothing (no pass
// begun or finished).
func (t *Tracker) Export() *Save {
	if t == nil || len(t.s.History) == 0 && t.s.Next == 0 && !t.s.Counted && t.s.Hour == 0 {
		return nil
	}
	s := t.s
	s.History = slices.Clone(t.s.History)
	return &s
}

// Import restores a saved tracker (nil: empty).
func (t *Tracker) Import(s *Save) {
	t.s = Save{}
	if s != nil {
		t.s = *s
		t.s.History = slices.Clone(s.History)
	}
}
