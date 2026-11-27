// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"testing"
)

// TestIDsAppendOnly pins every event's number by its key. IDs are saved and
// the threat events' order lays out the threat die, so a new event goes
// after the last one and no number ever moves.
func TestIDsAppendOnly(t *testing.T) {
	t.Parallel()
	keys := []string{
		"herd_crossing", "stray_joins", "farm_animals", "taken_a_liking", "timbergnaw_gnawing",
		"mad_animal", "visitor_group", "wanderer_joins", "party", "solar_flare",
		"short_circuit", "flu", "plague", "malaria", "sleeping_sickness",
		"gut_worms", "muscle_parasites", "trader_caravan", "orbital_trader", "crashed_ship_part",
		"crop_blight", "eclipse", "toxic_fallout", "heat_wave", "cold_snap",
		"volcanic_winter", "refugee_chased", "rival_raid", "traveler_group", "transport_pod_crash",
		"hive_swarm", "predator_pack", "manhunter_pack", "raid", "mech_raid",
		"ore_meteor", "supply_drop", "sky_song",
	}
	if NumIDs != 38 || len(keys) != int(NumIDs) {
		t.Fatalf("NumIDs = %d, pinned keys %d; want 38", NumIDs, len(keys))
	}
	for i, k := range keys {
		if d := DefOf(ID(i)); d.Key != k || d.ID != ID(i) {
			t.Errorf("ID %d: key %q id %d, want key %q", i, d.Key, d.ID, k)
		}
	}
	named := map[ID]int{
		TransportPodCrash: 29, HiveSwarm: 30, PredatorPack: 31, ManhunterPack: 32, Raid: 33, MechRaid: 34,
		OreMeteor: 35, SupplyDrop: 36, SkySong: 37,
	}
	for id, n := range named {
		if int(id) != n {
			t.Errorf("%s = %d, want %d", DefOf(id).Key, id, n)
		}
	}
}

// TestSeasonAllows: the contract's season limits.
func TestSeasonAllows(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		s              Season
		winter, summer bool
	}{{AnySeason, true, true}, {WinterOnly, true, false}, {NotWinter, false, true}} {
		if c.s.Allows(true) != c.winter || c.s.Allows(false) != c.summer {
			t.Errorf("season %d: winter %v, not winter %v; want %v, %v",
				c.s, c.s.Allows(true), c.s.Allows(false), c.winter, c.summer)
		}
	}
}

// TestSpend: PointsPct 0 (or below) spends every point, otherwise its
// percent, rounded down.
func TestSpend(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ pct, points, want int }{
		{0, 500, 500}, {-5, 500, 500}, {100, 500, 500}, {50, 501, 250}, {150, 200, 300},
	} {
		if got := (Def{PointsPct: c.pct}).Spend(c.points); got != c.want {
			t.Errorf("PointsPct %d of %d = %d, want %d", c.pct, c.points, got, c.want)
		}
	}
}

// TestCooled: a def never fired is cooled; one fired is not until its
// cooldown days have passed, and Allowed follows it.
func TestCooled(t *testing.T) {
	t.Parallel()
	d := DefOf(MadAnimal)
	var st State
	now := uint64(30 * clock.TicksPerDay)
	if !d.Cooled(st, now) {
		t.Fatal("never fired: not cooled")
	}
	st.Fired(MadAnimal, now)
	ready := now + uint64(d.Cooldown)*clock.TicksPerDay
	if d.Cooled(st, ready-1) || d.Allowed(st, ready-1, biome.TemperateForest) {
		t.Fatal("cooled before its cooldown ran out")
	}
	if !d.Cooled(st, ready) || !d.Allowed(st, ready, biome.TemperateForest) {
		t.Fatal("not cooled once its cooldown ran out")
	}
}
