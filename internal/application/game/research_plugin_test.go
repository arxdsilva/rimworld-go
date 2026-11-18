// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// researchBookWith is core's projects plus, per plugin key, a Medieval
// project of cost 300 that needs Traps and unlocks the recipe named as the
// key with ".mat" appended (a plugin recipe's name is its key).
func researchBookWith(keys ...string) research.Book {
	core := research.Core()
	list, all := slices.Clone(core.All()), core.Keys()
	for _, k := range keys {
		list = append(list, research.Def{Name: research.NameOf(k), Label: "Plugin " + k, Cost: 300, TechLevel: research.Medieval,
			Requires: []research.Project{research.Traps}, Unlocks: []string{"recipe:" + k + ".mat"}})
		all = append(all, k)
	}
	return research.NewBook(list, all)
}

// TestPluginProjectUnlocksPluginRecipe (PLG-026): a plugin recipe gated by
// a plugin project cannot be billed, and the Research tab's read model
// lists the project and the lock, until the project is picked through the
// tab's command and a colonist researches it at a bench (its prerequisite
// finished by the tab's developer command); then the bill goes in.
func TestPluginProjectUnlocksPluginRecipe(t *testing.T) {
	t.Parallel()
	recipes := recipeBookWith("frontier.weave.mat")
	mat, _ := recipes.IDOf("frontier.weave.mat")
	book := researchBookWith("frontier.weave")
	weave, _ := book.IDOf("frontier.weave")
	g := New(Config{Seed: 1, Width: 20, Height: 12, Content: Content{Recipes: recipes, Research: book}})
	g.ResetResearch() // a fresh colony's tree
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	if !g.BuildNow("research bench", "wood", "south", 4, 7) {
		t.Fatal("build research bench")
	}
	ada, _ := g.SpawnNamedPawn(12, 8, "Ada")
	bid := g.layerBldgAt(5, 2, construction.LayerObject)
	addBill := func() {
		g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(mat), Mode: uint8(recipe.Forever), Count: 1})
		g.Step(1)
	}
	addBill()
	w := g.World()
	if len(g.craft.Bills.Of(bid)) != 0 || w.Locked["recipe:frontier.weave.mat"] != "Plugin frontier.weave" {
		t.Fatalf("locked: bills %v, lock %q", billKeys(g, recipes, bid), w.Locked["recipe:frontier.weave.mat"])
	}
	i := slices.IndexFunc(w.Research, func(v ResearchView) bool { return v.ID == uint8(weave) })
	if i < 0 || w.Research[i].TechLevel != "Medieval" || w.Research[i].Available || !slices.Equal(w.Research[i].Requires, []string{"Traps"}) {
		t.Fatalf("the tab lists the plugin project in its tier with its prerequisite: %+v", w.Research)
	}
	pick := func(p research.Project) {
		g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(p)})
		g.Enqueue(command.Command{Kind: command.DevFinishResearch, Build: uint8(p)})
		g.Step(1)
	}
	pick(weave) // not open yet: ignored
	if g.lab.Done(weave) {
		t.Fatal("researched before its prerequisite")
	}
	pick(research.Traps)
	g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(weave)})
	g.Step(1)
	g.lab.Prog[weave] = book.DefOf(weave).Cost - 5 // the last points at the bench
	studied := false
	for i := 0; i < 400 && !g.lab.Done(weave); i++ {
		g.Step(50)
		g.SetNeeds(ada, 1<<20, 1<<20)
		if j, _ := g.jobs.Get(ada); j.Kind.String() == "research" {
			studied = true
		}
	}
	if !studied || g.lab.Set {
		t.Fatalf("Ada researches the plugin project at the bench (studied %v, progress %d)", studied, g.lab.Prog[weave])
	}
	addBill()
	if got := billKeys(g, recipes, bid); !g.lab.Done(weave) || !slices.Equal(got, []string{"frontier.weave.mat"}) {
		t.Fatalf("researched %v: bills %v", g.lab.Done(weave), got)
	}
	if w := g.World(); w.Locked["recipe:frontier.weave.mat"] != "" || !w.Research[weave].Done {
		t.Fatal("the read model shows it researched and the recipe open")
	}
}

// TestSavedResearchFollowsKeys (PLG-026): a save keeps plugin research by
// key: progress and the current project move where the plugin set moved
// the project; a gone plugin's points and selection are dropped; core's
// stay put.
func TestSavedResearchFollowsKeys(t *testing.T) {
	t.Parallel()
	saved := researchBookWith("a.first", "b.second")
	g := New(Config{Width: 12, Height: 12, Content: Content{Research: saved}})
	g.ResetResearch()
	first, _ := saved.IDOf("a.first")
	second, _ := saved.IDOf("b.second")
	g.AddResearch("traps", research.DefOf(research.Traps).Cost)
	g.AddResearch("a.first", 40)
	g.AddResearch("b.second", 70)
	g.PickResearch("b.second")
	g.Enqueue(command.Command{Kind: command.SetResearch, Build: uint8(first)}) // still pending in the save
	if g.lab.Prog[first] != 40 || g.lab.Prog[second] != 70 || !g.lab.Set || g.lab.Cur != second {
		t.Fatalf("setup: %d %d cur %d", g.lab.Prog[first], g.lab.Prog[second], g.lab.Cur)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "research"); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	for _, tc := range []struct {
		name          string
		book          research.Book
		first, second int  // points on each key in the loaded run (-1: absent)
		cur           bool // b.second still the current project before the pending pick
	}{
		{"same set", saved, 40, 70, true},
		{"reordered", researchBookWith("b.second", "a.first"), 40, 70, true},
		{"first gone", researchBookWith("b.second"), -1, 70, true},
		{"second gone", researchBookWith("a.first"), 40, -1, false},
		{"core only", research.Core(), -1, -1, false},
	} {
		l, _, err := LoadContent(bytes.NewReader(data), Content{Research: tc.book})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !l.lab.Done(research.Traps) {
			t.Errorf("%s: core research lost", tc.name)
		}
		for key, want := range map[string]int{"a.first": tc.first, "b.second": tc.second} {
			p, ok := tc.book.IDOf(key)
			if ok != (want >= 0) || ok && l.lab.Prog[p] != want {
				t.Errorf("%s: %s at %d (in book %v), want %d", tc.name, key, l.lab.Prog[p], ok, want)
			}
		}
		for p := int(research.NumProjects); p < research.MaxBook; p++ {
			if _, in := tc.book.IDOf(tc.book.Key(research.Project(p))); !in && l.lab.Prog[p] != 0 {
				t.Errorf("%s: points left on slot %d past the book", tc.name, p)
			}
		}
		sec, _ := tc.book.IDOf("b.second")
		if got := l.lab.Set && l.lab.Cur == sec; got != tc.cur {
			t.Errorf("%s: current b.second %v, want %v", tc.name, got, tc.cur)
		}
		l.Step(1) // the pending pick of a.first: where a.first went, or dropped
		fst, ok := tc.book.IDOf("a.first")
		if picked := l.lab.Set && l.lab.Cur == fst; picked != ok {
			t.Errorf("%s: pending pick of a.first %v, want %v", tc.name, picked, ok)
		}
	}
}
