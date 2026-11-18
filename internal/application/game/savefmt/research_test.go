// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// projectBook is core's projects with the plugin keys after them.
func projectBook(keys ...string) research.Book {
	core := research.Core()
	list, ks := slices.Clone(core.All()), core.Keys()
	for _, k := range keys {
		list, ks = append(list, research.Def{Name: k, Label: k, Cost: 100}), append(ks, k)
	}
	return research.NewBook(list, ks)
}

// Saved projects follow their keys: core's stay, a moved plugin project
// moves with its points, a gone one's points and selection are dropped.
func TestResearchFollowsKeys(t *testing.T) {
	t.Parallel()
	n := int(research.NumProjects)
	keys := Keys{research.Class: projectBook("p.a", "p.b").Keys()}
	pts := make([]int, n+2)
	pts[research.Traps], pts[n], pts[n+1] = 5, 7, 9 // traps, p.a, p.b

	swapped := keys.Research(projectBook("p.b", "p.a"))
	got := swapped.Progress(pts)
	if got[research.Traps] != 5 || got[n] != 9 || got[n+1] != 7 {
		t.Fatalf("swapped progress %v", got[n-1:])
	}
	if p, set := swapped.Current(research.Project(n), true); p != research.Project(n+1) || !set {
		t.Fatalf("current p.a moves to %d (%v)", p, set)
	}

	onlyB := keys.Research(projectBook("p.b"))
	got = onlyB.Progress(pts)
	if len(got) != n+1 || got[n] != 9 || got[research.Traps] != 5 {
		t.Fatalf("p.a's points are dropped, p.b moves down: %v", got[n-1:])
	}
	if p, set := onlyB.Current(research.Project(n), true); p != 0 || set {
		t.Fatal("a gone current project is let go")
	}
	if p, set := onlyB.Current(research.Traps, true); p != research.Traps || !set {
		t.Fatal("a core current project stays")
	}

	core := keys.Research(research.Core())
	if got := core.Progress(make([]int, n+2)); got == nil || len(got) != 0 {
		t.Fatalf("all zero: an empty, non-nil list (%v)", got)
	}
	if got := core.Progress(pts); len(got) != int(research.Traps)+1 || got[research.Traps] != 5 {
		t.Fatalf("no plugin: core's points only: %v", got)
	}
}

// A save without research keys (before PLG-026) keeps its projects.
func TestResearchWithoutKeysKeeps(t *testing.T) {
	t.Parallel()
	m := Keys{}.Research(projectBook("p.a"))
	pts := []int{1, 2, 3}
	if got := m.Progress(pts); !slices.Equal(got, pts) {
		t.Fatalf("progress %v", got)
	}
	if p, set := m.Current(research.Herbalism, true); p != research.Herbalism || !set {
		t.Fatal("current kept")
	}
}

// Pending SetResearch commands follow their project; a gone one is
// dropped, other commands are left alone.
func TestResearchCommandsFollowKeys(t *testing.T) {
	t.Parallel()
	n := uint8(research.NumProjects)
	m := Keys{research.Class: projectBook("p.a", "p.b").Keys()}.Research(projectBook("p.b"))
	cmds := []command.Command{
		{Kind: command.SetResearch, Build: uint8(research.Traps)},
		{Kind: command.SetResearch, Build: n},
		{Kind: command.DesignateBuild, Build: n},
		{Kind: command.SetResearch, Build: n + 1},
	}
	got := m.Commands(cmds)
	want := []command.Command{
		{Kind: command.SetResearch, Build: uint8(research.Traps)},
		{Kind: command.DesignateBuild, Build: n},
		{Kind: command.SetResearch, Build: n},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("commands %+v", got)
	}
	same := []command.Command{{Kind: command.SetResearch, Build: uint8(research.Traps)}}
	if got := m.Commands(same); &got[0] != &same[0] {
		t.Fatal("nothing dropped: the same slice")
	}
}
