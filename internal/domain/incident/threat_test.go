// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

func TestSpendAndSeason(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ pct, in, want int }{{0, 500, 500}, {100, 500, 500}, {50, 500, 250}, {-5, 500, 500}} {
		if got := (Def{PointsPct: c.pct}).Spend(c.in); got != c.want {
			t.Fatalf("pct %d: %d, want %d", c.pct, got, c.want)
		}
	}
	for _, c := range []struct {
		s      Season
		winter bool
		want   bool
	}{{AnySeason, true, true}, {AnySeason, false, true}, {WinterOnly, true, true}, {WinterOnly, false, false}, {NotWinter, true, false}, {NotWinter, false, true}} {
		if got := c.s.Allows(c.winter); got != c.want {
			t.Fatalf("%v winter %v: %v", c.s, c.winter, got)
		}
	}
}

func TestPointBudgetAndSeasonGate(t *testing.T) {
	t.Parallel()
	list := []Def{
		{ID: 0, Category: Threat, Weight: 3, MinPoints: 300},
		{ID: 1, Category: Threat, Weight: 2, Season: WinterOnly},
		{ID: 2, Category: Threat, Weight: 5, Season: NotWinter},
	}
	tot := func(o Odds) int { return totalOdds(list, Threat, State{}, 0, biome.TemperateForest, o, nil) }
	for _, c := range []struct {
		o    Odds
		want int
	}{
		{Odds{Points: 299}, 5}, {Odds{Points: 300}, 8},
		{Odds{Points: 299, Winter: true}, 2}, {Odds{Points: 300, Winter: true}, 5},
	} {
		if got := tot(c.o); got != c.want {
			t.Fatalf("%+v: total %d, want %d", c.o, got, c.want)
		}
	}
	id, ok := pickOdds(list, Threat, State{}, 0, biome.TemperateForest, Odds{Points: 300}, 2, nil)
	if !ok || id != 0 {
		t.Fatalf("roll 2: %v %v", id, ok)
	}
	id, _ = pickOdds(list, Threat, State{}, 0, biome.TemperateForest, Odds{Points: 299}, 0, nil)
	if id != 2 {
		t.Fatalf("below the minimum the first eligible is the not-winter one, got %v", id)
	}
	if _, ok := pickOdds(list, Threat, State{}, 0, biome.TemperateForest, Odds{}, 5, nil); ok {
		t.Fatal("roll past the total picks nothing")
	}
}

func TestThreatCatalogue(t *testing.T) {
	t.Parallel()
	if RivalRaid != 27 || PredatorPack != 28 || ManhunterPack != 29 || HiveSwarm != 30 || Raid != 31 || MechRaid != 32 || NumIDs != 33 {
		t.Fatalf("ids %d %d %d %d %d %d", RivalRaid, PredatorPack, ManhunterPack, HiveSwarm, Raid, MechRaid)
	}
	keys := map[ID]string{PredatorPack: "predator_pack", ManhunterPack: "manhunter_pack", HiveSwarm: "hive_swarm", Raid: "raid", MechRaid: "mech_raid"}
	for id, k := range keys {
		d := DefOf(id)
		if d.Key != k || d.Category != Threat || d.Cooldown != 0 || d.ID != id {
			t.Fatalf("%v: %+v", id, d)
		}
	}
	type f struct{ w, min, cd, day int }
	got := func(id ID) f { d := DefOf(id); return f{d.Weight, d.MinPoints, d.Cooldown, d.MinDay} }
	if d := DefOf(CrashedShipPart); got(CrashedShipPart) != (f{40, 400, 15, 10}) || !d.UnderRaid {
		t.Fatalf("ship part %+v", d)
	}
	if d := DefOf(MechRaid); got(MechRaid) != (f{1, 300, 0, 0}) || !d.UnderRaid {
		t.Fatalf("mech %+v", d)
	}
	if d := DefOf(Raid); d.Fill != 8 || d.Weight != 74 || d.UnderRaid || ThreatRolls != 8 {
		t.Fatalf("raid %+v", d)
	}
	if got(MadAnimal) != (f{1, 0, 10, 6}) {
		t.Fatalf("mad animal %+v", got(MadAnimal))
	}
	if DefOf(PredatorPack).Season != WinterOnly || DefOf(ManhunterPack).Season != NotWinter || DefOf(ManhunterPack).MinDay != 8 || DefOf(HiveSwarm).MinDay != 12 {
		t.Fatal("face season or first day")
	}
}

func threatCounts(winter bool, day int, st State, ok func(ID) bool) (map[ID]int, int) {
	now := uint64(day) * clock.TicksPerDay
	o := Odds{Disease: FullFreq, Climate: FullFreq, Winter: winter}
	n := TotalOdds(Threat, st, now, biome.TemperateForest, o, ok)
	m := map[ID]int{}
	for r := range n {
		if id, picked := PickOdds(Threat, st, now, biome.TemperateForest, o, r, ok); picked {
			m[id]++
		}
	}
	return m, n
}

func TestThreatSharesMatchDie(t *testing.T) {
	t.Parallel()
	type m = map[ID]int
	cases := []struct {
		winter bool
		days   []int
		want   m
	}{
		{false, []int{0, 5}, m{Raid: 8}},
		{false, []int{6, 7}, m{MadAnimal: 1, Raid: 7}},
		{false, []int{8, 11}, m{ManhunterPack: 2, MadAnimal: 1, Raid: 5}},
		{false, []int{12, 30}, m{ManhunterPack: 2, HiveSwarm: 1, MadAnimal: 1, Raid: 4}},
		{true, []int{0, 5}, m{PredatorPack: 4, Raid: 4}},
		{true, []int{6, 7, 8, 11}, m{PredatorPack: 4, MadAnimal: 1, Raid: 3}},
		{true, []int{12, 30}, m{PredatorPack: 4, HiveSwarm: 1, MadAnimal: 1, Raid: 2}},
	}
	for _, c := range cases {
		for _, day := range c.days {
			got, n := threatCounts(c.winter, day, State{}, nil)
			if n != ThreatRolls || len(got) != len(c.want) {
				t.Fatalf("winter %v day %d: total %d counts %v", c.winter, day, n, got)
			}
			for id, w := range c.want {
				if got[id] != w {
					t.Fatalf("winter %v day %d: %v got %d want %d (%v)", c.winter, day, id, got[id], w, got)
				}
			}
		}
	}
}

func TestThreatFiller(t *testing.T) {
	t.Parallel()
	day := 20
	var st State
	st.Fired(MadAnimal, uint64(day-1)*clock.TicksPerDay)
	got, n := threatCounts(false, day, st, nil)
	if n != 8 || got[MadAnimal] != 0 || got[Raid] != 5 {
		t.Fatalf("mad on cooldown: total %d %v", n, got)
	}
	got, n = threatCounts(false, day, State{}, func(id ID) bool { return id != Raid })
	if n != 4 || got[Raid] != 0 {
		t.Fatalf("raid refused: total %d %v", n, got)
	}
	got, n = threatCounts(true, day, State{}, func(id ID) bool { return id != PredatorPack })
	if n != 4 || got[PredatorPack] != 0 || got[Raid] != 2 {
		t.Fatalf("pack refused: total %d %v", n, got)
	}
	over := []Def{
		{ID: 0, Category: Threat, Weight: 6},
		{ID: 1, Category: Threat, Weight: 5},
		{ID: 2, Category: Threat, Fill: 8},
	}
	if n := totalOdds(over, Threat, State{}, 0, biome.TemperateForest, Odds{}, nil); n != 11 {
		t.Fatalf("filler 0 when the others exceed Fill: total %d", n)
	}
	two := append(over[:2:2], Def{ID: 2, Category: Threat, Fill: 20}, Def{ID: 3, Category: Threat, Fill: 20})
	if n := totalOdds(two, Threat, State{}, 0, biome.TemperateForest, Odds{}, nil); n != 11+9+9 {
		t.Fatalf("each filler gets the remainder: total %d", n)
	}
}

func TestUnderRaidNeverDrawn(t *testing.T) {
	t.Parallel()
	now := uint64(40 * clock.TicksPerDay)
	o := Odds{Disease: FullFreq, Climate: FullFreq, Points: 5000}
	n := TotalOdds(Threat, State{}, now, biome.TemperateForest, o, nil)
	for r := range n {
		id, ok := PickOdds(Threat, State{}, now, biome.TemperateForest, o, r, nil)
		if !ok || DefOf(id).UnderRaid {
			t.Fatalf("roll %d picked %v", r, id)
		}
	}
	if n != ThreatRolls {
		t.Fatalf("total %d", n)
	}
}

// TestPointGateDoesNotFlap: points hovering around a def's minimum never fire
// it more than once per cooldown window, and crossing the boundary alone does
// not lift the cooldown.
func TestPointGateDoesNotFlap(t *testing.T) {
	t.Parallel()
	const cooldown = 5
	list := []Def{{ID: 0, Category: Threat, Weight: 1, MinPoints: 300, Cooldown: cooldown}}
	var st State
	fired := []int{}
	for day := 0; day < 30; day++ {
		now := uint64(day) * clock.TicksPerDay
		o := Odds{Points: 299 + day%2*2} // 299, 301, 299, 301, ...
		if id, ok := pickOdds(list, Threat, st, now, biome.TemperateForest, o, 0, nil); ok {
			st.Fired(id, now)
			fired = append(fired, day)
		}
		if day%2 == 1 && len(fired) > 0 && fired[len(fired)-1] != day {
			// above the minimum inside the cooldown: still held
			if totalOdds(list, Threat, st, now, biome.TemperateForest, o, nil) != 0 && day-fired[len(fired)-1] < cooldown {
				t.Fatalf("day %d: boundary crossing lifted the cooldown", day)
			}
		}
	}
	for i := 1; i < len(fired); i++ {
		if fired[i]-fired[i-1] < cooldown {
			t.Fatalf("fired on days %v: inside the cooldown", fired)
		}
	}
	if len(fired) < 2 || len(fired) > 30/cooldown {
		t.Fatalf("fired on days %v", fired)
	}
}
