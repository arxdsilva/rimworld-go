// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import "errors"

// Reserve appends the fuel and depth globals (i64, mutable, init 0),
// exports them as c.FuelExport and c.DepthExport and records their
// indices in m.FuelGlobal and m.DepthGlobal. Existing indices do not move.
func Reserve(m *Module, c Config) error {
	if c.FuelExport == "" || c.DepthExport == "" || c.FuelExport == c.DepthExport {
		return errors.New("wasmrewrite: reserve: fuel and depth export names must be non-empty and distinct")
	}
	for _, e := range m.Exports {
		if e.Name == c.FuelExport || e.Name == c.DepthExport {
			return errors.New("wasmrewrite: reserve: export name " + e.Name + " is already taken")
		}
	}
	zero := func() Global {
		return Global{Type: 0x7E, Mutable: true, Init: []byte{0x42, 0x00, opEnd}}
	}
	m.FuelGlobal = m.AddGlobal(zero())
	m.DepthGlobal = m.AddGlobal(zero())
	m.AddExport(Export{Name: c.FuelExport, Kind: 3, Index: m.FuelGlobal})
	m.AddExport(Export{Name: c.DepthExport, Kind: 3, Index: m.DepthGlobal})
	return nil
}
