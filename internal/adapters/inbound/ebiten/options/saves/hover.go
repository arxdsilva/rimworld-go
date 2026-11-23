// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package saves

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
)

// RowCentre is the centre of the n-th shown save row on the Load screen
// (0 is the top row), where a capture rests the cursor so the row's card
// opens. ok is false when fewer than n+1 rows are shown.
func (b *Book) RowCentre(n, sw, sh int) (x, y int, ok bool) {
	seen := 0
	for _, bt := range b.LoadButtons(sw, sh) {
		id, found := strings.CutPrefix(bt.ID, ids.PrefixLoadSlot)
		if i, err := strconv.Atoi(id); !found || err != nil || i < 0 {
			continue
		}
		if seen == n {
			return int(bt.X + bt.W/2), int(bt.Y + bt.H/2), true
		}
		seen++
	}
	return 0, 0, false
}
