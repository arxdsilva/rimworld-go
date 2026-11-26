// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/sim/entity"
)

// Views is every caravan on the World tab, in fleet order; status is a
// member's short health note ("" when well; WS-M).
func Views(f Fleet, status func(entity.EntityID) string) []travel.CaravanView {
	var out []travel.CaravanView
	for i := range *f.list {
		c := &(*f.list)[i]
		cv := travel.CaravanView{ID: c.C.ID, Tile: c.C.Tile, Dest: c.C.Dest, Progress: c.C.Progress / 10, State: c.C.State.Label()}
		if c.Back {
			cv.Dest = c.Home
		}
		cv.FoodDays = foodDays(f, c) / 100 // tenths of a day
		for _, m := range c.C.Members {
			cv.Members = append(cv.Members, f.h.Name(entity.EntityID(m)))
			cv.Health = append(cv.Health, status(entity.EntityID(m)))
			cv.Prisoner = append(cv.Prisoner, f.h.Captive(entity.EntityID(m)))
		}
		out = append(out, cv)
	}
	return out
}
