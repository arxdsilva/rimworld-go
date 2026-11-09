package wealth

import "slices"

// Counter counts the colony's whole wealth on the current map in one full
// pass (the game's Wealth).
type Counter interface{ Wealth() int }

// Sample is the wealth counted at the absolute tick Tick.
type Sample struct {
	Tick  uint64
	Value int
}

// Save is a Tracker as saved: its samples, oldest first, and the tick of
// the next one (0: not scheduled).
type Save struct {
	History []Sample
	Next    uint64
}

// Tracker is one map's wealth. Build it with New; the last count is
// derived (never saved): a loaded game counts again on its first Step.
type Tracker struct {
	c       Counter
	value   int
	counted bool
	s       Save
}

// New is a tracker that counts through c.
func New(c Counter) *Tracker { return &Tracker{c: c} }

// Value is the wealth: the last Step's count, or a full pass when no Step
// has counted yet.
func (t *Tracker) Value() int {
	if t.counted {
		return t.value
	}
	return t.c.Wealth()
}

// History is the samples taken, oldest first (read only; EAI-0: none yet).
func (t *Tracker) History() []Sample {
	if t == nil {
		return nil
	}
	return t.s.History
}

// Step counts the wealth again at tick now (one full pass). EAI-0: it takes
// no sample and nothing in the tick calls it yet.
func (t *Tracker) Step(now uint64) {
	_ = now // the sampling schedule (Save.Next) lands with the wealth work
	t.value, t.counted = t.c.Wealth(), true
}

// Export is the tracker as saved; nil while it holds nothing to keep.
func (t *Tracker) Export() *Save {
	if t == nil || len(t.s.History) == 0 && t.s.Next == 0 {
		return nil
	}
	return &Save{History: slices.Clone(t.s.History), Next: t.s.Next}
}

// Import restores a saved tracker (nil: empty); the count is redone by
// the next Step or Value.
func (t *Tracker) Import(s *Save) {
	t.s, t.value, t.counted = Save{}, 0, false
	if s != nil {
		t.s = Save{History: slices.Clone(s.History), Next: s.Next}
	}
}
