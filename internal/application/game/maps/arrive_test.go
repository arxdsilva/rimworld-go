// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"fmt"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// noClaim is a Freight with no mineral site anywhere.
type noClaim struct{ Freight }

func (noClaim) Claim(int) (item.Kind, int, bool) { return 0, 0, false }

// arriveHost is a Host for Fleet.Arrived away from any map, site or
// mineral lump: the settlement's faction on every tile is fa.
type arriveHost struct {
	Host
	fa      *faction.Faction
	letters []string
}

func (h *arriveHost) ColonyTile() int              { return 0 }
func (h *arriveHost) CaravanArrived(uint64, int)   {}
func (h *arriveHost) SiteAt(int) (uint32, bool)    { return 0, false }
func (h *arriveHost) Cargo() Freight               { return noClaim{} }
func (h *arriveHost) Faction(int) *faction.Faction { return h.fa }
func (h *arriveHost) TileName(t int) string        { return fmt.Sprint("tile ", t) }
func (h *arriveHost) Letter(_ bool, title string, _ entity.EntityID, _ string, _ ...any) {
	h.letters = append(h.letters, title)
}

// TestArrivedVisitsOnlyWelcoming: a caravan reaching a settlement of a
// faction that welcomes the colony (faction.Welcomes) visits it until
// now plus visitTicks; a hostile, defeated or non-human faction's leaves
// it waiting for orders.
func TestArrivedVisitsOnlyWelcoming(t *testing.T) {
	t.Parallel()
	const now = 5000
	cases := []struct {
		name  string
		fa    *faction.Faction
		visit bool
	}{
		{"neutral", &faction.Faction{Type: faction.Settlers, Relation: faction.Neutral}, true},
		{"friendly", &faction.Faction{Type: faction.Clans, Relation: faction.Friendly}, true},
		{"hostile", &faction.Faction{Type: faction.Settlers, Relation: faction.Hostile}, false},
		{"defeated", &faction.Faction{Type: faction.Settlers, Relation: faction.Neutral, Defeated: true}, false},
		{"swarm", &faction.Faction{Type: faction.Swarm, Relation: faction.Neutral}, false},
		{"no settlement", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := &arriveHost{fa: c.fa}
			list := []Caravan{{C: caravan.Caravan{ID: 50, Tile: 9, Members: []uint64{7}, State: caravan.Travelling}}}
			Of(&list, &Atlas{}, h).Arrived(&list[0], now)
			got := list[0].C
			if visiting := got.State == caravan.Visiting; visiting != c.visit {
				t.Fatalf("visiting %v, want %v (state %v)", visiting, c.visit, got.State)
			}
			if c.visit && (got.VisitUntil != now+visitTicks || got.FoodDays != FoodDays) {
				t.Fatalf("visit until %d (want %d), food %d", got.VisitUntil, now+visitTicks, got.FoodDays)
			}
			if !c.visit && got.VisitUntil != 0 {
				t.Fatalf("a refused visit set VisitUntil %d", got.VisitUntil)
			}
			if len(h.letters) != 1 {
				t.Fatalf("letters %q", h.letters)
			}
		})
	}
}
