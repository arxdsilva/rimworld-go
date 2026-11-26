// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// CorrodeOnDeath leaves chemical injuries on exposed organic tissue of
// a dead humanlike body (wiki: Death acidifier). The retained three-to-six
// native injury count, part selection, damage and health cutoff are original
// tuning; the ordinary article does not establish their numeric values.
func CorrodeOnDeath(b *Body, count int, roll func(int) int, now uint64) int {
	if b == nil || b.Anatomy != Humanlike {
		return 0
	}
	made := 0
	for range count {
		total := 0
		for p := Part(0); p < NumParts; p++ {
			if !b.isMissing(p) && !b.hasReplacement(p) && b.partDmg(p) < b.maxHP(p)-1 {
				total += defs[p].Coverage
			}
		}
		if total == 0 {
			break
		}
		pick := roll(total)
		for p := Part(0); p < NumParts; p++ {
			if b.isMissing(p) || b.hasReplacement(p) || b.partDmg(p) >= b.maxHP(p)-1 {
				continue
			}
			if pick < defs[p].Coverage {
				b.hurtPart(p, 1, CorrosiveBurn, now)
				made++
				break
			}
			pick -= defs[p].Coverage
		}
	}
	return made
}
