// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
)

const (
	trapPath = "scripts/trap.wasm"
	badPath  = "scripts/bad.wasm"
)

// lifecycleHost is the state fixture (p), a guest whose on_hour traps (q)
// and a file that never loads (r).
func lifecycleHost(t *testing.T, o Options) *Host {
	t.Helper()
	return newHost(t, o,
		fixture(t, "p", "state"),
		plug("q", trapPath, newGuest().hook("on_hour", []byte{0x00}).bytes()),
		plug("r", badPath, []byte("not wasm")))
}

func problemFiles(ps []scripting.Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Plugin+"/"+p.File)
	}
	return out
}

// C1: Reset gives every file a fresh instance (state as a fresh host's,
// zero counters, a trapped file on again), keeps only the load-time
// Problems and compiles nothing.
func TestResetGivesFreshInstances(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		fresh, err := newHost(t, o, fixture(t, "p", "state")).Save("p", statePath)
		if err != nil {
			t.Fatal(err)
		}
		h := lifecycleHost(t, o)
		hours(h, 3)
		if st := h.Stats(); !st[1].Off || st[0].Calls == 0 {
			t.Fatalf("before Reset: %+v", st)
		}
		if got, _ := h.Save("p", statePath); reflect.DeepEqual(got, fresh) {
			t.Fatal("the run did not change the state")
		}
		compiles := h.compiles
		h.Reset()
		if h.compiles != compiles {
			t.Errorf("Reset compiled %d modules", h.compiles-compiles)
		}
		for _, st := range h.Stats() {
			if st.Off || st.Fuel != 0 || st.Calls != 0 {
				t.Errorf("after Reset: %+v", st)
			}
		}
		if got := problemFiles(h.Problems()); !reflect.DeepEqual(got, []string{"r/" + badPath}) {
			t.Errorf("Problems after Reset = %v, want only the load failure", got)
		}
		got, err := h.Save("p", statePath)
		if err != nil || !reflect.DeepEqual(got, fresh) {
			t.Errorf("blob after Reset = %x, %v; a fresh host's = %x", got, err, fresh)
		}
		res := fire(h, event(scripting.OnHour))
		if m := messages(res.Commands); !reflect.DeepEqual(m, []string{"count 1"}) {
			t.Errorf("first hour after Reset = %v", m)
		}
		if got := problemFiles(res.Disabled); !reflect.DeepEqual(got, []string{"q/" + trapPath}) {
			t.Errorf("Disabled after Reset = %v: the trap again, the load failure already sent", got)
		}
	})
}

// The load failures go out in one Result per host: Reset before the first
// Fire keeps them queued, Reset after it does not queue them again, and a
// previous game's runtime problems are dropped either way.
func TestResetKeepsUnsentLoadFailuresOnly(t *testing.T) {
	o := Options{Engine: Interpreter}
	trapLoad := plug("s", "scripts/x.wasm", stateGuest(i32c(0), []byte{0x00, 0x41, 0x00}))
	badLoad := []byte("RWMS\x01\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00")

	h := newHost(t, o, trapLoad, plug("r", badPath, []byte("not wasm")))
	if err := h.Load("s", "scripts/x.wasm", badLoad); err == nil {
		t.Fatal("load did not trap")
	}
	h.Reset()
	if got := problemFiles(fire(h, event(scripting.OnHour)).Disabled); !reflect.DeepEqual(got, []string{"r/" + badPath}) {
		t.Errorf("Reset before the first Fire: Disabled = %v, want the load failure only", got)
	}

	h = newHost(t, o, trapLoad, plug("r", badPath, []byte("not wasm")))
	fire(h, event(scripting.OnHour))
	_ = h.Load("s", "scripts/x.wasm", badLoad)
	h.Reset()
	if res := fire(h, event(scripting.OnHour)); res.Disabled != nil {
		t.Errorf("Reset after the first Fire: Disabled = %+v, want none", res.Disabled)
	}
	if got := problemFiles(h.Problems()); !reflect.DeepEqual(got, []string{"r/" + badPath}) {
		t.Errorf("Problems = %v", got)
	}
}

// A file whose instance cannot be made again turns off with a Problem;
// Stats, Save, Fire and the next Reset still work.
func TestResetFailureTurnsTheFileOff(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, fixture(t, "p", "state"), fixture(t, "q", "dice"))
	_ = h.mods[0].cm.Close(h.ctx)
	h.Reset()
	st := h.Stats()
	if !st[0].Off || st[0].Pages != 0 || st[1].Off {
		t.Errorf("Stats = %+v", st)
	}
	ps := h.Problems()
	if len(ps) != 1 || ps[0].Plugin != "p" || !strings.HasPrefix(ps[0].Msg, "instantiate: ") {
		t.Errorf("Problems = %+v", ps)
	}
	if _, err := h.Save("p", statePath); err == nil || !strings.Contains(err.Error(), "is off") {
		t.Errorf("Save = %v", err)
	}
	res := fire(h, event(scripting.OnHour))
	if len(res.Disabled) != 1 || res.Disabled[0].Plugin != "p" || len(res.Commands) == 0 {
		t.Errorf("Fire = %+v", res)
	}
	h.Reset()
	if ps := h.Problems(); len(ps) != 1 || !h.Stats()[0].Off {
		t.Errorf("second Reset: Problems = %+v", ps)
	}
}

// C10: tally's counter survives save, Reset and load: after 5 hours the
// next spawning hour drops 6 wood, where a reset host without the load
// drops nothing at its first hour and 2 at its second.
func TestTallyStateRoundTrip(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		const path = "scripts/tally.wasm"
		h := newHost(t, o, fixture(t, "p", "tally"))
		var spawned []int
		for i := 0; i < 5; i++ {
			spawned = append(spawned, counts(fire(h, event(scripting.OnHour)))...)
		}
		if want := []int{2, 4}; !reflect.DeepEqual(spawned, want) {
			t.Fatalf("5 hours spawned %v, want %v", spawned, want)
		}
		blob, err := h.Save("p", path)
		if err != nil {
			t.Fatal(err)
		}
		h.Reset()
		if got := counts(fire(h, event(scripting.OnHour))); got != nil {
			t.Errorf("reset host's first hour spawned %v", got)
		}
		if got := counts(fire(h, event(scripting.OnHour))); !reflect.DeepEqual(got, []int{2}) {
			t.Errorf("reset host's second hour spawned %v, want [2]", got)
		}
		h.Reset()
		if err := h.Load("p", path, blob); err != nil {
			t.Fatal(err)
		}
		if got := counts(fire(h, event(scripting.OnHour))); !reflect.DeepEqual(got, []int{6}) {
			t.Errorf("restored host's next hour spawned %v, want [6]", got)
		}
	})
}
