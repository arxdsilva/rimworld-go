// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// HealingDamage excludes permanent wounds and missing parts.
func HealingDamage(b *Body, injury int) int {
	if b == nil {
		return injury
	}
	n := max(0, injury-b.Damage())
	for _, w := range b.Wounds {
		if !w.Permanent {
			n += w.Dmg
		}
	}
	return n
}

// HasDisease reports a disease condition in the body.
func HasDisease(b *Body) bool {
	if b == nil {
		return false
	}
	for _, c := range b.Conds {
		if IsDisease(c.Kind) {
			return true
		}
	}
	return false
}
