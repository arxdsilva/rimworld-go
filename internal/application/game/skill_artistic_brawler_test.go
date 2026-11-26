// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/trait"
)

// All sculpture recipes must use Artistic
// as their sole skill, including learning, save/load and the Bio read model.
func TestArtisticIsTheSingleLiveSkillForSculptureAndSave(t *testing.T) {
	t.Parallel()
	for _, k := range []recipe.ID{recipe.SculptSmall, recipe.SculptLarge, recipe.SculptGrand} {
		if d := recipe.Core().DefOf(k); d.Learn != skill.Artistic {
			t.Fatalf("%s trains %s, want artistic", d.Name, d.Learn.Key())
		}
	}
	g := New(Config{Seed: 100, Width: 18, Height: 10})
	id := spawnGen(t, g, 3, 3, genPawn("Sculptor", func(p *pawngen.Pawn) {
		p.Skills[skill.Artistic] = skill.Skill{Level: 12, Passion: skill.Major}
		p.Skills[skill.Crafting] = skill.Skill{Level: 0}
		p.Skills[skill.Construction] = skill.Skill{Level: 0}
	}))
	if g.skillOf(id, skill.Artistic).Level != 12 {
		t.Fatal("Artistic was not installed as a live skill")
	}
	if (mindMuses{g}).SkillLevel(id, skill.Artistic) < 3 {
		t.Fatal("Artistic did not reach inspiration through the normal skill host")
	}
	g.learn(id, skill.Artistic, 500)
	live := g.skillOf(id, skill.Artistic)
	if live.XP == 0 || g.skillOf(id, skill.Crafting).Level != 0 {
		t.Fatalf("Artistic learning used another skill: artistic=%+v crafting=%+v", live, g.skillOf(id, skill.Crafting))
	}
	h := saveLoad(t, g)
	if got := h.skillOf(id, skill.Artistic); got != live {
		t.Fatalf("saved Artistic=%+v, want %+v", got, live)
	}
	if b, ok := h.bioView(id); !ok || len(b.Skills) != int(skill.NumSkills) || b.Skills[skill.Artistic].Level != 12 {
		t.Fatalf("Artistic missing from Bio view: %+v", b)
	}
}

func TestBrawlerAffectsMeleeShootingAndMood(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 19, Width: 18, Height: 10})
	id := spawnGen(t, g, 3, 3, genPawn("Brawler", func(p *pawngen.Pawn) {
		p.Traits = []trait.ID{trait.Scrapper}
		p.Skills[skill.Melee] = skill.Skill{Level: 11}
		p.Skills[skill.Shooting] = skill.Skill{Level: 0}
	}))
	if !g.meleeOf(id).Brawler || (mindMuses{g}).CanShoot(id) {
		t.Fatal("Brawler did not affect combat or shooting inspiration eligibility")
	}
	g.equip[id] = item.Rifle
	found := false
	for _, thought := range g.appendThoughts(nil, id) {
		if thought.Kind == mood.BrawlerRanged {
			found = true
			if thought.Value != -10 {
				t.Fatalf("ranged weapon thought %+v", thought)
			}
		}
	}
	if !found {
		t.Fatal("Brawler equipped a rifle without its ranged mood penalty")
	}
	g.equip[id] = item.Knife
	for _, thought := range g.appendThoughts(nil, id) {
		if thought.Kind == mood.BrawlerRanged {
			t.Fatal("Brawler retained ranged weapon thought after equipping a knife")
		}
	}
}
