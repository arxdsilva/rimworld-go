package quest

import qd "rimworld-go/internal/domain/quest"

// entry is one due timer: quest ID's event at Tick (Part for EvPart). ID 0
// is the cadence's next offer.
type entry struct {
	Tick uint64
	ID   qd.ID
	Ev   qd.Event
	Part int
}

// before orders entries by tick, then id, event and part, so equal ticks
// pop in one fixed order.
func (a entry) before(b entry) bool {
	if a.Tick != b.Tick {
		return a.Tick < b.Tick
	}
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	if a.Ev != b.Ev {
		return a.Ev < b.Ev
	}
	return a.Part < b.Part
}

// dueHeap is a binary min-heap of entries, by hand so a push boxes
// nothing. Stale entries stay until popped; the root checks each against
// its current timer (Quests.due).
type dueHeap []entry

func (h *dueHeap) push(e entry) {
	*h = append(*h, e)
	s := *h
	i := len(s) - 1
	for i > 0 {
		p := (i - 1) / 2
		if !s[i].before(s[p]) {
			break
		}
		s[i], s[p] = s[p], s[i]
		i = p
	}
}

// top is the earliest entry (ok false when empty).
func (h dueHeap) top() (entry, bool) {
	if len(h) == 0 {
		return entry{}, false
	}
	return h[0], true
}

func (h *dueHeap) pop() entry {
	s := *h
	e := s[0]
	n := len(s) - 1
	s[0] = s[n]
	s = s[:n]
	i := 0
	for {
		l, m := 2*i+1, i
		if l < n && s[l].before(s[m]) {
			m = l
		}
		if r := l + 1; r < n && s[r].before(s[m]) {
			m = r
		}
		if m == i {
			break
		}
		s[i], s[m] = s[m], s[i]
		i = m
	}
	*h = s
	return e
}
