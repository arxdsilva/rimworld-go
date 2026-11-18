// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package research

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/recipe"
	proj "rimworld-go/internal/domain/research"
)

// pluginBook is core's projects, Traps' cost patched to 100, plus
// "frontier.weave" (Medieval, 200, needs Traps) unlocking the recipe
// "frontier.mat".
func pluginBook() proj.Book {
	c := proj.Core()
	list := slices.Clone(c.All())
	list[proj.Traps].Cost = 100
	list = append(list, proj.Def{Name: "frontier.weave", Label: "Weaving", Desc: "Mats.", Cost: 200,
		TechLevel: proj.Medieval, Requires: []proj.Project{proj.Traps}, Unlocks: []string{"recipe:frontier.mat"}})
	return proj.NewBook(list, append(c.Keys(), "frontier.weave"))
}

// TestPluginProjectInTheLab (PLG-026): a plugin project lists in its tier
// with its prerequisite, gates its recipe, is picked and finished like a
// core one; a patched core cost is what the lab charges.
func TestPluginProjectInTheLab(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.book = pluginBook()
	f.add(construction.ResearchBench, 0, 0)
	l := New(f)
	l.Recipes = recipe.NewBook(append(recipe.Core().All(), recipe.Def{Name: "frontier.mat", Label: "Woven mat"}),
		append(recipe.Core().Keys(), "frontier.mat"))
	weave := proj.NumProjects
	views, locked := l.Lists(false)
	if len(views) != int(proj.NumProjects)+1 {
		t.Fatalf("%d views, want core + 1", len(views))
	}
	v := views[weave]
	if v.ID != uint8(weave) || v.Label != "Weaving" || v.TechLevel != "Medieval" || v.Available ||
		!slices.Equal(v.Requires, []string{"Traps"}) || !slices.Equal(v.Prereqs, []int{int(proj.Traps)}) ||
		!slices.Equal(v.Unlocks, []string{"Woven mat"}) {
		t.Fatalf("plugin view %+v", v)
	}
	if locked["recipe:frontier.mat"] != "Weaving" || l.LockedBy("recipe:frontier.mat") != "Weaving" {
		t.Fatal("the plugin project gates its recipe")
	}
	if views[proj.Traps].Cost != 100 {
		t.Fatalf("patched cost %d", views[proj.Traps].Cost)
	}
	l.Pick(weave)
	if l.Working() {
		t.Fatal("picked before its prerequisite")
	}
	l.Pick(proj.Traps)
	l.DevFinish()
	if !l.Done(proj.Traps) || l.Prog[proj.Traps] != 100 {
		t.Fatalf("traps at the patched cost: %d", l.Prog[proj.Traps])
	}
	l.Pick(weave)
	if !l.Working() {
		t.Fatal("the plugin project can be picked once traps is done")
	}
	if l.Alert() {
		t.Fatal("no alert while a project is under way")
	}
	l.DevFinish()
	if !l.Done(weave) || l.LockedBy("recipe:frontier.mat") != "" || l.Working() {
		t.Fatal("finishing the plugin project opens its recipe")
	}
	if got := f.letters[len(f.letters)-1]; got != "Research complete: Weaving" {
		t.Fatalf("letter %q", got)
	}
	l.Complete()
	if !l.Done(weave) {
		t.Fatal("Complete covers plugin projects")
	}
	l.Reset()
	l.Grant(weave)
	if !l.Done(weave) || !l.Done(proj.Traps) {
		t.Fatal("Grant finishes a plugin project and what it needs")
	}
}

// The run's book costs no allocation on the per-tick and bill-discovery
// paths (Working, LockedBy).
func TestPluginBookDoesNotAllocate(t *testing.T) {
	f := newFake()
	f.book = pluginBook()
	l := New(f)
	l.Pick(proj.Traps)
	if n := testing.AllocsPerRun(100, func() {
		_ = l.Working()
		_ = l.LockedBy("recipe:frontier.mat")
		_ = l.LockedBy("build:spike trap")
	}); n != 0 {
		t.Fatalf("%v allocations a run", n)
	}
}
