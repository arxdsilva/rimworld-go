// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
)

func trSame(t *testing.T, got, want trait.Book) {
	t.Helper()
	if got.Len() != want.Len() {
		t.Fatalf("%d traits, want %d", got.Len(), want.Len())
	}
	for i := 0; i <= want.Len(); i++ {
		id := trait.ID(i)
		if g, w := got.DefOf(id), want.DefOf(id); !reflect.DeepEqual(g, w) {
			t.Fatalf("id %d: %+v\nwant %+v", id, g, w)
		}
		if g, w := got.Key(id), want.Key(id); g != w {
			t.Fatalf("id %d: key %q, want %q", id, g, w)
		}
	}
}

func trPlugin() map[string]any {
	return map[string]any{"label": "Brave", "desc": "{name} does not flinch.", "group": "nerve",
		"mods":        []any{map[string]any{"stat": "break_threshold", "add": -3.0}, map[string]any{"stat": "move_speed", "mul": 1050.0}},
		"skill_bonus": map[string]any{"melee": 2.0}, "work_speed": map[string]any{"melee": 10.0}, "learn_mul": map[string]any{"melee": 120.0},
		"incapable": []any{"intellectual"}, "needs": []any{"violence"}, "rarity": 3.0, "beauty": 1.0}
}

// TestTraitCodecRoundTripsCore: decoding core's encoded defs (straight, and
// through JSON) gives core back, def for def and key for key.
func TestTraitCodecRoundTripsCore(t *testing.T) {
	t.Parallel()
	core := trait.Core()
	defs := EncodeTraits(core)
	if len(defs) != core.Len() {
		t.Fatalf("%d defs for %d traits", len(defs), core.Len())
	}
	for i, d := range defs {
		if d.Class != TraitClass || d.Plugin != Core || d.Key != core.Key(trait.ID(i+1)) {
			t.Fatalf("def %d: %s by %q", i, d.Ref(), d.Plugin)
		}
	}
	for name, in := range map[string][]Def{"trees": defs, "json": bsJSON(t, defs)} {
		got, probs := DecodeTraits(in)
		if len(probs) > 0 {
			t.Fatalf("%s: %v", name, probs)
		}
		trSame(t, got, core)
	}
}

// TestTraitPluginDefFollowsCore: a plugin's trait after core's takes the
// next ID under its own key, every field typed, and shares core's conflict
// group by name.
func TestTraitPluginDefFollowsCore(t *testing.T) {
	t.Parallel()
	defs := append(EncodeTraits(trait.Core()), Def{Class: TraitClass, Key: "moremods.brave", Plugin: "moremods", File: "defs/t.json", Body: trPlugin()})
	got, probs := DecodeTraits(bsJSON(t, defs))
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	id := trait.NumTraits
	d := got.DefOf(id)
	if got.Len() != trait.Core().Len()+1 || got.Key(id) != "moremods.brave" || d.ID != id || d.Key != "moremods.brave" || d.Label != "Brave" ||
		d.Group != trait.GroupNerve || d.Rarity != 3 || d.Beauty != 1 || d.Incapable != backstory.Intellectual || d.Needs != backstory.Violence ||
		d.SkillBonus[skill.Melee] != 2 || d.WorkSpeed[skill.Melee] != 10 || d.LearnMul[skill.Melee] != 120 ||
		!reflect.DeepEqual(d.Mods, []stat.Mod{{Stat: stat.BreakThreshold, Add: -3}, {Stat: stat.MoveSpeed, Mul: 1050}}) {
		t.Fatalf("plugin def %+v", d)
	}
	if !got.Conflicts(id, trait.Steady) || got.Conflicts(id, trait.Sunny) {
		t.Fatal("brave shares the nerve group with Steady only")
	}
	if !strings.Contains(d.Effects(), "Melee speed +10%") {
		t.Fatalf("effects %q", d.Effects())
	}
	trSame(t, trait.NewBook(got.All()[:trait.Core().Len()], got.Keys()[:trait.Core().Len()]), trait.Core())
}

// TestTraitOwnGroup: a group name core lacks is the plugin's own, shared by
// every trait naming it and clear of core's groups.
func TestTraitOwnGroup(t *testing.T) {
	t.Parallel()
	mk := func(key, group string) Def {
		b := map[string]any{"label": "T", "group": group}
		return Def{Class: TraitClass, Key: key, Plugin: "p", Body: b}
	}
	got, probs := DecodeTraits([]Def{mk("p.a", "courage"), mk("p.b", "courage"), mk("p.c", "wits")})
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	core := trait.ID(1) // IDs here start at 1: a book of three plugin traits
	a, b, c := core, core+1, core+2
	if !got.Conflicts(a, b) || got.Conflicts(a, c) || got.DefOf(a).Group < trait.FirstPluginGroup {
		t.Fatalf("groups %d %d %d", got.DefOf(a).Group, got.DefOf(b).Group, got.DefOf(c).Group)
	}
}

// TestTraitDecodeProblems: every strictness rule names its field, and a def
// with any problem is left out while the rest load.
func TestTraitDecodeProblems(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		edit  func(map[string]any)
		field string
		msg   string
	}{
		"label required":   {func(b map[string]any) { delete(b, "label") }, "label", "required"},
		"label too long":   {func(b map[string]any) { b["label"] = strings.Repeat("x", 31) }, "label", "characters"},
		"unknown field":    {func(b map[string]any) { b["colour"] = "red" }, "colour", "unknown field"},
		"label type":       {func(b map[string]any) { b["label"] = 4.0 }, "label", "want a string"},
		"unknown stat":     {func(b map[string]any) { b["mods"] = []any{map[string]any{"stat": "luck", "add": 1.0}} }, "mods.0.stat", "unknown stat"},
		"empty mod":        {func(b map[string]any) { b["mods"] = []any{map[string]any{"stat": "move_speed"}} }, "mods.0", "non-zero"},
		"mod stat needed":  {func(b map[string]any) { b["mods"] = []any{map[string]any{"add": 1.0}} }, "mods.0.stat", "required"},
		"mul out of range": {func(b map[string]any) { b["mods"] = []any{map[string]any{"stat": "move_speed", "mul": 99999.0}} }, "mods.0.mul", "out of range"},
		"fractional":       {func(b map[string]any) { b["rarity"] = 1.5 }, "rarity", "whole number"},
		"unknown skill":    {func(b map[string]any) { b["skill_bonus"] = map[string]any{"flying": 1.0} }, "skill_bonus.flying", "unknown skill"},
		"bonus range":      {func(b map[string]any) { b["skill_bonus"] = map[string]any{"melee": 50.0} }, "skill_bonus.melee", "out of range"},
		"unknown bit":      {func(b map[string]any) { b["incapable"] = []any{"flying"} }, "incapable.0", "unknown key"},
		"beauty range":     {func(b map[string]any) { b["beauty"] = 5.0 }, "beauty", "out of range"},
		"bad group":        {func(b map[string]any) { b["group"] = "Two Words" }, "group", "lowercase name"},
		"note is code":     {func(b map[string]any) { b["note"] = "Enjoys human flesh" }, "note", "game's own behaviour"},
		"thought scale":    {func(b map[string]any) { b["thought_scale"] = map[string]any{"ate raw": 25.0} }, "thought_scale", "game's own behaviour"},
		"add thought":      {func(b map[string]any) { b["add_thought"] = map[string]any{"ate fine meal": 2.0} }, "add_thought", "game's own behaviour"},
	} {
		body := trPlugin()
		c.edit(body)
		good := Def{Class: TraitClass, Key: "p.good", Plugin: "p", Body: map[string]any{"label": "Good"}}
		b, probs := DecodeTraits([]Def{{Class: TraitClass, Key: "p.bad", Plugin: "p", File: "defs/t.json", Body: body}, good})
		if len(probs) != 1 || probs[0].Field != c.field || !strings.Contains(probs[0].Msg, c.msg) {
			t.Errorf("%s: %v", name, probs)
			continue
		}
		if b.Len() != 1 || b.Key(1) != "p.good" {
			t.Errorf("%s: bad def kept or good one lost: %d traits", name, b.Len())
		}
	}
	if _, probs := DecodeTraits([]Def{{Class: "recipe", Key: "p.x", Plugin: "p", Body: map[string]any{}}}); len(probs) != 1 {
		t.Errorf("wrong class: %v", probs)
	}
}

// TestTraitCoreKeepsBehaviour: a patched core trait decodes with its plain
// fields changed and its behaviour (note, thoughts) the game's, and a patch
// cannot add behaviour.
func TestTraitCoreKeepsBehaviour(t *testing.T) {
	t.Parallel()
	defs := EncodeTraits(trait.Core())
	patch := func(id trait.ID, edit func(map[string]any)) (trait.Def, []Problem) {
		d := defs[id-1]
		d.Body = treeCopyMap(d.Body)
		edit(d.Body)
		b, probs := DecodeTraits([]Def{d})
		return b.DefOf(1), probs
	}
	d, probs := patch(trait.Cannibal, func(b map[string]any) { b["rarity"] = 9.0 })
	if len(probs) > 0 || d.Rarity != 9 || d.Note != trait.Core().DefOf(trait.Cannibal).Note {
		t.Fatalf("plain patch: %v %+v", probs, d)
	}
	d, probs = patch(trait.Roughneck, func(b map[string]any) { b["label"] = "Hardened" })
	if len(probs) > 0 || d.Label != "Hardened" || !reflect.DeepEqual(d.ThoughtScale, trait.Core().DefOf(trait.Roughneck).ThoughtScale) {
		t.Fatalf("thought scale kept: %v %+v", probs, d)
	}
	if _, probs = patch(trait.Sunny, func(b map[string]any) { b["note"] = "Never sleeps" }); len(probs) != 1 {
		t.Fatalf("note added to core: %v", probs)
	}
}

// TestTraitBookCap: past trait.MaxBook a trait is refused, not wrapped onto
// a byte ID another trait holds.
func TestTraitBookCap(t *testing.T) {
	t.Parallel()
	defs := EncodeTraits(trait.Core())
	for i := trait.Core().Len(); i <= trait.MaxBook; i++ {
		defs = append(defs, Def{Class: TraitClass, Key: "p.t" + strings.Repeat("x", i), Plugin: "p", Body: map[string]any{"label": "T"}})
	}
	b, probs := DecodeTraits(defs)
	if b.Len() != trait.MaxBook || len(probs) != 1 || !strings.Contains(probs[0].Msg, "too many traits") {
		t.Fatalf("len %d, problems %v", b.Len(), probs)
	}
}

// TestTraitInheritAndPatch runs the whole pipeline: a plugin trait
// inherits an abstract template, and a patch changes a core trait's number
// while a patch that breaks a core trait is refused.
func TestTraitInheritAndPatch(t *testing.T) {
	t.Parallel()
	defs := map[string]any{"trait": []any{
		map[string]any{"name": "template", "abstract": true, "group": "courage", "mods": []any{map[string]any{"stat": "mood_offset", "add": 2.0}}},
		map[string]any{"name": "bold", "parent": "template", "label": "Bold"},
		map[string]any{"name": "reckless", "parent": "template", "label": "Reckless", "mods": []any{map[string]any{"stat": "shoot_acc", "add": -4.0}}},
	}}
	ops := []any{
		map[string]any{"op": "set", "target": "trait/core.sunny", "path": "mods.0.add", "value": 11.0},
		map[string]any{"op": "set", "target": "trait/core.steady", "path": "label", "value": ""},
		map[string]any{"op": "remove", "target": "trait/core.fussy"},
	}
	cat, probs := Build([]Plugin{plugin("demo", defs, ops)}, []string{"demo"}, "0.1.0")
	core := trait.Core()
	if cat.Traits.Len() != core.Len()+2 {
		t.Fatalf("Len = %d, want core %d + 2", cat.Traits.Len(), core.Len())
	}
	bold, ok := cat.Traits.IDOf("demo.bold")
	rk, ok2 := cat.Traits.IDOf("demo.reckless")
	if !ok || !ok2 || bold != trait.NumTraits || rk != bold+1 {
		t.Fatalf("ids %d %d", bold, rk)
	}
	if d := cat.Traits.DefOf(bold); d.Group < trait.FirstPluginGroup || d.Mods[0].Stat != stat.MoodOffset || d.Mods[0].Add != 2 {
		t.Fatalf("template not inherited: %+v", d)
	}
	if d := cat.Traits.DefOf(rk); len(d.Mods) != 1 || d.Mods[0].Stat != stat.ShootAcc {
		t.Fatalf("child mods replace the parent's: %+v", d.Mods)
	}
	if !cat.Traits.Conflicts(bold, rk) {
		t.Fatal("the template's group is shared")
	}
	if cat.Traits.DefOf(trait.Sunny).Mods[0].Add != 11 || core.DefOf(trait.Sunny).Mods[0].Add != 8 {
		t.Fatal("patch did not change the core trait's number (or changed core)")
	}
	for id := trait.ID(1); int(id) <= core.Len(); id++ {
		if cat.Traits.Key(id) != core.Key(id) {
			t.Fatalf("core id %d moved", id)
		}
	}
	if cat.Traits.DefOf(trait.Steady).Label != "Steady" || cat.Traits.DefOf(trait.Fussy).Label != "Fussy" {
		t.Fatal("a broken or removed core trait was not kept")
	}
	var mine []Problem // other classes' problems are theirs to test
	for _, p := range probs {
		if strings.HasPrefix(p.Def, TraitClass+"/") {
			mine = append(mine, p)
		}
	}
	if len(mine) != 3 { // the bad patch (named for core and for demo) and the removal
		t.Fatalf("problems = %v", mine)
	}
}
