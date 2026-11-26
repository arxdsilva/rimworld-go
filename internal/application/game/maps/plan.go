// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Plan is how long a caravan of members, carrying load, would take from
// the current map to world tile dest, resting at night, and why it cannot
// ("" when it can): "No world map.", "No way to get there.", "Too slow to
// get there." or "Pick a destination for the caravan." (#14; the Form
// caravan dialog's estimate). Animals still on the map count for no mount,
// so the estimate errs slow.
func Plan(f Fleet, members []entity.EntityID, dest int, load int32, now uint64) (ticks int, why string) {
	p, home := f.h.Planet(), f.tileOf(f.h.Current())
	if p == nil || home < 0 {
		return 0, "No world map."
	}
	if dest < 0 || dest >= p.TileCount() || dest == home {
		return 0, "Pick a destination for the caravan."
	}
	route := p.Route(home, dest)
	if len(route) < 2 {
		return 0, "No way to get there."
	}
	c := caravan.Caravan{Slowest: slowest(f.h, members), Load: min(max(load, 0), 1000)}
	return travelTicks(f, route, &c, now)
}
