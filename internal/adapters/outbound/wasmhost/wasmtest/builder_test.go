// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmtest_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

// Serial on purpose: wazero v1.9.0's compiler races under -race when
// runtimes are created concurrently.
func configs() map[string]wazero.RuntimeConfig {
	m := map[string]wazero.RuntimeConfig{"interpreter": wazero.NewRuntimeConfigInterpreter()}
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
		m["compiler"] = wazero.NewRuntimeConfigCompiler()
	}
	return m
}

func build() []byte {
	var b Builder
	b.Memory(1, 2)
	b.Table(2)
	b.Global(I32, true, OpI32Const, 9, OpEnd)
	hostLog := b.Import("env", "log", []byte{I32}, nil)
	if hostLog != 0 {
		panic("first import is function 0")
	}
	f := b.Func([]byte{I32}, []byte{I32}, []byte{I32, I32, I64},
		OpLocalGet, 0, OpGlobalGet, 0, OpI32Add, OpEnd)
	if f != 1 {
		panic("first func follows the imports")
	}
	b.Elem(f)
	b.Data(0, []byte{1, 2, 3})
	b.Export("f", KindFunc, f)
	b.Export("mem", KindMemory, 0)
	b.Export("g", KindGlobal, 0)
	return b.Bytes()
}

func TestBuilderOutputCompilesAndRuns(t *testing.T) {
	wasm := build()
	ctx := context.Background()
	for name, cfg := range configs() {
		r := wazero.NewRuntimeWithConfig(ctx, cfg)
		_, err := r.NewHostModuleBuilder("env").NewFunctionBuilder().
			WithFunc(func(uint32) {}).Export("log").Instantiate(ctx)
		if err != nil {
			t.Fatal(err)
		}
		mod, err := r.InstantiateWithConfig(ctx, wasm, wazero.NewModuleConfig())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		res, err := mod.ExportedFunction("f").Call(ctx, 5)
		if err != nil || res[0] != 14 {
			t.Errorf("%s: f(5) = %v, %v", name, res, err)
		}
		if b, ok := mod.Memory().Read(0, 3); !ok || b[2] != 3 {
			t.Errorf("%s: data segment %v", name, b)
		}
		if g := mod.ExportedGlobal("g"); g == nil || g.Type() != api.ValueTypeI32 {
			t.Errorf("%s: global export", name)
		}
		r.Close(ctx)
	}
}

func TestBuilderTypeIsDeduplicated(t *testing.T) {
	var b Builder
	a := b.Type([]byte{I32}, nil)
	if b.Type([]byte{I32}, nil) != a || b.Type([]byte{I64}, nil) == a {
		t.Error("Type dedup")
	}
	b.Func([]byte{I32}, nil, nil, OpEnd)
	b.Func([]byte{I32}, nil, nil, OpEnd)
	if b.Func([]byte{}, nil, nil, OpEnd) != 2 {
		t.Error("function indices")
	}
}

func TestBuilderPassiveSegmentsAndStart(t *testing.T) {
	var b Builder
	b.Memory(1, 0)
	b.Table(1)
	f := b.Func(nil, nil, nil, OpEnd)
	b.ElemPassive(f)
	b.ElemDeclare(f)
	b.DataPassive([]byte{7})
	b.Start(f)
	ctx := context.Background()
	for name, cfg := range configs() {
		r := wazero.NewRuntimeWithConfig(ctx, cfg)
		if _, err := r.InstantiateWithConfig(ctx, b.Bytes(), wazero.NewModuleConfig()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		r.Close(ctx)
	}
}

func TestBuilderBytesIsRepeatable(t *testing.T) {
	var b Builder
	b.Memory(1, 0)
	b.Data(70000, nil)
	b.Func(nil, nil, nil, OpEnd)
	first, second := b.Bytes(), b.Bytes()
	if string(first) != string(second) {
		t.Error("Bytes changes between calls")
	}
}
