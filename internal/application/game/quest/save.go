package quest

import (
	"cmp"
	"slices"

	qd "rimworld-go/internal/domain/quest"
)

// Save is the root's saved state (QST-141): gob-friendly, field names
// stable. The heap, the indexes and the relation snapshot are rebuilt.
type Save struct {
	// Quests are every quest, ids ascending.
	Quests []qd.Quest
	// Next is the last id issued (ids are never reused).
	Next    qd.ID
	Cadence Cadence
	// NoGoodwill are the faction indexes whose goodwill is declined as a
	// reward (QST-083), ascending.
	NoGoodwill []int
}

// Export is the saved state; nil when nothing has ever happened (no
// quest, no window scheduled, no preference), so older and idle saves
// hash unchanged. The copy shares nothing with the root.
func (q *Quests) Export() *Save {
	if len(q.list) == 0 && q.next == 0 && !q.cad.Started && len(q.noGoodwill) == 0 {
		return nil
	}
	s := &Save{Next: q.next, Cadence: q.cad, NoGoodwill: slices.Clone(q.noGoodwill)}
	s.Quests = make([]qd.Quest, len(q.list))
	for i, n := range q.list {
		s.Quests[i] = cloneQuest(n)
	}
	return s
}

// Import replaces the root's state with s (nil: empty) and rebuilds the
// heap and the indexes from the quests. The next Hourly re-takes the
// relation snapshot and checks every live quest against it.
func (q *Quests) Import(s *Save) {
	q.list, q.heap, q.rel, q.noGoodwill = nil, nil, nil, nil
	q.due, q.idx = map[qd.ID]entry{}, newIndex()
	q.next, q.cad = 0, Cadence{}
	q.ver++
	if s == nil {
		return
	}
	q.list = make([]qd.Quest, 0, len(s.Quests))
	for _, n := range s.Quests {
		if n.ID != 0 {
			q.list = append(q.list, cloneQuest(n))
		}
	}
	slices.SortStableFunc(q.list, func(a, b qd.Quest) int { return cmp.Compare(a.ID, b.ID) })
	q.list = slices.CompactFunc(q.list, func(a, b qd.Quest) bool { return a.ID == b.ID })
	q.next = s.Next
	if n := len(q.list); n > 0 && q.list[n-1].ID > q.next {
		q.next = q.list[n-1].ID
	}
	q.noGoodwill = slices.Clone(s.NoGoodwill)
	slices.Sort(q.noGoodwill)
	q.noGoodwill = slices.Compact(q.noGoodwill)
	for i := range q.list {
		q.idx.set(q.list[i])
		q.schedule(q.list[i])
	}
	q.cad = s.Cadence
	if q.cad.Started {
		q.heap.push(entry{Tick: q.cad.At})
	}
}

// cloneQuest is a deep copy of n: parts, their lines and rewards.
func cloneQuest(n qd.Quest) qd.Quest {
	if n.Parts != nil {
		n.Parts = slices.Clone(n.Parts)
		for i := range n.Parts {
			n.Parts[i].Lines = slices.Clone(n.Parts[i].Lines)
		}
	}
	if n.Rewards != nil {
		n.Rewards = slices.Clone(n.Rewards)
		for i := range n.Rewards {
			n.Rewards[i].Lines = slices.Clone(n.Rewards[i].Lines)
		}
	}
	return n
}
