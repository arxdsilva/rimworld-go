// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"testing"

	"rimworld-go/internal/application/scripting"
)

// twoPlugins is plugin A (the state fixture, a file that never loads, a
// guest whose on_hour traps) and plugin B (dice, tally).
func twoPlugins(t *testing.T, o Options) *Host {
	t.Helper()
	trap := newGuest().hook("on_hour", []byte{0x00}).bytes()
	return newHost(t, o,
		Plugin{ID: "A", Files: []File{
			{Path: statePath, Wasm: readFixture(t, "state")},
			{Path: badPath, Wasm: []byte("not wasm")},
			{Path: trapPath, Wasm: trap},
		}},
		Plugin{ID: "B", Files: []File{
			{Path: "scripts/dice.wasm", Wasm: readFixture(t, "dice")},
			{Path: "scripts/tally.wasm", Wasm: readFixture(t, "tally")},
		}})
}

// joined is the views' Results in order, as one Result.
func joined(rs ...scripting.Result) scripting.Result {
	var out scripting.Result
	for _, r := range rs {
		out.Commands = append(out.Commands, r.Commands...)
		out.Disabled = append(out.Disabled, r.Disabled...)
	}
	return out
}

func sameResult(a, b scripting.Result) bool {
	if len(a.Commands) != len(b.Commands) || len(a.Disabled) != len(b.Disabled) {
		return false
	}
	for i := range a.Commands {
		if a.Commands[i] != b.Commands[i] {
			return false
		}
	}
	for i := range a.Disabled {
		if a.Disabled[i] != b.Disabled[i] {
			return false
		}
	}
	return true
}

// C4: Plugin(A).Fire then Plugin(B).Fire is Host.Fire: the same Commands
// and Disabled in the same order, the load failure in exactly one Result.
func TestPluginViewsMatchHostFire(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		split, whole := twoPlugins(t, o), twoPlugins(t, o)
		a, b := split.Plugin("A"), split.Plugin("B")
		loadFails := 0
		for hour := 0; hour < 4; hour++ {
			ev := event(scripting.OnHour)
			ev.Tick += uint64(hour) * 2500
			ra, rb := a.Fire(ev, fakeWorld{}, nil), b.Fire(ev, fakeWorld{}, nil)
			for _, d := range append(append([]scripting.Problem(nil), ra.Disabled...), rb.Disabled...) {
				if d.File == badPath {
					loadFails++
				}
			}
			got, want := joined(ra, rb), whole.Fire(ev, fakeWorld{}, nil)
			if !sameResult(got, want) || len(want.Commands) == 0 {
				t.Errorf("hour %d: views %v %+v, host %v %+v", hour, messages(got.Commands), got.Disabled, messages(want.Commands), want.Disabled)
			}
		}
		if loadFails != 1 {
			t.Errorf("the load failure is in %d Results, want 1", loadFails)
		}
		if a.Uses(scripting.OnPawn) || !b.Uses(scripting.OnPawn) || !a.Uses(scripting.OnHour) {
			t.Error("Uses does not cover only the view's files")
		}
	})
}

// A view stays valid across Reset, an unknown id is a view with no files,
// and any view's first Result carries the load failures.
func TestPluginViewEdges(t *testing.T) {
	h := twoPlugins(t, Options{Engine: Interpreter})
	none := h.Plugin("nope")
	if none.Uses(scripting.OnHour) {
		t.Error("an unknown plugin uses on_hour")
	}
	res := none.Fire(event(scripting.OnHour), fakeWorld{}, nil)
	if len(res.Commands) != 0 || len(res.Disabled) != 1 || res.Disabled[0].File != badPath {
		t.Errorf("unknown view's first Fire = %+v", res)
	}
	a := h.Plugin("A")
	hours(h, 2)
	h.Reset()
	if got := messages(a.Fire(event(scripting.OnHour), fakeWorld{}, nil).Commands); len(got) != 1 || got[0] != "count 1" {
		t.Errorf("view after Reset = %v", got)
	}
	if r := a.Fire(scripting.Event{Hook: scripting.OnPawn, Tick: 1}, fakeWorld{}, nil); len(r.Commands)+len(r.Disabled) != 0 {
		t.Errorf("on_pawn with nobody due = %+v", r)
	}
}

// C5: a view's Uses and StatsInto with enough room do not allocate.
func TestViewUsesAndStatsIntoDoNotAllocate(t *testing.T) {
	h := twoPlugins(t, Options{Engine: Interpreter})
	v := h.Plugin("B")
	if n := testing.AllocsPerRun(100, func() { _ = v.Uses(scripting.OnPawn) }); n != 0 {
		t.Errorf("view Uses: %v allocs", n)
	}
	buf := make([]Stat, 0, 8)
	if n := testing.AllocsPerRun(100, func() { buf = h.StatsInto(buf) }); n != 0 {
		t.Errorf("StatsInto: %v allocs", n)
	}
	if len(buf) != 4 || !buf[0].State || buf[2].State || !buf[3].State || buf[3].File != "scripts/tally.wasm" {
		t.Errorf("StatsInto = %+v", buf)
	}
}
