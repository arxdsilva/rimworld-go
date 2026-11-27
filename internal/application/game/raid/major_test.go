// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/incident"
)

// diceHost answers Roll from a script of results and records each bound
// asked; any other Host call panics (the nil embedded Host).
type diceHost struct {
	Host
	results []int
	asked   []int
}

func (h *diceHost) Roll(n int) int {
	h.asked = append(h.asked, n)
	r := h.results[0]
	h.results = h.results[1:]
	return r
}

// bigStory is the legacy raid face's story: its raid points, whether a
// ship part could come, and the people raids sent.
type bigStory struct {
	Story
	points int
	ship   bool
	ships  int
	raids  int
}

func (s *bigStory) RaidPoints(int) int { return s.points }
func (s *bigStory) ShipPart() bool     { s.ships++; return s.ship }
func (s *bigStory) ScaledRaid(int)     { s.raids++ }

// The legacy raid face splits by core's defs: a ship part that comes ends
// it; one that cannot, or mechanoids with none to send, is a people raid.
func TestBigRaidSplitsByCoreDefs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		ship         bool
		results      []int
		ships, raids int
		asked        []int
	}{
		{"ship part", true, []int{39}, 1, 0, []int{114}},
		{"no ship part", false, []int{39}, 1, 1, []int{114}},
		{"no mechanoids", true, []int{40, 0}, 0, 1, []int{114, 2}},
	}
	for _, c := range cases {
		h, s := &diceHost{results: c.results}, &bigStory{points: 300, ship: c.ship}
		(&Threats{h: h, s: s}).bigRaid(10, 0)
		if s.ships != c.ships || s.raids != c.raids || !slices.Equal(h.asked, c.asked) {
			t.Errorf("%s: ships %d raids %d asking %v", c.name, s.ships, s.raids, h.asked)
		}
	}
}

// Major splits a raid face by the incident defs it is given: the ship
// part's weight against the raid's, the mechanoids' point minimum and
// weight against each hostile human faction, the same draws as before the
// weights were data.
func TestMajorReadsDefs(t *testing.T) {
	t.Parallel()
	with := func(edit func(id incident.ID, d *incident.Def)) func(incident.ID) incident.Def {
		return func(id incident.ID) incident.Def {
			d := incident.DefOf(id)
			edit(id, &d)
			return d
		}
	}
	core := incident.DefOf
	noShip := with(func(id incident.ID, d *incident.Def) {
		if id == incident.CrashedShipPart {
			d.Weight = 0
		}
	})
	dearMechs := with(func(id incident.ID, d *incident.Def) {
		if id == incident.MechRaid {
			d.MinPoints, d.Weight = 500, 3
		}
	})
	cases := []struct {
		name           string
		points, humans int
		def            func(incident.ID) incident.Def
		results        []int
		want           Major
		asked          []int
	}{
		{"core ship part", 400, 1, core, []int{39}, MajorShipPart, []int{114}},
		{"core past the ship part", 200, 1, core, []int{40}, MajorRaid, []int{114}},
		{"core mechs", 300, 2, core, []int{40, 0}, MajorMechs, []int{114, 3}},
		{"core people", 300, 2, core, []int{113, 1}, MajorRaid, []int{114, 3}},
		{"legacy one faction", 300, 0, core, []int{50, 0}, MajorMechs, []int{114, 2}},
		{"ship weight 0", 300, 1, noShip, []int{0, 1}, MajorRaid, []int{74, 2}},
		{"mechs below their minimum", 499, 1, dearMechs, []int{60}, MajorRaid, []int{114}},
		{"mechs at their minimum and weight", 500, 2, dearMechs, []int{60, 2}, MajorMechs, []int{114, 5}},
		{"mechs past their weight", 500, 2, dearMechs, []int{60, 3}, MajorRaid, []int{114, 5}},
	}
	for _, c := range cases {
		h := &diceHost{results: c.results}
		th := &Threats{h: h}
		if got := th.Major(c.points, c.humans, c.def); got != c.want || !slices.Equal(h.asked, c.asked) {
			t.Errorf("%s: %v asking %v, want %v asking %v", c.name, got, h.asked, c.want, c.asked)
		}
	}
	// a book whose raid and ship part both weigh nothing still draws once
	none := with(func(id incident.ID, d *incident.Def) {
		if id == incident.CrashedShipPart || id == incident.Raid {
			d.Weight = 0
		}
	})
	h := &diceHost{results: []int{0, 1}}
	if got := (&Threats{h: h}).Major(100, 1, none); got != MajorRaid || !slices.Equal(h.asked, []int{1}) {
		t.Errorf("weightless: %v asking %v", got, h.asked)
	}
}
