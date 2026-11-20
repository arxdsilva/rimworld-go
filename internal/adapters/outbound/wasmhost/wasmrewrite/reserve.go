// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

// Reserve appends the fuel and depth globals (i64, mutable, init 0),
// exports them as c.FuelExport and c.DepthExport and records their
// indices in m.FuelGlobal and m.DepthGlobal.
func Reserve(m *Module, c Config) error {
	return nil // W0 stub
}
