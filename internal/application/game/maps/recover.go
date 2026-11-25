// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// RecoverySource supplies authoritative map-local goods and living owned
// crew eligibility, without giving the fleet access to simulation stores.
type RecoverySource interface {
	Source(entity.EntityID) (item.Stack, bool)
	CrewEligible(id, holder entity.EntityID) bool
	Debit(entity.EntityID, item.Stack, int)
	RecoverFreight() RecoveryFreight
}

type RecoveryFreight interface {
	RecoverCargo(uint64, item.Stack, int, []travel.Beast, func() bool) (int32, bool)
}

// RecoverCargo admits a requested native source, checks its actual caravan
// and route, and commits through the capacity/debit owner before pace refresh.
func RecoverCargo(f Fleet, h RecoverySource, c command.Command) {
	if !c.MapSet || c.Map != f.h.Current() || c.Zone == 0 || c.Target == 0 || c.Count < 0 {
		return
	}
	sid, cid := entity.EntityID(c.Target), entity.EntityID(c.Zone)
	st, ok := h.Source(sid)
	if !ok || uint64(st.Carrier) != c.Pawn || c.Count > st.Qty {
		return
	}
	cv := f.RecoveryAt(cid, c.Map)
	if cv == nil || !slices.ContainsFunc(cv.C.Members, func(m uint64) bool {
		return h.CrewEligible(entity.EntityID(m), cid)
	}) {
		return
	}
	roster := f.h.MarketCarriers(cv)
	members := CarrierCount(cv.C.Members, f.h.Alive)
	part := st
	if c.Count > 0 {
		part.Qty = c.Count
	}
	part.HaulMarked = false
	_, accepted := h.RecoverFreight().RecoverCargo(c.Zone, part, members, roster.Stashed, func() bool {
		h.Debit(sid, st, part.Qty)
		return true
	})
	if accepted {
		RefreshPace(f, cid)
	}
}

// MemberCaravanID captures identity before a member mutation may change
// or relocate the fleet's backing slice.
func MemberCaravanID(f Fleet, id entity.EntityID) entity.EntityID {
	if c := f.Of(id); c != nil {
		return entity.EntityID(c.C.ID)
	}
	return 0
}

// RemoveDeadMember updates the fleet's roster and derived movement after
// the simulation has completed the death and removed its world stash.
func RemoveDeadMember(f Fleet, cid, id entity.EntityID) {
	if c := f.At(cid); c != nil {
		c.C.Members = slices.DeleteFunc(c.C.Members, func(m uint64) bool { return m == uint64(id) })
		RefreshPace(f, cid)
	}
}
