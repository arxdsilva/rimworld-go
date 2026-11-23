// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
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
	var band int
	shooterLine := func(from, to entity.EntityID) (int, int) {
		t.Helper()
		rep, ok := g.HitChance(from, to)
		if !ok || rep.Melee || rep.Dist < 2 || rep.Chance != rep.Total() {
			t.Fatalf("report %d -> %d: %+v (ok %v)", from, to, rep, ok)
		}
		band = int(rep.Band)
		info := overlay.ShotInfoOf(nil, rep, 0, 0, 1, 1)
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
	// The shooter line is the step's own effect on the chain (SHT-201).
	chainLine := func(score int) int {
		f := attack.ShooterFactor(attack.ShooterPerTile(score), cd)
		return (band*int(f)/1000)/10 - band/10
	}
	if c != chainLine(15) || p != chainLine(10) {
		t.Fatalf("shooter lines %d / %d, want score 15 -> %d and 10 -> %d", c, p, chainLine(15), chainLine(10))
	}
}

// fakeOdds is a hitChancer with one fixed report.
type fakeOdds struct {
	rep attack.Report
	ok  bool
}

func (f fakeOdds) HitChance(entity.EntityID, entity.EntityID) (attack.Report, bool) {
	return f.rep, f.ok
}

// TestShotOverlayUndrafted (SHT-202): the card is built for a selected
// colonist whether or not drafted, and is the same card either way; with no
// report (no weapon, no target) there is none.
func TestShotOverlayUndrafted(t *testing.T) {
	t.Parallel()
	rep := attack.Report{Band: 800, Shooter: 1100, Weather: 1000, Size: 900, Light: 1000, Cover: 250,
		InRange: true, InSight: true}
	o := fakeOdds{rep: rep, ok: true}
	var drafted, undrafted overlay.Combat
	sel := game.PawnView{ID: 1, X: 2, Y: 3}
	hover := game.PawnView{ID: 9, X: 7, Y: 3}
	un, ok := shotCard(o, &undrafted, sel, hover)
	if !ok || len(undrafted.Factors) == 0 {
		t.Fatalf("an undrafted colonist gets a card: ok %v factors %v", ok, undrafted.Factors)
	}
	sel.Drafted = true
	dr, ok := shotCard(o, &drafted, sel, hover)
	if !ok || un.Chance != dr.Chance || !slices.Equal(undrafted.Factors, drafted.Factors) || un.Chance != int(rep.Total())/10 {
		t.Fatalf("drafted card %+v, undrafted %+v", dr, un)
	}
	if _, ok := shotCard(fakeOdds{}, &undrafted, sel, hover); ok {
		t.Fatal("no report, no card")
	}
}
