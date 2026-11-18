// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package research

import (
	"reflect"
	"testing"
)

// bookWith is core plus one plugin project keyed key that needs Traps and
// unlocks recipe name.
func bookWith(key, recipe string) Book {
	c := Core()
	list, keys := append([]Def(nil), c.All()...), c.Keys()
	list = append(list, Def{Name: NameOf(key), Label: "Plugin", Cost: 300, TechLevel: Medieval,
		Requires: []Project{Traps}, Unlocks: []string{"recipe:" + recipe}})
	return NewBook(list, append(keys, key))
}

// The zero Book is core; core's IDs, keys and package helpers agree.
func TestBookCoreIsTheTable(t *testing.T) {
	t.Parallel()
	var zero Book
	if zero.Len() != int(NumProjects) || Core().Len() != int(NumProjects) {
		t.Fatalf("core Len %d, want %d", zero.Len(), NumProjects)
	}
	if !reflect.DeepEqual(zero.All(), All()) {
		t.Fatal("the zero book lists the table")
	}
	if k := zero.Key(Traps); k != "core.traps" {
		t.Fatalf("key %q", k)
	}
	if p, ok := zero.IDOf("core.traps"); !ok || p != Traps {
		t.Fatal("IDOf core.traps")
	}
	if _, ok := zero.IDOf("traps"); ok {
		t.Fatal("IDOf takes keys, not names")
	}
	if NameOf("core.traps") != "traps" || NameOf("frontier.weave") != "frontier.weave" {
		t.Fatal("NameOf")
	}
	if zero.Complete() != Complete() {
		t.Fatal("Complete")
	}
}

// A plugin project follows core's, gates its unlock until done, and needs
// its prerequisites; core's numbers in the book win over the table's.
func TestBookPluginProject(t *testing.T) {
	t.Parallel()
	b := bookWith("frontier.weave", "frontier.mat")
	p, ok := b.IDOf("frontier.weave")
	if !ok || p != NumProjects || b.Len() != int(NumProjects)+1 || b.DefOf(p).Project != p {
		t.Fatalf("plugin project at %d (ok %v), Len %d", p, ok, b.Len())
	}
	if q, ok := b.Parse("frontier.weave"); !ok || q != p {
		t.Fatal("Parse by name (a plugin's name is its key)")
	}
	var pr Progress
	if b.Available(&pr, p) {
		t.Fatal("needs traps first")
	}
	if g, ok := b.Locks(&pr, "recipe:frontier.mat"); !ok || g != p {
		t.Fatal("the plugin project gates its recipe")
	}
	b.Add(&pr, Traps, b.DefOf(Traps).Cost)
	if !b.Available(&pr, p) || b.Add(&pr, p, 299) || !b.Add(&pr, p, 5) || pr[p] != 300 {
		t.Fatalf("researching the plugin project: %d", pr[p])
	}
	if _, ok := b.Locks(&pr, "recipe:frontier.mat"); ok {
		t.Fatal("researched: the recipe is open")
	}
	if c := b.Complete(); !b.Done(&c, p) {
		t.Fatal("the book's Complete covers plugin projects")
	}
	if Core().Done(&pr, p) || Core().Available(&pr, p) {
		t.Fatal("core knows no project past its own")
	}
	all := b.All()
	_ = append(all, Def{})
	if b.DefOf(p).Label != "Plugin" || len(b.All()) != b.Len() {
		t.Fatal("All is clipped: an append never writes into the book")
	}
}

// A patched core cost is what Done reads.
func TestBookPatchedCost(t *testing.T) {
	t.Parallel()
	list := Core().All()
	list = append([]Def(nil), list...)
	list[Traps].Cost = 50
	b := NewBook(list, Core().Keys())
	var pr Progress
	if !b.Add(&pr, Traps, 50) || !b.Done(&pr, Traps) || pr.Done(Traps) {
		t.Fatal("the book's cost decides, not the table's")
	}
}

// Slice keeps a core-only save as it was and carries plugin points.
func TestProgressSliceTrims(t *testing.T) {
	t.Parallel()
	var pr Progress
	if n := len(pr.Slice()); n != int(NumProjects) {
		t.Fatalf("core-only slice %d, want %d", n, NumProjects)
	}
	pr[NumProjects+2] = 7
	s := pr.Slice()
	if len(s) != int(NumProjects)+3 || FromSlice(s) != pr {
		t.Fatalf("plugin points round trip: len %d", len(s))
	}
	pr[MaxBook-1] = 1
	if FromSlice(pr.Slice()) != pr || FromSlice(make([]int, MaxBook+5)) != (Progress{}) {
		t.Fatal("the last slot round trips; extra entries are ignored")
	}
}

func TestTierKeys(t *testing.T) {
	t.Parallel()
	for l := TechLevel(0); l < NumTechLevels; l++ {
		if got, ok := ParseTechLevel(l.Key()); !ok || got != l {
			t.Fatalf("tech %v", l)
		}
	}
	for b := BenchTier(0); b < NumBenchTiers; b++ {
		if got, ok := ParseBenchTier(b.Key()); !ok || got != b {
			t.Fatalf("bench %v", b)
		}
	}
	if _, ok := ParseTechLevel("Neolithic"); ok {
		t.Fatal("keys are lower case")
	}
}
