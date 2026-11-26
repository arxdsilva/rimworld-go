// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/geology"
	miningrules "rimworld-go/internal/domain/world/mining"
	"rimworld-go/internal/domain/world/terrain"
)

// seedForMiningRolls finds a deterministic base seed whose first mining
// stream rolls have the requested outcome. The probe game is discarded, so
// the returned seed's stream is untouched when the test uses it.
func seedForMiningRolls(t *testing.T, rolls int, wantDrop bool) uint64 {
	t.Helper()
	for seed := uint64(1); seed < 10000; seed++ {
		g := New(Config{Seed: seed, Width: 4, Height: 4})
		rng := g.stream(streamMining)
		matches := true
		for range rolls {
			if (rng.IntN(4) == 0) != wantDrop {
				matches = false
				break
			}
		}
		if matches {
			return seed
		}
	}
	t.Fatal("could not find a deterministic mining seed")
	return 0
}

func TestMineNaturalRockChunkChance(t *testing.T) {
	t.Parallel()
	for _, wantDrop := range []bool{true, false} {
		g := New(Config{Seed: seedForMiningRolls(t, 1, wantDrop), Width: 10, Height: 5})
		g.SpawnNamedPawn(1, 2, "Bo")
		g.setTerrain(6, 2, terrain.Rock)
		g.Enqueue(command.Command{Kind: command.DesignateMine, X: 6, Y: 2, X2: 6, Y2: 2})
		g.Step(TicksPerCell*20 + g.mineWorkAt(6, 2)*2 + 10)
		gotDrop := g.stockOf(item.GraniteChunk) > 0
		if gotDrop != wantDrop {
			t.Errorf("seeded chunk roll drop=%v, want %v", gotDrop, wantDrop)
		}
	}
}

func TestMineWorkAtUsesMaterialHP(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 4})
	for x, c := range []struct {
		stone string
		want  int
	}{
		{"sandstone", 600},
		{"marble", 675},
		{"slate", 750},
		{"limestone", 1050},
		{"granite", 1350},
	} {
		if !g.SetGround(x, 0, x, 0, "rock", c.stone, "") {
			t.Fatalf("set %s ground", c.stone)
		}
		if got := g.mineWorkAt(x, 0); got != c.want {
			t.Errorf("%s: %d ticks, want %d", c.stone, got, c.want)
		}
	}
	for x, c := range []struct {
		ore  string
		want int
	}{
		{"steel", 2250},
		{"silver", 2250},
		{"gold", 2250},
		{"machinery", 3000}, {"jade", 2250}, {"uranium", 6000}, {"plasteel", 12000},
	} {
		if !g.SetGround(x, 1, x, 1, "rock", "granite", c.ore) {
			t.Fatalf("set %s ground", c.ore)
		}
		if got := g.mineWorkAt(x, 1); got != c.want {
			t.Errorf("%s: %d ticks, want %d", c.ore, got, c.want)
		}
	}
}

// An otherwise neutral level-eight worker with impaired sight must not
// bypass the sight factor through the neutral WorkSpeed fast path.
func TestMiningLiveCapacitiesAndSave(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	id, _ := g.SpawnNamedPawn(3, 3, "Ada")
	setSkill(g, id, skill.Mining, 8)
	if !g.DevLosePart(id, "left eye") {
		t.Fatal("eye loss fixture")
	}
	sight, manip := g.capacity(id, body.Sight), g.capacity(id, body.Manipulation)
	if sight >= 1000 {
		t.Fatalf("sight %d", sight)
	}
	want := miningrules.Yield(8, manip, sight)
	if got := g.miningYieldPermil(id); got != want || got >= 1000 {
		t.Fatalf("native yield %d want %d", got, want)
	}
	rate := int(g.statOf(id, stat.WorkSpeed)) * miningrules.SightSpeed(sight) / 1000
	units := 0
	for range 1000 {
		units += g.workProgress(id, skill.Mining, true)
	}
	if units != rate {
		t.Fatalf("work units %d want %d", units, rate)
	}
	if !g.SetGround(5, 3, 5, 3, "rock", "granite", "steel") {
		t.Fatal("ore")
	}
	applyNow(g, command.Command{Kind: command.DesignateMine, X: 5, Y: 3, X2: 5, Y2: 3})
	for range 400 {
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Mine && j.State == job.Do {
			break
		}
		g.Step(1)
	}
	j, _ := g.jobs.Get(id)
	if j.Kind != job.Mine || j.State != job.Do || j.WorkLeft <= 0 {
		t.Fatalf("mining did not start: %+v", j)
	}
	g.Step(200)
	h := saveLoad(t, g)
	g.World()
	h.World()
	for range 120 {
		runFed(g, 1, id)
		runFed(h, 1, id)
		if g.stockOf(item.Steel) > 0 {
			break
		}
	}
	if g.stockOf(item.Steel) != h.stockOf(item.Steel) || g.stockOf(item.Steel) <= 0 || g.StateHash() != h.StateHash() {
		t.Fatalf("mining replay: %d/%d", g.stockOf(item.Steel), h.stockOf(item.Steel))
	}
}

func TestMiningScannerCellYieldAgreement(t *testing.T) {
	for _, c := range []struct {
		ore geology.Ore
		qty int
	}{{geology.Steel, 40}, {geology.Silver, 40}, {geology.Gold, 40}, {geology.Machinery, 2}, {geology.Plasteel, 40}, {geology.Uranium, 40}, {geology.Jade, 40}} {
		if OreYield[c.ore] != c.qty {
			t.Fatalf("%s base yield %d want %d", c.ore, OreYield[c.ore], c.qty)
		}
	}
}

func TestMiningChunkStreamContinuesAfterLoad(t *testing.T) {
	g := New(Config{Seed: seedForMiningRolls(t, 1, true), Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(3, 3, "Ada")
	setSkill(g, id, skill.Mining, 8)
	for x := 5; x <= 7; x++ {
		g.SetGround(x, 3, x, 3, "rock", "granite", "")
	}
	applyNow(g, command.Command{Kind: command.DesignateMine, X: 5, Y: 3, X2: 5, Y2: 3})
	for range 80 {
		if g.TerrainAt(5, 3) != "rock" {
			break
		}
		runFed(g, 1, id)
	}
	if g.stockOf(item.GraniteChunk) != 1 {
		t.Fatal("first roll did not drop")
	}
	h := saveLoad(t, g)
	for _, w := range []*Game{g, h} {
		applyNow(w, command.Command{Kind: command.DesignateMine, X: 6, Y: 3, X2: 7, Y2: 3})
	}
	for range 150 {
		runFed(g, 1, id)
		runFed(h, 1, id)
		if g.TerrainAt(6, 3) != "rock" && g.TerrainAt(7, 3) != "rock" {
			break
		}
	}
	if g.TerrainAt(7, 3) == "rock" || g.StateHash() != h.StateHash() {
		t.Fatal("chunk RNG / mining did not continue identically")
	}
}

func TestMineNeedsCardinalWorkSpot(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 12})
	id, _ := g.SpawnNamedPawn(3, 3, "Miner")
	for _, pos := range [][2]int{{6, 6}, {5, 6}, {7, 6}, {6, 5}, {6, 7}} {
		g.setTerrain(pos[0], pos[1], terrain.Rock)
	}
	if _, _, ok := g.reachableSpot(id, 6, 6); !ok {
		t.Fatal("fixture lacks reachable diagonal")
	}
	if g.startMine(id, 6, 6, false) {
		t.Fatal("diagonal mining admitted")
	}
}
