// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

import (
	"maps"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"slices"
	"testing"
)

// Whole-game raid tests (raiders, sieges, sappers, hive swarms, saves)
// stay in internal/application/game: raids_test.go, raid_plans_test.go,
// hive_test.go.

func TestThreatOdds(t *testing.T) {
	t.Parallel()
	count := func(winter bool, day int) map[Threat]int {
		out := map[Threat]int{}
		for roll := 0; roll < ThreatRolls; roll++ {
			out[PickThreat(roll, day, winter)]++
		}
		return out
	}
	// SIM-025 turned one raid face into a mad animal from day 6, in every
	// season (Fire falls back to a raid when none can go mad).
	if c := count(false, 5); c[ThreatRaid] != ThreatRolls {
		t.Fatalf("before day 6 only raids: %v", c)
	}
	if c := count(false, 7); c[ThreatMad] != 1 || c[ThreatRaid] != ThreatRolls-1 {
		t.Fatalf("day 7: raids and a mad animal: %v", c)
	}
	if c := count(false, 10); c[ThreatManhunters] != 2 || c[ThreatHive] != 0 || c[ThreatMad] != 1 || c[ThreatRaid] != 5 {
		t.Fatalf("day 10: %v", c)
	}
	if c := count(false, 12); c[ThreatManhunters] != 2 || c[ThreatHive] != 1 || c[ThreatMad] != 1 || c[ThreatRaid] != 4 {
		t.Fatalf("day 12: %v", c)
	}
	if c := count(true, 12); c[ThreatPack] != 4 || c[ThreatHive] != 1 || c[ThreatMad] != 1 || c[ThreatRaid] != 2 {
		t.Fatalf("winter: %v", c)
	}
}

func TestPlanNames(t *testing.T) {
	t.Parallel()
	for p, want := range map[Plan]string{Assault: "assault", Siege: "siege", Sappers: "sappers", 9: "assault"} {
		if p.String() != want {
			t.Errorf("plan %d: %q, want %q", p, p, want)
		}
	}
}

// rockHost is the ground Rock needs; any other host call panics.
type rockHost struct {
	Host
	rock  map[[2]int]bool
	mined int
}

func (h *rockHost) Rock(x, y int) bool { return h.rock[[2]int{x, y}] }
func (h *rockHost) Mine(x, y int)      { h.mined++; delete(h.rock, [2]int{x, y}) }

func TestRockOpensAtItsHitPoints(t *testing.T) {
	t.Parallel()
	h := &rockHost{rock: map[[2]int]bool{{3, 4}: true}}
	r := New(h, Roles{}).Rock
	r.Dig(5, 5, RockHP) // open ground: nothing to dig
	r.Dig(3, 4, RockHP-1)
	if h.mined != 0 || r.Export()[[2]int{3, 4}] != RockHP-1 {
		t.Fatalf("one short of its hit points the rock holds: %v", r.Export())
	}
	saved := r.Export()
	back := New(h, Roles{}).Rock
	back.Import(nil) // an older save keeps the empty map
	back.Import(saved)
	back.Dig(3, 4, 1)
	if h.mined != 1 || len(back.Export()) != 0 {
		t.Fatalf("the restored rock opens on the last blow: mined %d, left %v", h.mined, back.Export())
	}
}

// scriptDice answers Roll from a script and records each range asked; a
// Roll(n <= 0) or one past the script fails the test (any other Host call
// panics: the nil embedded Host).
type scriptDice struct {
	Host
	t      *testing.T
	script []int
	asked  []int
}

func (d *scriptDice) Roll(n int) int {
	d.t.Helper()
	if n <= 0 || len(d.asked) >= len(d.script) {
		d.t.Fatalf("Roll(%d) after %v (script %v)", n, d.asked, d.script)
	}
	d.asked = append(d.asked, n)
	return d.script[len(d.asked)-1]
}

// TestMajorReadsDefs: Major splits a raid face by the incident defs it is
// given, with today's draws for the core defs: Roll(74+40) for the ship
// part, then Roll(humans+1) for the mechanoids from 300 points.
func TestMajorReadsDefs(t *testing.T) {
	t.Parallel()
	patched := func(f func(id incident.ID, d *incident.Def)) func(incident.ID) incident.Def {
		return func(id incident.ID) incident.Def {
			d := incident.DefOf(id)
			f(id, &d)
			return d
		}
	}
	noShip := patched(func(id incident.ID, d *incident.Def) {
		if id == incident.CrashedShipPart {
			d.Weight = 0
		}
	})
	noWeights := patched(func(id incident.ID, d *incident.Def) {
		if id == incident.CrashedShipPart || id == incident.Raid {
			d.Weight = 0
		}
	})
	ship, mech := incident.DefOf(incident.CrashedShipPart), incident.DefOf(incident.MechRaid)
	cases := []struct {
		name           string
		def            func(incident.ID) incident.Def
		points, humans int
		script, asked  []int
		want           Major
	}{
		{"ship part", incident.DefOf, 1000, 1, []int{39}, []int{114}, MajorShipPart},
		{"ship part below its points", incident.DefOf, ship.MinPoints - 1, 1, []int{0}, []int{114}, MajorRaid},
		{"ship part at its points", incident.DefOf, ship.MinPoints, 1, []int{0}, []int{114}, MajorShipPart},
		{"mechs", incident.DefOf, mech.MinPoints, 2, []int{40, 0}, []int{114, 3}, MajorMechs},
		{"people against two factions", incident.DefOf, mech.MinPoints, 2, []int{40, 1}, []int{114, 3}, MajorRaid},
		{"no faction weighs one", incident.DefOf, 1000, 0, []int{113, 0}, []int{114, 2}, MajorMechs},
		{"mechs below their points", incident.DefOf, mech.MinPoints - 1, 1, []int{40}, []int{114}, MajorRaid},
		{"no weights, no draw", noWeights, 1000, 1, nil, nil, MajorRaid},
	}
	for _, c := range cases {
		d := &scriptDice{t: t, script: c.script}
		if got := (&Threats{h: d}).Major(c.points, c.humans, c.def); got != c.want || !slices.Equal(d.asked, c.asked) {
			t.Fatalf("%s: %v asking %v, want %v asking %v", c.name, got, d.asked, c.want, c.asked)
		}
	}
	for r := range 74 { // a patched-out ship part never comes, whatever the roll
		d := &scriptDice{t: t, script: []int{r, 1}}
		if got := (&Threats{h: d}).Major(1000, 1, noShip); got == MajorShipPart || d.asked[0] != 74 {
			t.Fatalf("roll %d with no ship part weight: %v asking %v", r, got, d.asked)
		}
	}
}

// TestThreatDefsMatchDie is the oracle between the legacy die (PickThreat)
// and the incident defs' threat pick: in every band each face has the
// same share of ThreatRolls.
func TestThreatDefsMatchDie(t *testing.T) {
	t.Parallel()
	face := map[Threat]incident.ID{ThreatRaid: incident.Raid, ThreatPack: incident.PredatorPack,
		ThreatManhunters: incident.ManhunterPack, ThreatHive: incident.HiveSwarm, ThreatMad: incident.MadAnimal}
	if ThreatRolls != incident.ThreatRolls {
		t.Fatalf("the die has %d faces, the defs %d", ThreatRolls, incident.ThreatRolls)
	}
	for _, winter := range []bool{false, true} {
		for _, day := range []int{0, 5, 6, 7, 8, 11, 12, 30} {
			die, defs := map[incident.ID]int{}, map[incident.ID]int{}
			for roll := range ThreatRolls {
				die[face[PickThreat(roll, day, winter)]]++
			}
			now := uint64(day) * clock.TicksPerDay
			o := incident.Odds{Disease: incident.FullFreq, Climate: incident.FullFreq, Winter: winter}
			n := incident.TotalOdds(incident.Threat, incident.State{}, now, biome.TemperateForest, o, nil)
			for r := range n {
				if id, ok := incident.PickOdds(incident.Threat, incident.State{}, now, biome.TemperateForest, o, r, nil); ok {
					defs[id]++
				}
			}
			if n != ThreatRolls || !maps.Equal(die, defs) {
				t.Fatalf("winter %v day %d: die %v, defs %v of %d", winter, day, die, defs, n)
			}
		}
	}
}
