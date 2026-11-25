// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package tabview

import (
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// BuildWorldMap projects the authoritative caravans into a detached read
// model. The game supplies its name, health, cargo and market views.
func BuildWorldMap(tile int, now uint64, caravans []maps.Caravan,
	member func(entity.EntityID) travel.CaravanMemberFacts,
	cargo func(uint64) []item.Stack,
	market func(*maps.Caravan) (MarketView, bool),
	split func(*maps.Caravan, []item.Stack) travel.SplitView) WorldMap {
	v := WorldMap{Tile: tile}
	for i := range caravans {
		c := &caravans[i]
		cv := travel.CaravanView{
			ID: c.C.ID, Tile: c.C.Tile, Dest: c.C.Dest,
			Progress: c.C.Progress / 10, State: c.C.State.Label(),
			TravelState: c.C.State, FoodDays: c.C.FoodDays,
			FoodKnown:        c.C.State != caravan.Forming && c.C.State != caravan.Home,
			WaitingEncounter: c.Ask.Letter != 0,
		}
		if cv.FoodKnown {
			cv.SpeedPermil = c.C.Speed()
			cv.TilesPerDay = caravan.TilesPerDay(cv.SpeedPermil)
		}
		if !cv.WaitingEncounter && len(c.C.Path) > 0 &&
			(c.C.State == caravan.Travelling || c.C.State == caravan.Returning) {
			cv.Moving = caravan.Moving(int(now % clock.TicksPerDay / clock.TicksPerHour))
			cv.Resting = !cv.Moving
		}
		if c.Back {
			cv.Dest = c.Home
		}
		for _, raw := range c.C.Members {
			id := entity.EntityID(raw)
			m := member(id)
			cv.Members = append(cv.Members, m.Name)
			cv.Health = append(cv.Health, m.Health)
			cv.MemberFacts = append(cv.MemberFacts, m)
		}
		cv.Cargo = cargo(c.C.ID)
		if split != nil {
			cv.Split = split(c, cv.Cargo)
		}
		v.Caravans = append(v.Caravans, cv)
		if m, ok := market(c); ok {
			v.Markets = append(v.Markets, m)
		}
	}
	return v
}
