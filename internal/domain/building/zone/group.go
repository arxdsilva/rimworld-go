package zone

// Storage groups (STO-2, wiki: Shelf: linked storage). Building zones of
// one kind (shelves with shelves, hoppers with hoppers, graves with
// graves) link into a group that shares one set of settings: the filter,
// special filters, priority and name of the zone they were linked to. A
// group is named by its first member's zone id; a group left with one
// member dissolves.

// shared copies the settings a storage group shares from src onto z.
func shared(z *Zone, src Zone) {
	z.Food, z.Wood, z.Stone, z.Corpses, z.Chunks = src.Food, src.Wood, src.Stone, src.Corpses, src.Chunks
	z.Custom, z.Filter, z.Special, z.Priority, z.Name = src.Custom, src.Filter, src.Special, src.Priority, src.Name
}

// share hands src's shared settings to every other member of its group.
func (m *Map) share(src Zone) {
	if src.Group == 0 {
		return
	}
	for _, id := range m.ids {
		if r := m.rec(id); id != src.ID && r.z.Group == src.Group {
			shared(&r.z, src)
		}
	}
}

// Members lists the zones of group g by id (none for 0).
func (m *Map) Members(g ID) []ID {
	if g == 0 {
		return nil
	}
	var out []ID
	for _, id := range m.ids {
		if m.rec(id).z.Group == g {
			out = append(out, id)
		}
	}
	return out
}

// Link puts building zone id into leader's storage group (starting one
// named after leader when it has none), taking the group's settings. Both
// must be building zones of one kind; a zone already in another group
// leaves it first. It reports whether anything changed.
func (m *Map) Link(id, leader ID) bool {
	m.ver++
	r, l := m.rec(id), m.rec(leader)
	if r == nil || l == nil || id == leader || !r.z.Kind.Building() || r.z.Kind != l.z.Kind {
		return false
	}
	if l.z.Group == 0 {
		l.z.Group = leader
	}
	if r.z.Group == l.z.Group {
		return false
	}
	m.Unlink(id)
	r.z.Group = l.z.Group
	shared(&r.z, l.z)
	return true
}

// Unlink takes zone id out of its storage group (keeping the settings it
// had); a group left with a single member dissolves. It reports whether
// id was in a group.
func (m *Map) Unlink(id ID) bool {
	m.ver++
	r := m.rec(id)
	if r == nil || r.z.Group == 0 {
		return false
	}
	g := r.z.Group
	r.z.Group = 0
	if rest := m.Members(g); len(rest) == 1 {
		m.rec(rest[0]).z.Group = 0
	}
	return true
}

// SameStore reports zone a is zone b or in b's storage group (a bill's
// store or source target names one zone of a group for the group).
func (m *Map) SameStore(a, b ID) bool {
	if a == b {
		return a != 0
	}
	za, b1 := m.Zone(a)
	zb, b2 := m.Zone(b)
	return b1 && b2 && za.Group != 0 && za.Group == zb.Group
}
