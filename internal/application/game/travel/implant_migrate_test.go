// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
)

func TestTravelMigratesNestedLegacyBodyParts(t *testing.T) {
	old := func() Beast {
		fitted := make([]item.Kind, int(body.LegL)+1)
		fitted[body.LegL] = item.PegLeg
		return Beast{ID: 9, State: animal.State{Body: &body.Body{Missing: 1 << body.LegL, Fitted: fitted}}}
	}
	s := Import(&Save{
		Holds:   []PodHold{{Pod: 1, Hold: Hold{Animals: []Beast{old()}}}},
		Cargo:   []CaravanCargo{{Caravan: 2, Hold: Hold{Animals: []Beast{old()}}}},
		Flights: []Flight{{ID: 3, Load: Hold{Animals: []Beast{old()}}}},
		Roped:   []RopedBeast{{Hauler: 4, Beast: old()}},
	})
	for name, b := range map[string]*body.Body{
		"pod":    s.holds[1].Animals[0].State.Body,
		"cargo":  s.cargo[2].Animals[0].State.Body,
		"flight": s.flights[0].Load.Animals[0].State.Body,
		"roped":  s.roped[4].State.Body,
	} {
		if b == nil || b.ReplacementAt(body.LegL) != item.PegLeg || len(b.Fitted) > 0 {
			t.Errorf("%s lost the legacy replacement: %+v", name, b)
		}
	}
}
