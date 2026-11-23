// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// TestManipulationCostsShootingScore: manipulation adds 8 shooting score
// points per 100 %, capped at 100 % (SHT-001); the body's Add enters the
// stat sheet and ShotAim.
func TestManipulationCostsShootingScore(t *testing.T) {
	t.Parallel()
	for manip, want := range map[int32]int32{500: -4, 0: -8, 1000: 0, 1500: 0} {
		var got int32
		for _, m := range bodyMods(bodyKey{1000, manip, 1000}) {
			if m.Stat == stat.ShootAcc {
				got = m.Add
			}
		}
		if got != want {
			t.Errorf("manip %d: shoot_acc %+d, want %+d", manip, got, want)
		}
	}
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Ana", func(p *pawngen.Pawn) {
		p.Skills[skill.Shooting].Level = 10
		p.Traits, p.Conditions = nil, nil
	}))
	healthy, _ := combatSkills{g}.ShotAim(id)
	if want := attack.ShooterPerTile(attack.ShootingScore(10, 0)); healthy != want {
		t.Fatalf("healthy %d, want %d", healthy, want)
	}
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	b.Missing |= 1<<body.HandL | 1<<body.HandR
	g.bodyChanged(id, b, &v)
	manip := int(b.Capacity(body.Manipulation))
	pts := attack.ManipPoints(manip)
	if manip >= 1000 || pts >= 0 {
		t.Fatalf("no hands left manipulation %d, points %d", manip, pts)
	}
	if acc := int(g.statOf(id, stat.ShootAcc)); acc != pts {
		t.Fatalf("shoot_acc %d, want %d", acc, pts)
	}
	hurt, _ := combatSkills{g}.ShotAim(id)
	if want := attack.ShooterPerTile(attack.ShootingScore(10, pts)); hurt != want || hurt >= healthy {
		t.Fatalf("hurt %d, want %d below %d", hurt, want, healthy)
	}
}

// TestSoftCapInGame: past the day's 4000 XP a skill still learns at 20 %,
// saturated reads true, and the next day trains at full rate (SHT-302).
func TestSoftCapInGame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Kit", func(p *pawngen.Pawn) {
		p.Skills[skill.Mining] = skill.Skill{Level: 3, Passion: skill.Minor}
		p.Traits = []trait.ID{trait.Gentle}
	}))
	xp := func() int64 { return roster.TotalXP(g.skillOf(id, skill.Mining)) }
	before := xp()
	for g.learnLogOf(id).today[skill.Mining] < skill.SoftCapDay {
		g.learn(id, skill.Mining, 50)
	}
	atCap := xp() - before
	if c, _ := g.Dossier(id); !c.Saturated[skill.Mining] {
		t.Fatal("not saturated at the cap")
	}
	for range 500 {
		g.learn(id, skill.Mining, 10)
	}
	if extra := xp() - before - atCap; extra < 900 || extra > 1100 { // 5000 raw at 20 %
		t.Fatalf("past the cap gained %d, want about 1000", extra)
	}
	g.clock.Advance(24 * 2500)
	mark := xp()
	g.learn(id, skill.Mining, 100)
	if got := xp() - mark; got != 100 {
		t.Fatalf("next day gained %d, want 100", got)
	}
}

// TestDecayInGame: the daily decay table (L15 loses 540, L9 nothing).
func TestDecayInGame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Ox", func(p *pawngen.Pawn) {
		p.Skills[skill.Mining] = skill.Skill{Level: 15, Passion: skill.Minor}
		p.Skills[skill.Plants] = skill.Skill{Level: 9, Passion: skill.Minor}
	}))
	m, pl := roster.TotalXP(g.skillOf(id, skill.Mining)), roster.TotalXP(g.skillOf(id, skill.Plants))
	g.dayPasses(id)
	if d := m - roster.TotalXP(g.skillOf(id, skill.Mining)); d != 540 {
		t.Errorf("L15 lost %d, want 540", d)
	}
	if d := pl - roster.TotalXP(g.skillOf(id, skill.Plants)); d != 0 {
		t.Errorf("L9 lost %d", d)
	}
}

// TestScrapperBrawler: the Scrapper trait sets Melee.Brawler (+4 hit).
func TestScrapperBrawler(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	mk := func(name string, tr ...trait.ID) attack.Melee {
		id := spawnGen(t, g, 2, 2, genPawn(name, func(p *pawngen.Pawn) { p.Traits = tr }))
		return g.meleeOf(id)
	}
	s, n := mk("Sam", trait.Scrapper), mk("Nan")
	if !s.Brawler || n.Brawler {
		t.Fatalf("brawler %v / %v", s.Brawler, n.Brawler)
	}
	if a, b := attack.HitChanceOf(s), attack.HitChanceOf(n); a <= b {
		t.Fatalf("hit chance %d vs %d", a, b)
	}
}

// TestLightFootedNimble: the LightFooted trait sets Melee.Nimble, adding
// exactly 15 000 to the dodge score, and the dossier dodge line shows it (SHT-105).
func TestLightFootedNimble(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	mk := func(name string, tr ...trait.ID) entity.EntityID {
		return spawnGen(t, g, 2, 2, genPawn(name, func(p *pawngen.Pawn) {
			p.Traits = tr
			p.Skills[skill.Melee].Level = 4
		}))
	}
	b, n := mk("Bea", trait.LightFooted), mk("Nan")
	mb, mn := g.meleeOf(b), g.meleeOf(n)
	if !mb.Nimble || mn.Nimble {
		t.Fatalf("nimble %v / %v", mb.Nimble, mn.Nimble)
	}
	same := mb
	same.Nimble = false
	if same != mn {
		t.Fatalf("other melee numbers differ: %+v vs %+v", same, mn)
	}
	if d := attack.DodgeScore(mb) - attack.DodgeScore(mn); d != 15_000 {
		t.Fatalf("dodge score delta %d, want 15000", d)
	}
	dodge := func(id entity.EntityID) int {
		for _, l := range card(t, g, id).ExtraLines[skill.Melee] {
			if l.Label == "Melee dodge chance" {
				return l.Pct
			}
		}
		t.Fatal("no dodge line")
		return 0
	}
	if db, dn := dodge(b), dodge(n); db <= dn || db != attack.DodgeChanceOf(mb)/10 {
		t.Fatalf("dodge lines %d vs %d, chance %d", db, dn, attack.DodgeChanceOf(mb)/10)
	}
}

// TestMaxedSkillLogsBankedGain: a skill at the level cap banks only up to
// its cushion, and the day's log counts what was banked, not what was fed.
func TestMaxedSkillLogsBankedGain(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Max", func(p *pawngen.Pawn) {
		p.Skills[skill.Mining] = skill.Skill{Level: skill.MaxLevel, XP: int32(skill.Cost(skill.MaxLevel) - 100), Passion: skill.Minor}
	}))
	before := roster.TotalXP(g.skillOf(id, skill.Mining))
	for range 500 {
		g.learn(id, skill.Mining, 100)
	}
	banked := roster.TotalXP(g.skillOf(id, skill.Mining)) - before
	if got := int64(g.learnLogOf(id).today[skill.Mining]); got != banked || banked >= 1000 {
		t.Fatalf("logged %d, banked %d of 50000 fed, want 100 at most", got, banked)
	}
}
