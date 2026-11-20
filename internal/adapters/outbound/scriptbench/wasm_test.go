// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scriptbench

import (
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

// wasmEngines are the wazero engines measured; the compiler is the shipping
// one on amd64 and arm64.
var wasmEngines = []struct {
	name   string
	engine wasmhost.Engine
}{{"wasm-compiler", wasmhost.Compiler}, {"wasm-interp", wasmhost.Interpreter}}

// wasmBytes reads a wasmhost fixture by relative path (never edited here).
func wasmBytes(tb testing.TB, name string) []byte {
	tb.Helper()
	b, err := os.ReadFile(filepath.Join("..", "wasmhost", "testdata", name+".wasm"))
	if err != nil {
		tb.Fatalf("fixture %s: %v", name, err)
	}
	return b
}

func wasmPlugin(tb testing.TB, name string) wasmhost.Plugin {
	tb.Helper()
	return wasmhost.Plugin{ID: "bench", Files: []wasmhost.File{{Path: "scripts/bench.wasm", Wasm: wasmBytes(tb, name)}}}
}

// wasmHost builds the real wasmhost adapter (fuel metering on, default fuel)
// around one fixture and closes it with the test.
func wasmHost(tb testing.TB, name string, e wasmhost.Engine) *wasmhost.Host {
	tb.Helper()
	h := wasmhost.New([]wasmhost.Plugin{wasmPlugin(tb, name)}, wasmhost.Options{Engine: e})
	tb.Cleanup(func() { _ = h.Close() })
	if p := h.Problems(); len(p) != 0 {
		tb.Fatalf("problems: %v", p)
	}
	return h
}

// wasmFire mirrors starFire.
func wasmFire(tb testing.TB, h *wasmhost.Host, c *colony) []string {
	tb.Helper()
	r := h.Fire(scripting.Event{Hook: scripting.OnHour, Tick: 2500, Seed: 7}, c, c)
	if len(r.Disabled) != 0 {
		tb.Fatalf("disabled: %v", r.Disabled)
	}
	var out []string
	for _, cmd := range r.Commands {
		if cmd.Kind == command.ScriptMessage {
			out = append(out, cmd.Name)
		}
	}
	return out
}

// TestWasmFuelHeadroom fires each bench workload once and checks that its
// fuel fits under wasmhost.DefaultFuel (a Fire that ran out would disable the
// module), logging the fuel and the share of the budget used.
func TestWasmFuelHeadroom(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		n    int
	}{{"bench_batched", 50}, {"bench_batched", 500}, {"bench_percall", 50}, {"bench_percall", 500}, {"bench_noop", 50}}
	for _, e := range wasmEngines {
		for _, tc := range cases {
			h := wasmHost(t, tc.name, e.engine)
			c := newColony(tc.n)
			wasmFire(t, h, c)
			st := h.Stats()
			if len(st) != 1 || st[0].Off {
				t.Fatalf("%s %s n=%d: stats %+v", e.name, tc.name, tc.n, st)
			}
			f := st[0].Fuel
			t.Logf("%s %s n=%d: fuel %d of %d (%.2f%%)", e.name, tc.name, tc.n, f, wasmhost.DefaultFuel, 100*float64(f)/float64(wasmhost.DefaultFuel))
			if f >= wasmhost.DefaultFuel {
				t.Errorf("%s %s n=%d: fuel %d does not fit DefaultFuel %d", e.name, tc.name, tc.n, f, wasmhost.DefaultFuel)
			}
		}
	}
}
