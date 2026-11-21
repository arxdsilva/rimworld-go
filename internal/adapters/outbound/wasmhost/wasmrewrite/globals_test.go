// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

// globalsModule has a mutable i32 (0), an immutable i64 (1), a mutable f64
// (2), an immutable funcref (3), the user's own export of global 0, and get0
// returning global 0.
func globalsModule() []byte {
	var b Builder
	b.Memory(1, 1)
	b.Global(I32, true, OpI32Const, 5, OpEnd)
	b.Global(I64, false, OpI64Const, 0, OpEnd)
	b.Global(F64, true, append(append([]byte{OpF64Const}, make([]byte, 8)...), OpEnd)...)
	b.Global(0x70, false, 0xD0, 0x70, OpEnd) // ref.null func
	b.Export("mine", KindGlobal, 0)
	b.Export("get0", KindFunc, b.Func(nil, []byte{I32}, nil, 0x23, 0, OpEnd))
	return b.Bytes()
}

// C6: Prepare exports the defined mutable numeric globals, and only them,
// as __rimworld_g<index>; no index moves and the user's exports stay.
func TestPrepareExportsMutableGlobals(t *testing.T) {
	in := globalsModule()
	orig, err := wasmrewrite.Decode(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := wasmrewrite.SavedGlobals(orig); !slices.Equal(got, []uint32{0, 2}) {
		t.Errorf("SavedGlobals = %v, want [0 2]", got)
	}
	out, err := wasmrewrite.Prepare(in, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	m, err := wasmrewrite.Decode(out)
	if err != nil {
		t.Fatal(err)
	}
	exp := map[string]wasmrewrite.Export{}
	for _, e := range m.Exports {
		exp[e.Name] = e
	}
	for name, idx := range map[string]uint32{"__rimworld_g0": 0, "__rimworld_g2": 2, "mine": 0} {
		if e, ok := exp[name]; !ok || e.Kind != KindGlobal || e.Index != idx {
			t.Errorf("export %s = %+v (%v), want global %d", name, e, ok, idx)
		}
	}
	for _, name := range []string{"__rimworld_g1", "__rimworld_g3", "__rimworld_g4", "__rimworld_g5"} {
		if _, ok := exp[name]; ok {
			t.Errorf("export %s: immutable, reference or reserved global exported", name)
		}
	}
	if wasmrewrite.GlobalName(7) != wasmrewrite.GlobalPrefix+"7" || wasmrewrite.GlobalPrefix != "__rimworld_g" {
		t.Errorf("GlobalName(7) = %q", wasmrewrite.GlobalName(7))
	}
	for i, g := range orig.Globals { // no index moved
		if m.Globals[i].Type != g.Type || m.Globals[i].Mutable != g.Mutable {
			t.Errorf("global %d changed: %+v -> %+v", i, g, m.Globals[i])
		}
	}

	ctx := context.Background()
	for _, e := range engines(t) {
		r := newRuntime(e)
		mod, err := r.InstantiateWithConfig(ctx, out, wazero.NewModuleConfig().WithStartFunctions())
		if err != nil {
			t.Fatalf("%s: %v", e.name, err)
		}
		g, ok := mod.ExportedGlobal("__rimworld_g0").(api.MutableGlobal)
		if !ok {
			t.Fatalf("%s: __rimworld_g0 is not a mutable global", e.name)
		}
		g.Set(41)
		if fuel, ok := mod.ExportedGlobal("__rimworld_fuel").(api.MutableGlobal); ok {
			fuel.Set(1 << 20)
		}
		res, err := mod.ExportedFunction("get0").Call(ctx)
		if err != nil || res[0] != 41 {
			t.Errorf("%s: get0 = %v, %v; want 41 through the export", e.name, res, err)
		}
		_ = r.Close(ctx)
	}
}

// C12: a mutable reference global is rejected, since no copy of an
// instance could carry it; an immutable one stays allowed.
func TestCheckRejectsMutableReferenceGlobals(t *testing.T) {
	for _, typ := range []byte{0x70, 0x6F} { // funcref, externref
		var b Builder
		b.Global(I32, true, OpI32Const, 0, OpEnd)
		b.Global(typ, true, 0xD0, typ, OpEnd)
		_, err := wasmrewrite.Prepare(b.Bytes(), testConfig())
		if err == nil || !strings.Contains(err.Error(), wasmrewrite.RuleRefGlobal) || !strings.Contains(err.Error(), "global 1") {
			t.Errorf("mutable 0x%x global: %v", typ, err)
		}
		var ok Builder
		ok.Global(typ, false, 0xD0, typ, OpEnd)
		if _, err := wasmrewrite.Prepare(ok.Bytes(), testConfig()); err != nil {
			t.Errorf("immutable 0x%x global: %v", typ, err)
		}
	}
}

// A module with imported globals names its defined globals by their place
// in the whole global index space.
func TestSavedGlobalsCountImportedGlobals(t *testing.T) {
	var b Builder
	b.ImportRaw("rimworld", "g", KindGlobal, I32, 0)
	b.Global(I32, true, OpI32Const, 0, OpEnd)
	m, err := wasmrewrite.Decode(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got := wasmrewrite.SavedGlobals(m); !slices.Equal(got, []uint32{1}) {
		t.Errorf("SavedGlobals = %v, want [1]", got)
	}
}
