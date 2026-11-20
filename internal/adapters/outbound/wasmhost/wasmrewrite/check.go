// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

// Check enforces the allow-list on the original module before any
// rewrite: opcodes, imports, memory, start section, reserved export
// names, type arity, body size and locals, and every index immediate
// against the original counts. Each error names the rule it broke.
func Check(m *Module, c Config) error {
	return nil // W0 stub
}
