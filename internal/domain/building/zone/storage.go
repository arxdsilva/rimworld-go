package zone

// Stores lists a map's storage zones by priority (STO-1), so a haul
// search visits only the storage zones of the priority it is trying, not
// every zone. Each priority's zones are in id order with their bounding
// boxes; a zone's cells stay the map's own row-major list (EachCell). The
// lists are rebuilt whenever the map's Ver moves (every edit, filters and
// priorities included) or the map was replaced (a load). The zero value
// is ready to use; it is never saved.
type Stores struct {
	m    *Map
	ver  uint64
	ok   bool
	prio [len(Priorities)][]Store
}

// Store is one storage zone in Stores: its settings and bounding box.
type Store struct {
	Zone
	Box Rect
}

// Of lists the storage zones of priority p on m, in id order (none for
// an invalid priority). The list is shared: callers only read it, and
// only until m changes.
func (s *Stores) Of(m *Map, p Priority) []Store {
	if !p.Valid() || m == nil {
		return nil
	}
	if !s.ok || s.m != m || s.ver != m.ver {
		s.build(m)
	}
	return s.prio[PriorityCritical-p]
}

func (s *Stores) build(m *Map) {
	s.m, s.ver, s.ok = m, m.ver, true
	for i := range s.prio {
		clear(s.prio[i])
		s.prio[i] = s.prio[i][:0]
	}
	for _, id := range m.ids {
		r := m.rec(id)
		if !r.z.Kind.Storage() || !r.z.Priority.Valid() {
			continue
		}
		box, _ := m.Bounds(id)
		i := PriorityCritical - r.z.Priority
		s.prio[i] = append(s.prio[i], Store{Zone: r.z, Box: box})
	}
}

// Dist is the Chebyshev distance from (x, y) to the nearest tile of r (0
// inside it): no tile of a zone inside r is nearer.
func (r Rect) Dist(x, y int) int {
	dx := max(r.MinX-x, x-r.MaxX, 0)
	dy := max(r.MinY-y, y-r.MaxY, 0)
	return max(dx, dy)
}
