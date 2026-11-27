// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// tunedThoughts is core's thought book with the given kinds retuned.
func tunedThoughts(t testing.TB, edit map[mood.Kind]func(*mood.Tuning)) mood.Book {
	t.Helper()
	core := mood.Core()
	list := make([]mood.Tuning, core.Len())
	for i := range list {
		d := core.DefOf(mood.Kind(i))
		list[i] = mood.Tuning{Label: d.Label, Value: d.Value, Duration: d.Duration, StackLimit: d.StackLimit, StackFactor: d.StackFactor}
		if f := edit[mood.Kind(i)]; f != nil {
			f(&list[i])
		}
	}
	b, ok := mood.NewBook(list)
	if !ok {
		t.Fatal("NewBook refused a full list")
	}
	return b
}

func setValue(label string, v int) func(*mood.Tuning) {
	return func(tu *mood.Tuning) { tu.Value = v; tu.Label = label }
}

func memoryOf(g *Game, id entity.EntityID, k mood.Kind) (mood.Memory, bool) {
	for _, m := range g.memories[id] {
		if m.Kind == k {
			return m, true
		}
	}
	return mood.Memory{}, false
}

// C2: a patched def reaches play. The memory made through remember reads
// the patched value and line in the pawn's read model, and the thought
// level moves by the patch's difference.
func TestPatchedThoughtReachesPlay(t *testing.T) {
	t.Parallel()
	const label = "slept under the stars"
	patched := tunedThoughts(t, map[mood.Kind]func(*mood.Tuning){mood.SleptOutside: setValue(label, -2)})
	core := New(Config{Seed: 3, Width: 12, Height: 8})
	g := New(Config{Seed: 3, Width: 12, Height: 8, Content: Content{Thoughts: patched}})
	a, _ := core.SpawnNamedPawn(2, 2, "Ada")
	b, _ := g.SpawnNamedPawn(2, 2, "Ada")
	base := g.thoughtLevel(b)
	core.remember(a, mood.SleptOutside)
	g.remember(b, mood.SleptOutside)

	var got *ThoughtView
	pv := pawnViewOf(g, b)
	for i := range pv.Thoughts {
		if pv.Thoughts[i].Label == label {
			got = &pv.Thoughts[i]
		}
	}
	if got == nil || got.Value != -2 {
		t.Fatalf("read model thoughts %+v lack %q at -2", pv.Thoughts, label)
	}
	if want := core.thoughtLevel(a) + 2; g.thoughtLevel(b) != want {
		t.Fatalf("level %d, want core's %d plus 2 (base %d)", g.thoughtLevel(b), want, base)
	}
}

// C3: a patched state thought and a patched diet thought both read the book.
func TestPatchedStateAndDietThoughts(t *testing.T) {
	t.Parallel()
	patched := tunedThoughts(t, map[mood.Kind]func(*mood.Tuning){
		mood.Hungry: setValue("peckish", -2),
		mood.AteRaw: setValue("raw again", -3),
	})
	g := New(Config{Seed: 5, Width: 12, Height: 8, Content: Content{Thoughts: patched}})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.HungryAt, needs.Full)
	if v, ok := thoughtValue(g, id, mood.Hungry); !ok || v != -2 {
		t.Fatalf("hungry thought %d (present %v), want -2", v, ok)
	}
	raw := item.Stack{Kind: item.Potatoes, Qty: 1}
	if got := diet.AteMood(patched, raw, false); got != -3 {
		t.Fatalf("AteMood under the patched book = %d, want -3", got)
	}
	g.afterEat(id, raw, false)
	m, ok := memoryOf(g, id, mood.AteRaw)
	if !ok || m.Value != -3 {
		t.Fatalf("stored raw-food memory %+v (present %v)", m, ok)
	}
	var line mood.Thought
	for _, th := range g.thoughts(id) {
		if th.Kind == mood.AteRaw {
			line = th
		}
	}
	if line.Value != -3 || line.Label != "raw again" {
		t.Fatalf("raw-food thought %+v, want -3 %q", line, "raw again")
	}
}

// The food picker scores with the run's book, so a softened raw-food
// thought makes raw food more attractive.
func TestPatchedThoughtMovesFoodScore(t *testing.T) {
	t.Parallel()
	patched := tunedThoughts(t, map[mood.Kind]func(*mood.Tuning){mood.AteRaw: setValue("raw", 0)})
	core := New(Config{Seed: 5, Width: 12, Height: 8})
	g := New(Config{Seed: 5, Width: 12, Height: 8, Content: Content{Thoughts: patched}})
	a, _ := core.SpawnNamedPawn(2, 2, "Ada")
	b, _ := g.SpawnNamedPawn(2, 2, "Ada")
	st := &item.Stack{Kind: item.Potatoes, Qty: 1}
	cs := core.sk.diet.Picker(thoughtsOf(core), a, false, false).Score(st, 0)
	ps := g.sk.diet.Picker(thoughtsOf(g), b, false, false).Score(st, 0)
	if ps <= cs {
		t.Fatalf("patched score %d is not above core's %d", ps, cs)
	}
}

// C4: a duration patch sets the expiry of memories made after New; a memory
// saved under core keeps its saved expiry when loaded under a patched book.
func TestPatchedDuration(t *testing.T) {
	t.Parallel()
	const dur = 100
	patched := tunedThoughts(t, map[mood.Kind]func(*mood.Tuning){mood.SleptOutside: func(tu *mood.Tuning) { tu.Duration = dur }})
	g := New(Config{Seed: 7, Width: 12, Height: 8, Content: Content{Thoughts: patched}})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.remember(id, mood.SleptOutside)
	m, ok := memoryOf(g, id, mood.SleptOutside)
	if !ok || m.Expires != uint64(g.clock.Now())+dur {
		t.Fatalf("memory %+v at now %d, want expiry now+%d", m, g.clock.Now(), dur)
	}

	c := New(Config{Seed: 7, Width: 12, Height: 8})
	cid, _ := c.SpawnNamedPawn(2, 2, "Ada")
	c.remember(cid, mood.SleptOutside)
	saved, _ := memoryOf(c, cid, mood.SleptOutside)
	var buf bytes.Buffer
	if err := c.Save(&buf, "dur"); err != nil {
		t.Fatal(err)
	}
	l, _, err := LoadContent(bytes.NewReader(buf.Bytes()), Content{Thoughts: patched})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := memoryOf(l, cid, mood.SleptOutside); !ok || got.Expires != saved.Expires {
		t.Fatalf("loaded memory %+v, saved expiry %d", got, saved.Expires)
	}
}
