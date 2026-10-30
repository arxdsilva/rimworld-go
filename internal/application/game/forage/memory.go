package forage

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Memory numbers (original tuning): a spot an animal gave up on is
// skipped for ForgetTicks; it remembers at most maxMarks.
const (
	ForgetTicks = 2500
	maxMarks    = 4
)

// Mark is a spot an animal could not get to, skipped until Until.
type Mark struct {
	X, Y  int
	Until uint64
}

// Memory is the tame animals' feeding memory, saved with the wildlife
// state: the spots each gave up on (Skip) and who is starving and
// searching its whole area (Far). The zero value is empty.
type Memory struct {
	Skip map[entity.EntityID][]Mark
	Far  map[entity.EntityID]bool
}

// GaveUp notes that animal id could not get to its meal at (x, y).
func (m *Memory) GaveUp(id entity.EntityID, x, y int, now uint64) {
	if m.Skip == nil {
		m.Skip = map[entity.EntityID][]Mark{}
	}
	ms := live(m.Skip[id], now)
	if len(ms) >= maxMarks {
		ms = ms[1:]
	}
	m.Skip[id] = append(ms, Mark{X: x, Y: y, Until: now + ForgetTicks})
}

// Marks is animal id's live marks (read only).
func (m *Memory) Marks(id entity.EntityID) []Mark { return m.Skip[id] }

// Skips reports that marks hold (x, y) at tick now.
func Skips(marks []Mark, x, y int, now uint64) bool {
	for _, k := range marks {
		if k.X == x && k.Y == y && now < k.Until {
			return true
		}
	}
	return false
}

// Searching reports that animal id, with food, searches far
// (animal.FarSearch: from below StarveSearch until it eats or is above
// StarveSearchEnd), and keeps the answer.
func (m *Memory) Searching(id entity.EntityID, food int) bool {
	far := animal.FarSearch(food, m.Far[id])
	switch {
	case far && !m.Far[id]:
		if m.Far == nil {
			m.Far = map[entity.EntityID]bool{}
		}
		m.Far[id] = true
	case !far && m.Far[id]:
		delete(m.Far, id)
	}
	return far
}

// IsFar reports animal id searching far (read only).
func (m *Memory) IsFar(id entity.EntityID) bool { return m.Far[id] }

// Ate ends animal id's far search: it found food.
func (m *Memory) Ate(id entity.EntityID) { delete(m.Far, id) }

// Prune drops expired marks and every entry of an animal alive no longer
// reports (gone from the map). Deleting while ranging is order-free.
func (m *Memory) Prune(now uint64, alive func(entity.EntityID) bool) {
	for id, ms := range m.Skip {
		if ms = live(ms, now); len(ms) == 0 || !alive(id) {
			delete(m.Skip, id)
		} else {
			m.Skip[id] = ms
		}
	}
	for id := range m.Far {
		if !alive(id) {
			delete(m.Far, id)
		}
	}
}

// live is ms without its expired marks (in place).
func live(ms []Mark, now uint64) []Mark {
	out := ms[:0]
	for _, k := range ms {
		if now < k.Until {
			out = append(out, k)
		}
	}
	return out
}
