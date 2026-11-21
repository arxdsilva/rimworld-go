// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

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
	// MaxWeight bounds one function's weight w(f) after metering; Meter
	// rejects a heavier function (RuleWeight). It keeps every accepted
	// body within wazevo arm64's 16 MiB machine-code cap per function, so
	// no engine or arch refuses a module another one loads. Zero means
	// DefaultMaxWeight.
	MaxWeight uint64
}

// Prepare checks wasm and returns the rewritten module: Decode, Check,
// Reserve, the saved-global exports (SavedGlobals, GlobalName),
// CanonicaliseNaN, Meter, Encode. Check runs before any rewrite, so an
// invalid module is rejected, never repaired.
func Prepare(wasm []byte, c Config) ([]byte, error) {
	m, err := Decode(wasm)
	if err != nil {
		return nil, err
	}
	if err := Check(m, c); err != nil {
		return nil, err
	}
	saved := SavedGlobals(m)
	if err := Reserve(m, c); err != nil {
		return nil, err
	}
	exportGlobals(m, saved)
	if err := CanonicaliseNaN(m); err != nil {
		return nil, err
	}
	if err := Meter(m, c); err != nil {
		return nil, err
	}
	return m.Encode(), nil
}
