// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
)

// traitBookWith is core's traits plus one plugin trait per key, each
// carrying its key's index as a work-speed bonus so they tell apart.
func traitBookWith(keys ...string) trait.Book {
	core := trait.Core()
	list, all := core.All(), core.Keys()
	for i, k := range keys {
		list = append(list, trait.Def{Key: k, Label: "Plugin " + k, Desc: "{name} is " + k + ".",
			Mods:       []stat.Mod{{Stat: stat.MoveSpeed, Mul: 1500}},
			SkillBonus: map[skill.ID]int8{skill.Melee: int8(i + 1)}})
		all = append(all, k)
	}
	return trait.NewBook(list, all)
}

// A plugin trait is drawn by generation, its stat mod applies to the
// colonist, and the Bio tab lists it with its label and effects (PLG-026).
func TestPluginTraitIsGeneratedAndApplies(t *testing.T) {
	book := traitBookWith("a.swift")
	id, _ := book.IDOf("a.swift")
	rules := pawngen.Rules{Traits: book}
	var found pawngen.Pawn
	for seed := uint64(1); seed < 2000; seed++ {
		if p := pawngen.Generate(seed, 0, 0, rules); slices.Contains(p.Traits, id) {
			found = p
			break
		}
	}
	if found.First == "" {
		t.Fatal("no pawn rolled the plugin trait in 2000 seeds")
	}
	for seed := uint64(1); seed < 300; seed++ {
		if p := pawngen.Generate(seed, 0, 0, pawngen.Rules{Traits: book, Legacy: true}); slices.Contains(p.Traits, id) {
			t.Fatalf("legacy rules rolled the plugin trait (seed %d)", seed)
		}
	}
	found.Traits = []trait.ID{id}
	g := New(Config{Width: 12, Height: 12, Content: Content{Traits: book}})
	pid, err := g.spawnGenerated(found, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	// Move speed is the plugin trait's 1.5x on top of body and age.
	want := pawngen.Stats(found, book)[stat.MoveSpeed]
	if got := g.statOf(pid, stat.MoveSpeed); got != want || want <= pawngen.Stats(found, trait.Core())[stat.MoveSpeed] {
		t.Fatalf("move speed %d, want %d (core book reads %d)", got, want, pawngen.Stats(found, trait.Core())[stat.MoveSpeed])
	}
	v, ok := g.bioView(pid)
	if !ok || len(v.Traits) != 1 || v.Traits[0].Label != "Plugin a.swift" || v.Traits[0].Effects == "" || v.Traits[0].Desc == "" {
		t.Fatalf("bio traits %+v", v.Traits)
	}
}

// A save keeps its plugin traits by key (PLG-044, PLG-026): loaded where the
// plugin set moved them they keep their meaning; loaded where the plugin is
// gone they are taken off the pawn, never turned into another trait.
func TestSavedTraitsFollowTheirKeys(t *testing.T) {
	saved := traitBookWith("a.first", "b.second")
	g := New(Config{Width: 12, Height: 12, Content: Content{Traits: saved}})
	second, _ := saved.IDOf("b.second")
	first, _ := saved.IDOf("a.first")
	id, err := g.spawnGenerated(pawngen.Pawn{First: "Ida", Traits: []trait.ID{trait.Sunny, second, first, trait.Sturdy}}, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "traits"); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	for _, tc := range []struct {
		name string
		book trait.Book
		want []string
	}{
		{"same set", saved, []string{"core.sunny", "b.second", "a.first", "core.sturdy"}},
		{"reordered", traitBookWith("b.second", "a.first"), []string{"core.sunny", "b.second", "a.first", "core.sturdy"}},
		{"first plugin gone", traitBookWith("b.second"), []string{"core.sunny", "b.second", "core.sturdy"}},
		{"core only", trait.Core(), []string{"core.sunny", "core.sturdy"}},
	} {
		l, _, err := LoadContent(bytes.NewReader(data), Content{Traits: tc.book})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		var got []string
		for _, tr := range l.sk.bios[id].Traits {
			got = append(got, tc.book.Key(tr))
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: traits %v, want %v", tc.name, got, tc.want)
		}
	}
}
