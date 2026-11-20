// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
)

// Limits and names of ABI v1 (doc.go).
const (
	ABIVersion      = 1
	Namespace       = "rimworld"
	DefaultFuel     = 1_000_000 // per call, or per file per tick for on_pawn
	DefaultMaxPages = 256       // 16 MiB
	MaxState        = 64 << 10  // bytes of a save blob's data
	MaxDepth        = 1000      // floor-weight frames that fit in StackBudget
	StackBudget     = 1 << 22   // weighted call-depth units
	MaxArity        = 64        // params, and results, per type
	MaxLocals       = 4096      // params plus locals per function
	MaxBody         = 64 << 10  // bytes per function body

	fuelExport  = "__rimworld_fuel"
	depthExport = "__rimworld_depth"
)

// Engine picks wazero's engine.
type Engine uint8

// The engines.
const (
	Auto        Engine = iota // wazero's default for the platform
	Compiler                  // wazevo
	Interpreter               // the portable interpreter
)

// Options tune a Host; a zero field means its default.
type Options struct {
	Engine   Engine
	Fuel     uint64 // per call; 0 is DefaultFuel
	MaxPages uint32 // per memory; 0 is DefaultMaxPages
}

// withDefaults fills the zero fields.
func (o Options) withDefaults() Options {
	if o.Fuel == 0 {
		o.Fuel = DefaultFuel
	}
	if o.MaxPages == 0 {
		o.MaxPages = DefaultMaxPages
	}
	return o
}

// runtimeConfig is the wazero config for o (defaults filled): ABI v1's
// features, o's memory limit and engine, and no close on context done.
func runtimeConfig(o Options) wazero.RuntimeConfig {
	var c wazero.RuntimeConfig
	switch o.Engine {
	case Compiler:
		c = wazero.NewRuntimeConfigCompiler()
	case Interpreter:
		c = wazero.NewRuntimeConfigInterpreter()
	default:
		c = wazero.NewRuntimeConfig()
	}
	return c.
		WithCoreFeatures(api.CoreFeaturesV2 &^ api.CoreFeatureSIMD).
		WithMemoryLimitPages(o.MaxPages).
		WithCloseOnContextDone(false)
}

// rewriteConfig is the rewriter's view of the constants above.
func rewriteConfig() wasmrewrite.Config {
	return wasmrewrite.Config{
		Namespace:   Namespace,
		FuelExport:  fuelExport,
		DepthExport: depthExport,
		MaxDepth:    MaxDepth,
		MaxLocals:   MaxLocals,
		MaxBody:     MaxBody,
		MaxArity:    MaxArity,
		StackBudget: StackBudget,
	}
}
