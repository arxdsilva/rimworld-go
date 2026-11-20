// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
)

type exampleWorld struct{ hour int }

func (w exampleWorld) Day() int                     { return 4 }
func (w exampleWorld) Hour() int                    { return w.hour }
func (w exampleWorld) Season() string               { return "summer" }
func (w exampleWorld) Colonists() int               { return 1 }
func (w exampleWorld) Wealth() int                  { return 0 }
func (w exampleWorld) ItemCount(string) (int, bool) { return 0, false }

type examplePawns []scripting.Pawn

func (e examplePawns) Pawns() []scripting.Pawn { return e }
func (e examplePawns) Pawn(id uint64) (scripting.Pawn, bool) {
	for _, p := range e {
		if p.ID == id {
			return p, true
		}
	}
	return scripting.Pawn{}, false
}

// exampleFrontier loads the shipped example plugin's scripts from disk.
func exampleFrontier(t *testing.T) *Host {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "mods", "example_frontier")
	paths, err := filepath.Glob(filepath.Join(dir, "scripts", "*.star"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no example scripts under %s: %v", dir, err)
	}
	sort.Strings(paths)
	pl := Plugin{ID: "example_frontier"}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		pl.Files = append(pl.Files, File{Path: "scripts/" + filepath.Base(p), Src: src})
	}
	h := New([]Plugin{pl})
	if ps := h.Problems(); len(ps) != 0 {
		t.Fatalf("example_frontier has problems: %+v", ps)
	}
	return h
}

func TestExampleFrontierOnPawn(t *testing.T) {
	t.Parallel()
	h := exampleFrontier(t)
	hungry := scripting.Pawn{ID: 5, Name: "Ada", Food: 10}
	fire := func(hour int, tick uint64, p scripting.Pawn) []string {
		ev := scripting.Event{Hook: scripting.OnPawn, Tick: tick, Seed: 1, Pawns: []scripting.Pawn{p}}
		return msgs(h.Fire(ev, exampleWorld{hour: hour}, examplePawns{p}))
	}
	got := fire(12, 12*2500+7, hungry)
	if len(got) != 1 || !strings.Contains(got[0], "Ada") {
		t.Fatalf("hour 12: got %v, want one message naming Ada", got)
	}
	if got := fire(11, 11*2500+7, hungry); len(got) != 0 {
		t.Fatalf("hour 11: got %v, want none", got)
	}
	if got := fire(12, 12*2500+60, hungry); len(got) != 0 {
		t.Fatalf("after the first 60 ticks: got %v, want none", got)
	}
	fed := hungry
	fed.Food = 50
	if got := fire(12, 12*2500+7, fed); len(got) != 0 {
		t.Fatalf("fed colonist: got %v, want none", got)
	}
	down := hungry
	down.Downed = true
	if got := fire(12, 12*2500+7, down); len(got) != 0 {
		t.Fatalf("downed colonist: got %v, want none", got)
	}
}
