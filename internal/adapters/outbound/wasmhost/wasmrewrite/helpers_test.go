// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
)

// Tests that build wazero runtimes stay serial: wazero v1.9.0's compiler
// engine races under -race when NewRuntime runs concurrently.

type testEngine struct {
	name string
	cfg  func() wazero.RuntimeConfig
}

// engines returns the interpreter always and the compiler where wazero
// supports it (amd64, arm64).
func engines(t *testing.T) []testEngine {
	t.Helper()
	es := []testEngine{{"interpreter", wazero.NewRuntimeConfigInterpreter}}
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
		es = append(es, testEngine{"compiler", wazero.NewRuntimeConfigCompiler})
	}
	return es
}

func newRuntime(e testEngine) wazero.Runtime {
	cfg := e.cfg().
		WithCoreFeatures(api.CoreFeaturesV2 &^ api.CoreFeatureSIMD).
		WithCloseOnContextDone(false)
	return wazero.NewRuntimeWithConfig(context.Background(), cfg)
}

// compileErr compiles wasm on the engine and returns the error, if any.
func compileErr(e testEngine, wasm []byte) error {
	r := newRuntime(e)
	defer r.Close(context.Background())
	_, err := r.CompileModule(context.Background(), wasm)
	return err
}

// tryRun instantiates wasm (start functions off) and calls export.
func tryRun(e testEngine, wasm []byte, export string, args ...uint64) ([]uint64, error) {
	ctx := context.Background()
	r := newRuntime(e)
	defer r.Close(ctx)
	mod, err := r.InstantiateWithConfig(ctx, wasm, wazero.NewModuleConfig().WithStartFunctions())
	if err != nil {
		return nil, err
	}
	// Metered output starts with 0 fuel; give it a budget no test exhausts.
	if g, ok := mod.ExportedGlobal("__rimworld_fuel").(api.MutableGlobal); ok {
		g.Set(1 << 62)
	}
	return mod.ExportedFunction(export).Call(ctx, args...)
}

// run is tryRun that fails the test on error.
func run(t *testing.T, e testEngine, wasm []byte, export string, args ...uint64) []uint64 {
	t.Helper()
	res, err := tryRun(e, wasm, export, args...)
	if err != nil {
		t.Fatalf("%s: run %s: %v", e.name, export, err)
	}
	return res
}

func testConfig() wasmrewrite.Config {
	return wasmrewrite.Config{
		Namespace: "rimworld", FuelExport: "__rimworld_fuel", DepthExport: "__rimworld_depth",
		MaxDepth: 1000, MaxLocals: 4096, MaxBody: 64 << 10, MaxArity: 64, StackBudget: 1 << 22,
		MaxWeight: 1 << 17,
	}
}
