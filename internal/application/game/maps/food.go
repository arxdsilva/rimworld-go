// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Prisoners who board and food that is eaten (#14). Package-level funcs on
// a Fleet: the Fleet type is at its method ceiling.

// maxCaptives is how many captives board looks at (original tuning, far
// above any real prison): a hostile save cannot make it loop long.
const maxCaptives = 64

// listed reports id is in a caravan, as a member or as a captive waiting
// to board.
func listed(f Fleet, id entity.EntityID) bool {
	for i := range *f.list {
		c := &(*f.list)[i]
		if slices.Contains(c.C.Members, uint64(id)) || slices.Contains(c.Captives, id) {
			return true
		}
	}
	return false
}

// pickCaptives is the prisoners named in list (comma-separated ids) that can
// leave: standing, not breaking, in no caravan and not one of colonists,
// ascending.
func pickCaptives(f Fleet, list string, colonists []entity.EntityID) []entity.EntityID {
	return travel.ParseIDs(list, func(id entity.EntityID) bool {
		return f.h.Captive(id) && f.h.Standing(id) && !f.h.Breaking(id) && !listed(f, id) && !slices.Contains(colonists, id)
	})
}

// board takes caravan c's captives off the map with it as it sets out:
// those still captive and standing and who can be stashed join its members,
// the rest stay home. Captives is emptied.
func board(f Fleet, c *Caravan) {
	ids := slices.Clone(c.Captives)
	c.Captives = nil
	if len(ids) > maxCaptives {
		ids = ids[:maxCaptives]
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	cid := entity.EntityID(c.C.ID)
	for _, id := range ids {
		if slices.Contains(c.C.Members, uint64(id)) || !f.h.Captive(id) || !f.h.Standing(id) || !f.h.Stash(id, cid) {
			continue
		}
		c.C.Members = append(c.C.Members, uint64(id))
	}
}

// eat feeds c's members for an hour off its cargo, once the reserve is
// spent: whole units, spoiling first, the surplus kept in Fed. False when
// the cargo cannot cover the hour (what is left is eaten and they starve).
func eat(f Fleet, c *Caravan) bool {
	cargo := f.h.Cargo()
	need := caravan.HourNeed(len(c.C.Members)) - int(c.Fed)
	units, fed, ok := caravan.Ration(cargo.Goods(c.C.ID), need)
	for _, s := range units {
		cargo.Yield(c.C.ID, s.Kind, s.Qty)
	}
	c.Fed = int32(fed)
	return ok
}

// foodDays is the days c's people can eat, in milli-days: the reserve plus
// what its cargo feeds them.
func foodDays(f Fleet, c *Caravan) int {
	return int(c.C.FoodDays) + int(caravan.Days(caravan.Nutrition(f.h.Cargo().Goods(c.C.ID))+int(c.Fed), len(c.C.Members)))
}

// slowest is the slowest Moving capacity among members still walking (the
// downed are carried), at most 1000 and at least 1.
func slowest[T ~uint64](h Host, members []T) int32 {
	s := int32(1000)
	for _, m := range members {
		if v := h.Capacity(entity.EntityID(m), body.Moving); v > 0 {
			s = min(s, v)
		}
	}
	return max(s, 1)
}
