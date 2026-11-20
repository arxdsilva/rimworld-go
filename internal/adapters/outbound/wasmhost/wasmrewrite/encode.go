// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

// Encode writes the module: unchanged sections are copied raw, changed
// ones are re-encoded with recomputed sizes, custom sections are dropped.
func (m *Module) Encode() []byte {
	return nil // W0 stub
}
