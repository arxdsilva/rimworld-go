// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import "errors"

// errNotYet marks a wave-0 stub; it is gone once every stub is replaced.
var errNotYet = errors.New("wasmrewrite: not implemented")

// Config is what the rewriter needs from the host; wasmhost fills it from
// its constants.
type Config struct {
	// Namespace is the only module name imports may come from ("rimworld").
	Namespace string
	// FuelExport and DepthExport are the reserved globals' export names
	// ("__rimworld_fuel", "__rimworld_depth"). No original export may
	// start with "__rimworld".
	FuelExport, DepthExport string
	// MaxDepth is the most floor-weight frames that fit in StackBudget.
	MaxDepth uint32
	// MaxLocals bounds params plus locals per function.
	MaxLocals uint32
	// MaxBody bounds a function body's size in bytes.
	MaxBody uint32
	// MaxArity bounds the params and, separately, the results of every
	// type in the type section.
	MaxArity uint32
	// StackBudget is the weighted call-depth budget in units.
	StackBudget uint64
}

// Prepare checks wasm and returns the rewritten module: Decode, Check,
// Reserve, CanonicaliseNaN, Meter, Encode. Check runs before any rewrite,
// so an invalid module is rejected, never repaired.
func Prepare(wasm []byte, c Config) ([]byte, error) {
	m, err := Decode(wasm)
	if err != nil {
		return nil, err
	}
	if err := Check(m, c); err != nil {
		return nil, err
	}
	if err := Reserve(m, c); err != nil {
		return nil, err
	}
	if err := CanonicaliseNaN(m); err != nil {
		return nil, err
	}
	if err := Meter(m, c); err != nil {
		return nil, err
	}
	return m.Encode(), nil
}
