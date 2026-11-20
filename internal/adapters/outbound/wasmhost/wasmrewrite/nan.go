// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

// CanonicaliseNaN follows every f32/f64 add, sub, mul, div, sqrt, min,
// max, ceil, floor, trunc, nearest, f32.demote_f64 and f64.promote_f32
// with a select that replaces any NaN by the canonical one (0x7FC00000,
// 0x7FF8000000000000), using one scratch local per float type per body.
func CanonicaliseNaN(m *Module) error {
	return nil // W0 stub
}
