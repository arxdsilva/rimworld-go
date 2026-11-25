// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package prisoner

// Arrest values are the existing wiki-derived capture rules, in permille.
const (
	ArrestBase      = 600
	ArrestPerSocial = 20
	ArrestMax       = 980
)

// ArrestChance scales an awake target's chance by the arrester's social
// level and ArrestSuccess stat; sleepers and downed targets are certain.
func ArrestChance(downed, asleep bool, social int, arrestSuccess int32) int {
	if downed || asleep {
		return 1000
	}
	chance := int64(ArrestBase+ArrestPerSocial*social) * int64(arrestSuccess) / 1000
	return int(min(max(chance, 0), ArrestMax))
}
