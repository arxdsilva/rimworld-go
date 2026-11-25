// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import "rimworld-go/internal/domain/faction"

// CrashFaction chooses a stranger or a surviving human faction (including
// hostile ones). Equal tickets are local tuning; hidden factions never
// become random human crash victims.
func CrashFaction(fs []faction.Faction, roll func(int) int) int {
	n := 1
	for _, f := range fs {
		if f.Type.HasWorldSettlements() && !f.Defeated {
			n++
		}
	}
	pick := roll(n)
	if pick == 0 {
		return 0
	}
	for i, f := range fs {
		if f.Type.HasWorldSettlements() && !f.Defeated {
			pick--
			if pick == 0 {
				return i + 1
			}
		}
	}
	return 0
}
