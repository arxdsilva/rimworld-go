// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// hasOpenWound reports a wound on b that can still heal, or a fresh
// stump a tend must close (MED-01).
func HasOpenWound(b *Body) bool {
	if b.Fresh != 0 {
		return true
	}
	for _, w := range b.Wounds {
		if !w.Permanent {
			return true
		}
	}
	return false
}

// OpenWoundDamage sums open wounds; permanent scars do not affect injury repair.
func OpenWoundDamage(b *Body) int {
	n := 0
	for _, w := range b.Wounds {
		if !w.Permanent {
			n += w.Dmg
		}
	}
	return n
}

// NeedsCare includes damaged artificial replacements that still heal.
func NeedsCare(b *Body) bool {
	return b != nil && (HasOpenWound(b) || len(b.Conds) > 0 ||
		b.BloodLoss > 0 || HasDamagedReplacement(b))
}

// TemporaryHealthFraction is our health-bar approximation for threat
// sizing. Healed scars and permanently missing parts do not count as
// current injuries. With no body, use the existing legacy injury record.
func TemporaryHealthFraction(b *Body, injury, maxHP int) float64 {
	if b != nil {
		injury = OpenWoundDamage(b)
		for _, r := range b.Replacements {
			injury += int(r.Damage)
		}
		for p := Part(0); p < b.numParts(); p++ {
			if b.FreshStump(p) {
				injury += b.maxHP(p)
			}
		}
	}
	return float64(min(maxHP, max(0, maxHP-injury))) / float64(max(1, maxHP))
}
