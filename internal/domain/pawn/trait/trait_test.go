// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trait

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

func TestCatalogue(t *testing.T) {
	all := Core().All()
	if len(all) != 43 { // SHT-105: LightFooted
		t.Fatalf("%d traits, want 43", len(all))
	}
	keys, labels := map[string]bool{}, map[string]bool{}
	for i, d := range all {
		if d.ID != ID(i+1) || d.Key == "" || d.Label == "" || d.Desc == "" || d.Effects() == "" {
			t.Fatalf("trait %d incomplete: %+v (effects %q)", i+1, d, d.Effects())
		}
		if keys[d.Key] || labels[d.Label] {
			t.Fatalf("duplicate %q", d.Key)
		}
		keys[d.Key], labels[d.Label] = true, true
		if got, ok := Core().Parse(d.Key); !ok || got != d.ID {
			t.Fatalf("Core().Parse(%q)", d.Key)
		}
		if Core().DefOf(d.ID).Key != d.Key {
			t.Fatal("DefOf")
		}
	}
	if _, ok := Core().Parse("nope"); ok {
		t.Fatal("parsed unknown key")
	}
	if Core().DefOf(NumTraits).Key != "" || Core().DefOf(None).Key != "" {
		t.Fatal("DefOf out of range")
	}
}

func TestConflicts(t *testing.T) {
	cases := []struct {
		a, b ID
		want bool
	}{
		{Tireless, Unhurried, true}, {Sunny, Brooding, true}, {Steady, Brittle, true},
		{FleetFooted, Plodding, true}, {HeartyEater, LightEater, true}, {Roughneck, Fussy, true},
		{DeliberateAim, QuickDraw, true}, {Gentle, DeliberateAim, true}, {QuickDraw, Gentle, true},
		{Sunny, Sunny, true}, {Sturdy, Sturdy, true},
		{Tireless, Sunny, false}, {GreenThumb, Bookish, false}, {Gentle, Sturdy, false},
		{None, Sunny, false}, {Sunny, NumTraits, false},
	}
	for _, c := range cases {
		if Core().Conflicts(c.a, c.b) != c.want || Core().Conflicts(c.b, c.a) != c.want {
			t.Fatalf("Core().Conflicts(%d,%d) want %v", c.a, c.b, c.want)
		}
	}
}

func TestClashes(t *testing.T) {
	soldier := backstory.Def{Skills: map[skill.ID]int8{skill.Shooting: 4}}
	hurt := backstory.Def{Skills: map[skill.ID]int8{skill.Shooting: -2}}
	noPlants := backstory.Def{Incapable: backstory.Plants}
	pacifist := backstory.Def{Incapable: backstory.Violence}
	switch {
	case !Core().Clashes(Gentle, soldier):
		t.Fatal("gentle soldier")
	case Core().Clashes(Gentle, hurt):
		t.Fatal("a negative skill delta is no clash")
	case !Core().Clashes(GreenThumb, noPlants):
		t.Fatal("green thumb without plants")
	case !Core().Clashes(QuickDraw, pacifist):
		t.Fatal("quick draw pacifist")
	case Core().Clashes(Gentle, pacifist), Core().Clashes(Sunny, soldier), Core().Clashes(None, soldier), Core().Clashes(NumTraits, soldier):
		t.Fatal("false clash")
	}
}

func TestEffectsText(t *testing.T) {
	cases := map[ID][]string{
		Tireless:    {"Work speed +20%"},
		Unhurried:   {"Work speed -20%", "Recreation drain -15%"},
		Roughneck:   {"\"slept outside\" x0.25", "\"ate raw food\" x0.25"},
		Fussy:       {"x2", "\"slept in my bedroom\" x1.5"},
		HeartyEater: {"Hunger rate +25%", "\"ate a fine meal\" +2", "\"ate a lavish meal\" +2"},
		Cannibal:    {"Enjoys human flesh"},
		Bookish:     {"Intellectual speed +25%", "Intellectual learning x1.5"},
		Gentle:      {"Social +3 levels", "Incapable: Violence"},
		ColdHardy:   {"Cold comfort -6 °C"},
		Sturdy:      {"Pain tolerance +15%"},
		Steady:      {"Break threshold -6"},
	}
	for id, wants := range cases {
		e := Core().DefOf(id).Effects()
		for _, w := range wants {
			if !strings.Contains(e, w) {
				t.Fatalf("%s effects %q lack %q", Core().DefOf(id).Label, e, w)
			}
		}
	}
	if pct(125) != "1.25" || pct(5) != "0.05" || pct(300) != "3" {
		t.Fatal("pct")
	}
}

func TestTraitStatMods(t *testing.T) {
	s := stat.Build(stat.Base(), Core().DefOf(Tireless).Mods, Core().DefOf(FleetFooted).Mods, Core().DefOf(Sunny).Mods)
	if s[stat.WorkSpeed] != 1200 || s[stat.MoveSpeed] != 1150 || s[stat.MoodOffset] != 8 {
		t.Fatalf("%v", s)
	}
	// SHT-101: the trigger traits' wiki numbers, as score points and aim time
	for id, want := range map[ID][2]int{DeliberateAim: {5, 1250}, QuickDraw: {-5, 500}} {
		s := stat.Build(stat.Base(), Core().DefOf(id).Mods)
		if int(s[stat.ShootAcc]) != want[0] || int(s[stat.AimCooldown]) != want[1] {
			t.Fatalf("%s: shoot_acc %d aim %d, want %v", Core().DefOf(id).Label, s[stat.ShootAcc], s[stat.AimCooldown], want)
		}
	}
}

// TestGenerated: legacy rules roll only the original 19 traits (old seeds
// keep their pawns); otherwise every trait, in ID order.
func TestGenerated(t *testing.T) {
	legacy, all := Core().Generated(true), Core().Generated(false)
	if len(legacy) != 19 || legacy[0] != Tireless || legacy[18] != Sturdy {
		t.Fatalf("legacy = %v", legacy)
	}
	if len(all) != int(NumTraits)-1 || all[len(all)-1] != LightFooted {
		t.Fatalf("all = %v", all)
	}
	for i, id := range all {
		if id != ID(i+1) {
			t.Fatalf("all[%d] = %d", i, id)
		}
	}
}

// TestSocialTraitGroups: the ENG-003 groups exclude each other within, and
// orientation traits are rare.
func TestSocialTraitGroups(t *testing.T) {
	for _, c := range [][2]ID{{Striking, Grotesque}, {Fair, Homely}, {Gay, Bisexual}, {Asexual, Gay},
		{Abstainer, Hedonist}, {Indulgent, Abstainer}, {AugmentFan, NaturalPurist}, {DistrustsMen, DistrustsWomen},
		{Idle, Tireless}, {Bloodthirsty, Gentle}} {
		if !Core().Conflicts(c[0], c[1]) {
			t.Errorf("%s and %s should conflict", Core().DefOf(c[0]).Label, Core().DefOf(c[1]).Label)
		}
	}
	if Core().Conflicts(Striking, Gay) || Core().Conflicts(Gracious, ColdHearted) {
		t.Error("false conflict")
	}
	if Core().DefOf(Gay).Rarity != 10 || Core().DefOf(Bisexual).Rarity != 10 || Core().DefOf(Asexual).Rarity != 20 || Core().DefOf(Striking).Rarity != 0 {
		t.Error("orientation rarity")
	}
	if Core().DefOf(Striking).Beauty != 2 || Core().DefOf(Grotesque).Beauty != -2 {
		t.Error("beauty")
	}
	if e := Core().DefOf(Idle).Effects(); !strings.Contains(e, "Work speed -40%") {
		t.Errorf("idle effects %q", e)
	}
	for _, d := range Core().All() {
		if strings.Contains(d.Note, "{") {
			t.Errorf("%s: placeholder in note %q", d.Label, d.Note)
		}
	}
}

func TestBook(t *testing.T) {
	core := Core()
	if core.Len() != int(NumTraits)-1 || core.Key(Sunny) != "core.sunny" || len(core.Keys()) != core.Len() || core.Keys()[0] != "core.tireless" {
		t.Fatalf("core book: %d %q", core.Len(), core.Key(Sunny))
	}
	var zero Book
	if zero.Len() != core.Len() || zero.Key(Sunny) != core.Key(Sunny) || zero.DefOf(Sunny).Key != "sunny" {
		t.Fatal("zero Book is core")
	}
	list := core.All()
	keys := core.Keys()
	list = append(list, Def{Key: "x.brave", Label: "Brave", Rarity: 3})
	keys = append(keys, "x.brave")
	b := NewBook(list, keys)
	id, ok := b.IDOf("x.brave")
	if !ok || id != NumTraits || b.DefOf(id).ID != id || b.Len() != core.Len()+1 {
		t.Fatalf("plugin trait id %d %v", id, ok)
	}
	if b.DefOf(Sunny).Label != "Sunny" || b.Key(Sunny) != "core.sunny" {
		t.Fatal("core ids kept")
	}
	if p, ok := b.Parse("x.brave"); !ok || p != id {
		t.Fatal("Parse namespaced")
	}
	if g := b.Generated(true); len(g) != int(legacyTraits)-1 {
		t.Fatalf("legacy draws only the original set, got %d", len(g))
	}
	if g := b.Generated(false); len(g) != b.Len() || g[len(g)-1] != id {
		t.Fatal("plugin trait generated outside legacy")
	}
	if !b.Conflicts(id, id) || b.Conflicts(id, Sunny) {
		t.Fatal("conflicts")
	}
}

// TestScrapperTrait (SHT-104): the brawler numbers, the Bio-tab strings and
// the generation and conflict rules.
func TestScrapperTrait(t *testing.T) {
	d := Core().DefOf(Scrapper)
	if d.Key != "scrapper" || d.Label != "Scrapper" || d.Note == "" || d.Desc == "" || d.Needs != backstory.Violence {
		t.Fatalf("def = %+v", d)
	}
	if d.SkillBonus[skill.Melee] != 4 || d.SkillBonus[skill.Shooting] != -10 {
		t.Fatalf("skill bonus = %v", d.SkillBonus)
	}
	if id, ok := Core().Parse("scrapper"); !ok || id != Scrapper {
		t.Fatalf("Parse = %d %v", id, ok)
	}
	if !slices.Contains(Core().Generated(false), Scrapper) || slices.Contains(Core().Generated(true), Scrapper) {
		t.Fatal("Scrapper is generated outside Legacy rules only")
	}
	if !Core().Conflicts(Scrapper, DeliberateAim) || !Core().Conflicts(Scrapper, QuickDraw) {
		t.Fatal("Scrapper conflicts with the trigger group")
	}
}

// TestLightFootedTrait (SHT-105): the dodge note, the generation rule and
// the absence of any conflict or requirement.
func TestLightFootedTrait(t *testing.T) {
	d := Core().DefOf(LightFooted)
	if d.Key != "light_footed" || d.Label != "Light-footed" || d.Note != "Melee dodge +15" || d.Desc == "" {
		t.Fatalf("def = %+v", d)
	}
	if d.Needs != 0 || d.Group != 0 || len(d.SkillBonus) != 0 {
		t.Fatalf("needs/group/bonus = %v %v %v", d.Needs, d.Group, d.SkillBonus)
	}
	if len(d.Effects()) == 0 {
		t.Fatal("no effects")
	}
	if id, ok := Core().Parse("light_footed"); !ok || id != LightFooted {
		t.Fatalf("Parse = %d %v", id, ok)
	}
	if !slices.Contains(Core().Generated(false), LightFooted) || slices.Contains(Core().Generated(true), LightFooted) {
		t.Fatal("LightFooted is generated outside Legacy rules only")
	}
	for id := ID(1); id < NumTraits; id++ {
		if id != LightFooted && Core().Conflicts(LightFooted, id) {
			t.Fatalf("conflicts with %d", id)
		}
	}
}
