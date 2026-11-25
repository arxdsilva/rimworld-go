// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package visit

import (
	"math"

	"rimworld-go/internal/domain/sim/entity"
)

// LegacyCaravan carries the explicit membership retained in older saves.
type LegacyCaravan struct {
	Trader  entity.EntityID
	Members []entity.EntityID
	Faction int
	LeaveAt uint64
}

// RebuildLegacy restores party identity without replacing a visitor's
// active job or making any random rolls. Ambiguous old visits stay separate.
func (v Visits) RebuildLegacy(caravans []LegacyCaravan, now uint64) []uint64 {
	ids := make([]uint64, len(caravans))
	for i, c := range caravans {
		ms := make([]Member, 0, len(c.Members))
		for _, id := range c.Members {
			role := RoleGuard
			if id == c.Trader {
				role = RoleTrader
			}
			ms = append(ms, Member{ID: id, Role: role})
		}
		ids[i] = v.NewGroup(KindCaravan, c.Faction, ms, now, c.LeaveAt)
	}
	rs := v.h.Records(false)
	if rs == nil {
		return ids
	}
	for i := range *rs {
		r := &(*rs)[i]
		if r.GroupID != 0 {
			continue
		}
		kind, role := KindVisitor, RoleLeader
		switch {
		case r.Ally:
			kind, role = KindAid, RoleFighter
		case r.LeaveAt == math.MaxUint64:
			kind = KindQuest
		}
		gid := v.NewGroup(kind, r.Faction, []Member{{ID: r.ID, Role: role}}, now, r.LeaveAt)
		if r.Leaving {
			for j := range *v.h.Groups(true) {
				g := &(*v.h.Groups(true))[j]
				if g.ID == gid {
					g.Phase = PhaseLeaving
				}
			}
		}
	}
	return ids
}
