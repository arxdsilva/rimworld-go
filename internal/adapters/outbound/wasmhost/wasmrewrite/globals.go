// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import "strconv"

// GlobalPrefix starts the export name of every saved global: the host
// reads and writes them through these exports to copy a live instance
// (wasmhost's save copy). Check reserves the "__rimworld" prefix, so no
// original export can take one.
const GlobalPrefix = "__rimworld_g"

// GlobalName is the export name of the global at idx in the global index
// space.
func GlobalName(idx uint32) string {
	return GlobalPrefix + strconv.FormatUint(uint64(idx), 10)
}

// SavedGlobals lists, in index order, the global-index-space indices of
// m's defined mutable numeric (i32, i64, f32, f64) globals: the state a
// copy of an instance must carry besides its memory. Call it before
// Reserve, so the fuel and depth globals are not among them. Check rejects
// a mutable reference global (RuleRefGlobal): its value points into the
// instance that holds it, so no copy could carry it.
func SavedGlobals(m *Module) []uint32 {
	base := m.importCount(3)
	var out []uint32
	for i, g := range m.Globals {
		if g.Mutable && numeric(g.Type) {
			out = append(out, base+uint32(i))
		}
	}
	return out
}

// exportGlobals exports each of idx under GlobalName.
func exportGlobals(m *Module, idx []uint32) {
	for _, i := range idx {
		m.AddExport(Export{Name: GlobalName(i), Kind: 3, Index: i})
	}
}

func numeric(t byte) bool {
	return t == 0x7F || t == 0x7E || t == 0x7D || t == 0x7C
}
