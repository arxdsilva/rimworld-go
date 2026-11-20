// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"fmt"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
	"rimworld-go/internal/application/scripting"
)

// fuelSpin queues a message, then loops forever in hook.
func fuelSpin(hook string) []byte {
	g := newGuest()
	g.hook(hook,
		call("message", false, g.str("spin before")),
		[]byte{wasmtest.OpLoop, wasmtest.Empty, wasmtest.OpBr, 0, wasmtest.OpEnd})
	return g.bytes()
}

// fuelHealthy sends "healthy" on on_hour.
func fuelHealthy() []byte {
	g := newGuest()
	g.hook("on_hour", call("message", false, g.str("healthy")))
	return g.bytes()
}

// C2: the spinning file's call is all or nothing: its message queued
// before the loop is dropped, the file is off with "out of fuel in
// on_hour", it is never called again, and the healthy file's commands are
// kept, in either load order, on both engines.
func TestFuelSpinIsAllOrNothingAndTheHealthyFileRuns(t *testing.T) {
	for _, spinFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("spinFirst=%t", spinFirst), func(t *testing.T) {
			goldenEachEngine(t, func(t *testing.T, o Options) {
				ps := []Plugin{
					plug("spin", "scripts/spin.wasm", fuelSpin("on_hour")),
					plug("ok", "scripts/ok.wasm", fuelHealthy()),
				}
				if !spinFirst {
					slices.Reverse(ps)
				}
				h := newHost(t, o, ps...)
				res := fire(h, event(scripting.OnHour))
				want := scripting.Problem{Plugin: "spin", File: "scripts/spin.wasm", Msg: fmt.Sprintf("out of fuel in on_hour (budget %d)", DefaultFuel)}
				if len(res.Disabled) != 1 || res.Disabled[0] != want {
					t.Fatalf("disabled %+v, want [%+v]", res.Disabled, want)
				}
				if got := messages(res.Commands); !slices.Equal(got, []string{"healthy"}) {
					t.Errorf("commands %q, want only the healthy file's", got)
				}
				if probs := h.Problems(); len(probs) != 1 || probs[0] != want {
					t.Errorf("problems %+v", probs)
				}
				spin := fuelStat(t, h, "spin")
				if !spin.Off || spin.Calls != 1 || spin.Fuel != DefaultFuel {
					t.Errorf("spin stats %+v, want off after 1 call that spent the whole budget", spin)
				}
				res = fire(h, event(scripting.OnHour))
				if got := messages(res.Commands); !slices.Equal(got, []string{"healthy"}) || len(res.Disabled) != 0 {
					t.Errorf("second fire: commands %q disabled %+v", got, res.Disabled)
				}
				if again := fuelStat(t, h, "spin"); again.Calls != 1 || again.Fuel != DefaultFuel {
					t.Errorf("an off file was called again: %+v", again)
				}
				if !h.Uses(scripting.OnHour) {
					t.Error("Uses(on_hour) is false, but the healthy file is still on")
				}
				// Fuel lands on the file that spent it: the healthy hook is
				// i32.const, i32.const, call, end (cost 5) plus message's
				// 4 + 1 word, so 10 per call.
				if ok := fuelStat(t, h, "ok"); ok.Off || ok.Calls != 2 || ok.Fuel != 2*10 {
					t.Errorf("healthy stats %+v, want on, 2 calls, fuel 20", ok)
				}
			})
		})
	}
}

// The loop stops at the same fuel on both engines for a small budget, and
// a budget one short of a healthy call's need stops it too.
func TestFuelBudgetBoundaryIsExactOnBothEngines(t *testing.T) {
	need := uint64(fuelKnownCost) // one call of fuelKnown
	goldenEachEngine(t, func(t *testing.T, o Options) {
		for _, tc := range []struct {
			fuel uint64
			off  bool
		}{{need, false}, {need - 1, true}} {
			o.Fuel = tc.fuel
			h := newHost(t, o, plug("k", "scripts/k.wasm", fuelKnown()))
			res := fire(h, event(scripting.OnHour))
			st := h.Stats()[0]
			if st.Off != tc.off || st.Fuel != tc.fuel {
				t.Errorf("budget %d: %+v (disabled %+v), want off=%t fuel=%d", tc.fuel, st, res.Disabled, tc.off, tc.fuel)
			}
			if tc.off && (len(res.Commands) != 0 || len(res.Disabled) != 1 ||
				res.Disabled[0].Msg != fmt.Sprintf("out of fuel in on_hour (budget %d)", tc.fuel)) {
				t.Errorf("budget %d: commands %v disabled %+v", tc.fuel, res.Commands, res.Disabled)
			}
		}
	})
}

// fuelKnown's on_hour, hand-metered (doc.go "Fuel"; each instruction
// belongs to its innermost loop or to the function, its end included):
//
//	i32.const 10; local.set 0                       function
//	loop                                            function
//	  local.get 0; i32.const 1; i32.sub;
//	  local.tee 0; br_if 0; end                     loop: 6, cost 7, entered 10 times
//	i32.const outAt; i32.const 0; i32.const 1000;
//	memory.fill                                     function, plus 1000 at run time
//	i32.const 1; memory.grow; drop                  function
//	end                                             function
//
// function region 1 + 2 + 1 + 4 + 3 + 1 = 12; loop 10 x 7 = 70; fill 1000.
// So one call is 12 + 70 + 1000 = 1082 fuel and grows memory by a page.
const fuelKnownCost = 12 + 70 + 1000

func fuelKnown() []byte {
	g := newGuest()
	g.b.Memory(1, 8)
	code := cat(
		i32c(10), []byte{wasmtest.OpLocalSet, 0, wasmtest.OpLoop, wasmtest.Empty, wasmtest.OpLocalGet, 0},
		i32c(1), []byte{wasmtest.OpI32Sub, wasmtest.OpLocalTee, 0, wasmtest.OpBrIf, 0, wasmtest.OpEnd},
		i32c(outAt), i32c(0), i32c(1000), []byte{wasmtest.PrefixFC, wasmtest.FCMemoryFill, 0},
		i32c(1), []byte{wasmtest.OpMemoryGrow, 0, wasmtest.OpDrop, wasmtest.OpEnd},
	)
	g.b.Export("on_hour", wasmtest.KindFunc, g.b.Func(nil, nil, []byte{wasmtest.I32}, code...))
	return g.bytes()
}

// fuelStat is the Stat of plugin id.
func fuelStat(t *testing.T, h *Host, id string) Stat {
	t.Helper()
	for _, s := range h.Stats() {
		if s.Plugin == id {
			return s
		}
	}
	t.Fatalf("no stat for %s in %+v", id, h.Stats())
	return Stat{}
}

// C4: Stats' fuel and pages are exact for a known module: 1082 fuel and
// one page per call, the same on both engines.
func TestFuelStatsAreExactForAKnownModule(t *testing.T) {
	goldenEachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, plug("k", "scripts/k.wasm", fuelKnown()))
		if st := h.Stats()[0]; st.Fuel != 0 || st.Calls != 0 || st.Pages != 1 {
			t.Fatalf("fresh: %+v", st)
		}
		for i := uint64(1); i <= 3; i++ {
			res := fire(h, event(scripting.OnHour))
			if len(res.Disabled) != 0 {
				t.Fatalf("disabled %+v", res.Disabled)
			}
			st := h.Stats()[0]
			if st.Fuel != i*fuelKnownCost || st.Calls != i || st.Pages != uint32(1+i) || st.Off {
				t.Errorf("after %d calls: %+v, want fuel %d pages %d", i, st, i*fuelKnownCost, 1+i)
			}
		}
	})
}

// An import's cost is exact through Stats too: on_hour = day(); drop is
// function cost 1 + 3 (call, drop, end) plus day's 4.
func TestFuelImportCostIsInStats(t *testing.T) {
	goldenEachEngine(t, func(t *testing.T, o Options) {
		g := newGuest()
		g.hook("on_hour", call("day", true))
		h := newHost(t, o, plug("d", "scripts/d.wasm", g.bytes()))
		fire(h, event(scripting.OnHour))
		fire(h, event(scripting.OnHour))
		if st := h.Stats()[0]; st.Fuel != 2*(1+3+importCost) || st.Calls != 2 {
			t.Errorf("stats %+v, want fuel %d", st, 2*(1+3+importCost))
		}
	})
}
