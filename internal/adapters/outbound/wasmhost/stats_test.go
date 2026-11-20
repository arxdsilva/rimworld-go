// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"testing"

	"rimworld-go/internal/application/scripting"
)

// C8: Stats lists loaded files in load order; Calls, Pages and Off per
// file; Fuel is what the host reads back from the fuel global.
func TestStatsReportCallsPagesOffAndFuel(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		o.Fuel = 50_000
		g := newGuest()
		trapper := g.hook("on_hour", []byte{0x00}).bytes()
		h := newHost(t, o,
			fixture(t, "p", "hooks"),
			plug("bad", "scripts/bad.wasm", trapper),
			plug("junk", "scripts/junk.wasm", []byte("nope")))
		st := h.Stats()
		if len(st) != 2 || st[0].Plugin != "p" || st[1].Plugin != "bad" {
			t.Fatalf("stats = %+v (a file that failed to load has none)", st)
		}
		if st[0].Calls != 0 || st[0].Fuel != 0 || st[0].Pages == 0 || st[0].Off {
			t.Errorf("fresh: %+v", st[0])
		}

		fire(h, event(scripting.OnHour))
		fire(h, event(scripting.OnDay))
		st = h.Stats()
		if st[0].Calls != 2 || st[0].Off {
			t.Errorf("hooks: %+v", st[0])
		}
		if !st[1].Off || st[1].Calls != 1 {
			t.Errorf("trapper: %+v", st[1])
		}
		// 13 reads and 3 commands per call, each at least 4 fuel
		if st[0].Fuel < 2*16*4 || st[0].Fuel > 2*o.Fuel {
			t.Errorf("fuel = %d", st[0].Fuel)
		}
		// after one more fire, the delta is exactly what the global shows
		before := h.Stats()[0].Fuel
		fire(h, event(scripting.OnHour))
		left := int64(h.mods[0].fuel.Get())
		if delta := h.Stats()[0].Fuel - before; delta != o.Fuel-uint64(left) {
			t.Errorf("fuel delta %d, global says %d", delta, o.Fuel-uint64(left))
		}
	})
}

func TestStatsOutOfFuelSpendsTheWholeBudgetAndPagesGrowth(t *testing.T) {
	g := newGuest()
	g.hook("on_hour", call("day", true), call("day", true))
	h := newHost(t, Options{Engine: Interpreter, Fuel: 5}, plug("p", "scripts/x.wasm", g.bytes()))
	fire(h, event(scripting.OnHour))
	if st := h.Stats()[0]; st.Fuel != 5 || !st.Off {
		t.Errorf("stats = %+v", st)
	}
	if st := h.Stats()[0]; st.Pages != 1 {
		t.Errorf("pages = %d, want the module's 1", st.Pages)
	}
}
