// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

// Opcodes the fuel tests use that the builder does not name.
const (
	fuelOpI64LeS     byte = 0x57
	fuelOpI32WrapI64 byte = 0xA7
)

// fuelPlenty is a fuel budget no test exhausts.
const fuelPlenty = int64(1) << 62

// fuelMeter runs Decode, Check, Reserve, Meter and Encode on wasm. It skips
// CanonicaliseNaN so the fuel and weight numbers here do not depend on the
// NaN pass; the modules below use no float ops anyway.
func fuelMeter(t *testing.T, wasm []byte) []byte {
	t.Helper()
	m := fuelMetered(t, wasm)
	return m.Encode()
}

func fuelMetered(t *testing.T, wasm []byte) *wasmrewrite.Module {
	t.Helper()
	return fuelMeteredWith(t, wasm, testConfig())
}

func fuelMeteredWith(t *testing.T, wasm []byte, c wasmrewrite.Config) *wasmrewrite.Module {
	t.Helper()
	m, err := wasmrewrite.Decode(wasm)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := wasmrewrite.Check(m, c); err != nil {
		t.Fatalf("check: %v", err)
	}
	if err := wasmrewrite.Reserve(m, c); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := wasmrewrite.Meter(m, c); err != nil {
		t.Fatalf("meter: %v", err)
	}
	return m
}

// fuelInstance is one instantiated module whose reserved globals the test
// sets before a call and reads back after it, trap or not.
type fuelInstance struct {
	rt  wazero.Runtime
	mod api.Module
}

func fuelInstantiate(t *testing.T, e testEngine, wasm []byte) *fuelInstance {
	t.Helper()
	ctx := context.Background()
	rt := newRuntime(e)
	mod, err := rt.InstantiateWithConfig(ctx, wasm, wazero.NewModuleConfig().WithStartFunctions())
	if err != nil {
		rt.Close(ctx)
		t.Fatalf("%s: instantiate: %v", e.name, err)
	}
	t.Cleanup(func() { rt.Close(ctx) })
	return &fuelInstance{rt: rt, mod: mod}
}

func (fi *fuelInstance) global(t *testing.T, name string) api.Global {
	t.Helper()
	g := fi.mod.ExportedGlobal(name)
	if g == nil {
		t.Fatalf("no exported global %q", name)
	}
	return g
}

func (fi *fuelInstance) set(t *testing.T, name string, v int64) {
	t.Helper()
	g, ok := fi.global(t, name).(api.MutableGlobal)
	if !ok {
		t.Fatalf("global %q is not mutable", name)
	}
	g.Set(uint64(v))
}

func (fi *fuelInstance) get(t *testing.T, name string) int64 {
	t.Helper()
	return int64(fi.global(t, name).Get())
}

// fuelOut is one call's outcome and the reserved globals after it.
type fuelOut struct {
	res         []uint64
	err         error
	fuel, depth int64
}

// call resets fuel to fuel and depth to 0, as the host does before every
// top-level call, calls export and reads both globals back.
func (fi *fuelInstance) call(t *testing.T, fuel int64, export string, args ...uint64) fuelOut {
	t.Helper()
	fi.set(t, "__rimworld_fuel", fuel)
	fi.set(t, "__rimworld_depth", 0)
	fn := fi.mod.ExportedFunction(export)
	if fn == nil {
		t.Fatalf("no exported function %q", export)
	}
	var out fuelOut
	out.res, out.err = fn.Call(context.Background(), args...)
	out.fuel = fi.get(t, "__rimworld_fuel")
	out.depth = fi.get(t, "__rimworld_depth")
	return out
}

// fuelUsed runs export with plenty of fuel and returns the fuel it used.
func fuelUsed(t *testing.T, fi *fuelInstance, export string, args ...uint64) int64 {
	t.Helper()
	out := fi.call(t, fuelPlenty, export, args...)
	if out.err != nil {
		t.Fatalf("%s%v: %v", export, args, out.err)
	}
	return fuelPlenty - out.fuel
}

// fuelWeight reads w(f) of defined function fn from its metered body: the
// body starts with global.get depth; i64.const w.
func fuelWeight(t *testing.T, m *wasmrewrite.Module, fn int) int64 {
	t.Helper()
	var ins []wasmrewrite.Instr
	_ = wasmrewrite.Walk(m.Bodies[fn].Code, func(in wasmrewrite.Instr) error {
		ins = append(ins, in)
		if len(ins) == 2 {
			return errFuelStop
		}
		return nil
	})
	if len(ins) < 2 || ins[0].Op != uint32(OpGlobalGet) || uint32(ins[0].A) != m.DepthGlobal || ins[1].Op != uint32(OpI64Const) {
		t.Fatalf("body %d does not start with the depth charge: %+v", fn, ins)
	}
	return int64(ins[1].A)
}

type fuelStopErr struct{}

func (fuelStopErr) Error() string { return "stop" }

var errFuelStop = fuelStopErr{}

// fuelOracleWeight recomputes w(f) from the contract's formula on the
// metered body: max(params + locals + V(f), StackBudget/MaxDepth), V(f)
// summing 1 + pushes over every instruction. It is the test's own push
// table, independent of Meter's.
func fuelOracleWeight(t *testing.T, m *wasmrewrite.Module, fn int) int64 {
	t.Helper()
	var ftypes []uint32
	for _, im := range m.Imports {
		if im.Kind == KindFunc {
			ftypes = append(ftypes, im.Index)
		}
	}
	ftypes = append(ftypes, m.Funcs...)
	body := m.Bodies[fn]
	n := uint64(len(m.Types[m.Funcs[fn]].Params))
	for _, r := range body.Locals {
		n += uint64(r.Count)
	}
	arity := func(bt int64) uint64 {
		switch {
		case bt == -64:
			return 0
		case bt < 0:
			return 1
		}
		return uint64(len(m.Types[bt].Results))
	}
	err := wasmrewrite.Walk(body.Code, func(in wasmrewrite.Instr) error {
		n++
		op := in.Op
		switch {
		case op == uint32(OpBlock) || op == uint32(OpLoop) || op == uint32(OpIf):
			n += arity(int64(in.A))
		case op == uint32(OpCall):
			n += uint64(len(m.Types[ftypes[in.A]].Results))
		case op == uint32(OpCallIndirect):
			n += uint64(len(m.Types[in.A].Results))
		case op == 0x1B || op == 0x1C || op == 0x20 || op == 0x22 || op == 0x23 || op == 0x25,
			op >= 0x28 && op <= 0x35, op >= 0x3F && op <= 0xC4, op >= 0xD0 && op <= 0xD2,
			op>>16 == 0xFC && op&0xFFFF <= 7:
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk body %d: %v", fn, err)
	}
	c := testConfig()
	return int64(max(n, c.StackBudget/uint64(c.MaxDepth)))
}
