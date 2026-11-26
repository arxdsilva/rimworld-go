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
	"testing"
)

func TestCaravanTravelFactsUseNativePaceAndSchedule(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		state                  caravan.State
		hour                   int
		demand, empty          bool
		moving, resting, known bool
	}{
		{"day", caravan.Travelling, 6, false, false, true, false, true},
		{"night", caravan.Travelling, 22, false, false, false, true, true},
		{"return", caravan.Returning, 21, false, false, true, false, true},
		{"held", caravan.Returning, 3, true, false, false, false, true},
		{"arrived", caravan.Arrived, 3, false, true, false, false, true},
		{"visiting", caravan.Visiting, 12, false, true, false, false, true},
		{"forming", caravan.Forming, 12, false, false, false, false, false},
		{"empty-route", caravan.Returning, 12, false, true, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := maps.Caravan{C: caravan.Caravan{ID: 9, Members: []uint64{17}, State: tc.state, FoodDays: 1250, Slowest: 700, Load: 400, Mount: 1300, Path: []int{3}}}
			if tc.demand {
				c.Ask.Letter = 4
			}
			if tc.empty {
				c.C.Path = nil
			}
			v := BuildWorldMap(1, uint64(tc.hour*clock.TicksPerHour), []maps.Caravan{c},
				func(id entity.EntityID) travel.CaravanMemberFacts {
					return travel.CaravanMemberFacts{ID: id, Name: "Ada", Health: "bleeding"}
				},
				func(uint64) []item.Stack { return nil }, func(*maps.Caravan) (MarketView, bool) { return MarketView{}, false }, nil).Caravans[0]
			if v.Moving != tc.moving || v.Resting != tc.resting || v.WaitingEncounter != tc.demand || v.FoodKnown != tc.known || v.TravelState != tc.state {
				t.Fatalf("incorrect travel facts %+v", v)
			}
			if tc.known && (v.FoodDays != 1250 || v.SpeedPermil != c.C.Speed() || v.TilesPerDay != caravan.TilesPerDay(c.C.Speed())) {
				t.Fatalf("native speed or food lost: %+v", v)
			}
			if !tc.known && (v.SpeedPermil != 0 || v.TilesPerDay != 0) {
				t.Fatal("forming caravan invented a speed")
			}
			if len(v.MemberFacts) != 1 || v.MemberFacts[0].ID != 17 || v.Members[0] != "Ada" || v.Health[0] != "bleeding" {
				t.Fatalf("member projection %+v", v)
			}
		})
	}
}
