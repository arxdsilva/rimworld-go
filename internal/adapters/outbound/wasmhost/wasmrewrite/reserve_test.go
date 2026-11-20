// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
)

// C7: after Reserve the module compiles, exports both reserved globals,
// and the existing global indices are unchanged.
func TestReserve(t *testing.T) {
	m, err := wasmrewrite.Decode(richModule())
	if err != nil {
		t.Fatal(err)
	}
	n := m.NumGlobals()
	first := m.Globals[0]
	if err := wasmrewrite.Reserve(m, testConfig()); err != nil {
		t.Fatal(err)
	}
	if m.FuelGlobal != n || m.DepthGlobal != n+1 {
		t.Fatalf("indices %d %d, want %d %d", m.FuelGlobal, m.DepthGlobal, n, n+1)
	}
	out := m.Encode()
	back, err := wasmrewrite.Decode(out)
	if err != nil {
		t.Fatal(err)
	}
	if back.Globals[0].Type != first.Type || back.Globals[0].Mutable != first.Mutable || string(back.Globals[0].Init) != string(first.Init) {
		t.Error("global 0 changed")
	}
	ctx := context.Background()
	for _, e := range engines(t) {
		r := newRuntime(e)
		mod, err := r.InstantiateWithConfig(ctx, out, wazero.NewModuleConfig().WithStartFunctions())
		if err != nil {
			t.Fatalf("%s: %v", e.name, err)
		}
		for _, name := range []string{"__rimworld_fuel", "__rimworld_depth"} {
			g := mod.ExportedGlobal(name)
			if g == nil || g.Type() != api.ValueTypeI64 || g.Get() != 0 {
				t.Errorf("%s: global %s = %v", e.name, name, g)
				continue
			}
			mg, ok := g.(api.MutableGlobal)
			if !ok {
				t.Fatalf("%s: %s is not mutable", e.name, name)
			}
			mg.Set(42)
			if mg.Get() != 42 {
				t.Errorf("%s: %s not settable", e.name, name)
			}
		}
		res, err := mod.ExportedFunction("calc").Call(ctx)
		if err != nil || res[0] != 12 {
			t.Errorf("%s: calc = %v %v", e.name, res, err)
		}
		r.Close(ctx)
	}
}

func TestReserveErrors(t *testing.T) {
	m, _ := wasmrewrite.Decode(richModule())
	for name, mut := range map[string]func(*wasmrewrite.Config){
		"empty fuel":  func(c *wasmrewrite.Config) { c.FuelExport = "" },
		"empty depth": func(c *wasmrewrite.Config) { c.DepthExport = "" },
		"same":        func(c *wasmrewrite.Config) { c.DepthExport = c.FuelExport },
		"taken":       func(c *wasmrewrite.Config) { c.FuelExport = "add" },
	} {
		c := testConfig()
		mut(&c)
		if err := wasmrewrite.Reserve(m, c); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if len(m.Globals) != 1 {
		t.Error("a failed Reserve changed the module")
	}
}
