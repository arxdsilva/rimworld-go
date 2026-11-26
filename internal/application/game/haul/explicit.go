// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package haul

import (
	"fmt"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Leg names an exact physical stack, requested count and destination.
type Leg struct {
	Stack       entity.EntityID
	Count, X, Y int
}

// ParseLegs decodes a player's one-to-four leg command. Reject trailing
// input, invalid counts and overflow rather than accepting a prefix.
func ParseLegs(s string) ([]Leg, bool) {
	parts := strings.SplitN(s, ";", 5)
	if len(parts) > 4 {
		return nil, false
	}
	legs := make([]Leg, 0, len(parts))
	for _, p := range parts {
		left, dst, ok := strings.Cut(strings.TrimSpace(p), "@")
		sid, qty, sep := strings.Cut(left, ":")
		x, y, xy := strings.Cut(dst, ",")
		id, e1 := strconv.ParseUint(sid, 10, 64)
		n, e2 := strconv.Atoi(qty)
		dx, e3 := strconv.Atoi(x)
		dy, e4 := strconv.Atoi(y)
		if !ok || !sep || !xy || e1 != nil || e2 != nil || e3 != nil || e4 != nil || id == 0 || n <= 0 {
			return nil, false
		}
		legs = append(legs, Leg{entity.EntityID(id), n, dx, dy})
	}
	return legs, len(legs) > 0
}

func EncodeLegs(legs []Leg) string {
	parts := make([]string, len(legs))
	for i, l := range legs {
		parts[i] = fmt.Sprintf("%d:%d@%d,%d", l.Stack, l.Count, l.X, l.Y)
	}
	return strings.Join(parts, ";")
}

// ExplicitHost reads the same stores and reachability as native hauling.
type ExplicitHost interface {
	Stack(entity.EntityID) (item.Stack, bool)
	Reachable(entity.EntityID, int, int) bool
	Visible(int, int) bool
	Walkable(int, int) bool
	Items() item.Book
	MealRule() item.MealRule
}

type ExplicitClaims interface {
	FreeFor(entity.EntityID, entity.EntityID, int) int
	TileOwner(int, int) (entity.EntityID, bool)
}

// ExplicitPlan plans the first load, combining consecutive compatible legs
// only when preceding picks are whole stacks and the native load fits.
// Remaining legs stay in the existing player queue, in order.
func ExplicitPlan(h ExplicitHost, claims ExplicitClaims, id entity.EntityID, legs []Leg) (job.Record, int, string) {
	if len(legs) == 0 {
		return job.Record{}, 0, "invalid haul legs"
	}
	first := legs[0]
	st, ok := h.Stack(first.Stack)
	if !ok || st.Carrier != 0 || st.Qty <= 0 || !h.Visible(st.X, st.Y) {
		return job.Record{}, 0, "stack no longer available"
	}
	if !h.Visible(first.X, first.Y) || !h.Walkable(first.X, first.Y) {
		return job.Record{}, 0, "destination blocked"
	}
	if !h.Reachable(id, st.X, st.Y) || !h.Reachable(id, first.X, first.Y) {
		return job.Record{}, 0, "unreachable"
	}
	if owner, held := claims.TileOwner(first.X, first.Y); held && owner != id {
		return job.Record{}, 0, "destination reserved"
	}
	limit := min(item.CarryLimit, h.Items().StackLimit(st.Kind))
	if first.Count <= 0 || first.Count > st.Qty {
		return job.Record{}, 0, "requested quantity unavailable"
	}
	n := min(first.Count, limit)
	if n <= 0 || claims.FreeFor(id, first.Stack, st.Qty) < n {
		return job.Record{}, 0, "stack reserved"
	}
	rec := job.Record{Kind: job.Haul, Target: first.Stack, TX: st.X, TY: st.Y,
		DestX: first.X, DestY: first.Y, Count: n, Forced: true}
	// Zero used legs means this trip finishes only part of the first leg.
	if n < first.Count {
		return rec, 0, ""
	}
	i := 1
	for n == st.Qty && i < len(legs) && rec.Count < limit {
		l := legs[i]
		next, ok := h.Stack(l.Stack)
		if !ok || l.X != first.X || l.Y != first.Y || l.Stack == first.Stack ||
			next.Qty <= 0 || next.Carrier != 0 || !item.SameMeal(st, next, h.MealRule()) ||
			!h.Visible(next.X, next.Y) || !h.Reachable(id, next.X, next.Y) {
			break
		}
		duplicate := false
		for j := 0; j < len(rec.Pickups); j += 2 {
			duplicate = duplicate || rec.Pickups[j] == l.Stack
		}
		take := l.Count
		if duplicate || take <= 0 || take > next.Qty || take > limit-rec.Count || claims.FreeFor(id, l.Stack, next.Qty) < take {
			break
		}
		rec.Pickups = append(rec.Pickups, l.Stack, entity.EntityID(take))
		rec.Count += take
		i++
	}
	return rec, i, ""
}

// ExplicitFirstShare is the first stack's planned portion, independent of
// any goods merged into it after the order was accepted.
func ExplicitFirstShare(rec job.Record) int {
	n := rec.Count
	for i := 1; i < len(rec.Pickups); i += 2 {
		n -= int(rec.Pickups[i])
	}
	return max(0, n)
}
