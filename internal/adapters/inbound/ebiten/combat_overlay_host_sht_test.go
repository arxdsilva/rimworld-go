// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// TestShotInfoShowsShootingScore (CBT-3.2, SHT-002): the hit card's
// "shooter" line carries Shooting accuracy points. Two Shooting 10
// colonists of a real colony aim at each other (the same distance both
// ways): the Deliberate aim one's line is higher, and each card's chance
// is the report's pure product (no flat term after range).
func TestShotInfoShowsShootingScore(t *testing.T) {
	t.Parallel()
	d, ok := scenario.Builtin("ember_clan") // a standing arrival: everyone is on the map at once
	if !ok {
		t.Fatal("no built-in scenario ember_clan")
	}
	s := &game.Setup{
		Scenario: d, Storyteller: "steady", Preset: difficulty.Seasoned, Difficulty: difficulty.Of(difficulty.Seasoned),
		Site: site.Site{Biome: biome.TemperateForest, Climate: biome.DefOf(biome.TemperateForest).Climate,
			Stones: []geology.Stone{geology.Granite}},
		Factions: faction.Generate(faction.DefaultSlots(), randx.New(7)),
		MapSize:  60, Season: game.SeasonSpring, Seed: 4242,
	}
	s.Colonists = game.NewCandidatePool(s.Seed, s.Scenario.Apply()).Selected()
	for i := range s.Colonists {
		p := &s.Colonists[i]
		p.Skills[skill.Shooting].Level = 10
		p.Traits, p.Conditions = nil, nil
	}
	s.Colonists[0].Traits = []trait.ID{trait.DeliberateAim}
	b := game.NewColonyBuilder(game.Config{Setup: s})
	var g *game.Game
	for i := 0; i < 200 && g == nil; i++ {
		if _, _, done := b.Step(1); done {
			g = b.Game()
		}
	}
	if g == nil {
		t.Fatal("the colony never finished building")
	}
	var ids []entity.EntityID
	for _, c := range g.Colonists() {
		ids = append(ids, c.ID)
	}
	slices.Sort(ids)
	if len(ids) < 2 {
		t.Fatalf("colonists %v", ids)
	}
	careful, plain := ids[0], ids[1] // spawned in Setup order
	g.Arm(careful, item.Rifle)
	g.Arm(plain, item.Rifle)
	shooterLine := func(from, to entity.EntityID) (int, int) {
		t.Helper()
		rep, ok := g.HitChance(from, to)
		if !ok || rep.Melee || rep.Dist < 2 || rep.Chance != rep.Total() {
			t.Fatalf("report %d -> %d: %+v (ok %v)", from, to, rep, ok)
		}
		info := shotInfo(nil, rep, 0, 0, 1, 1)
		if info.Chance != int(rep.Total())/10 {
			t.Fatalf("card chance %d, report total %d", info.Chance, rep.Total())
		}
		for _, f := range info.Factors {
			if f.Label == "shooter" {
				return f.Pct, rep.Dist
			}
		}
		t.Fatalf("no shooter line on %+v (report %+v)", info, rep)
		return 0, 0
	}
	c, cd := shooterLine(careful, plain)
	p, pd := shooterLine(plain, careful)
	t.Logf("shooter lines at %d tiles: deliberate aim %+d%%, no trait %+d%%", cd, c, p)
	if cd != pd || c <= p {
		t.Fatalf("shooter lines: deliberate aim %d%% at %d, no trait %d%% at %d; want higher at the same distance", c, cd, p, pd)
	}
	line := func(score int) int { return (int(attack.ShooterFactor(attack.ShooterPerTile(score), cd)) - 1000) / 10 }
	if c != line(15) || p != line(10) {
		t.Fatalf("shooter lines %d / %d, want score 15 -> %d and 10 -> %d", c, p, line(15), line(10))
	}
}
