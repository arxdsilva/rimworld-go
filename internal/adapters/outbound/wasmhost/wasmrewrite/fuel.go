// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

// Meter inserts the fuel charges (function entry, after each loop opcode,
// bulk memory ops by length) and the weighted call-depth charge around
// every body.
func Meter(m *Module, c Config) error {
	return nil // W0 stub
}
