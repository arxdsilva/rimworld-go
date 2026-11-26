// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package quest

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	qd "rimworld-go/internal/domain/quest"
)

// The quest snapshot owns nested physical lots, not just the line headers.
func TestExportOwnsPhysicalLots(t *testing.T) {
	lines := []trade.Line{{Kind: item.Wood, Qty: 2, Lots: []trade.Lot{{ID: 1, Stack: item.Stack{Kind: item.Wood, Qty: 2}}}}}
	q := &Quests{list: []qd.Quest{{ID: 1, Parts: []qd.Part{{Kind: qd.PartDelivery, Lines: lines}}, Rewards: []qd.Reward{{Lines: lines}}}}}
	frozen := q.Export()
	lines[0].Lots[0].Stack.Qty = 1
	if frozen.Quests[0].Parts[0].Lines[0].Lots[0].Stack.Qty != 2 ||
		frozen.Quests[0].Rewards[0].Lines[0].Lots[0].Stack.Qty != 2 {
		t.Fatal("quest export retained live lot storage")
	}
	frozen.Quests[0].Parts[0].Lines[0].Lots[0].Stack.Qty = 3
	if lines[0].Lots[0].Stack.Qty != 1 || frozen.Quests[0].Rewards[0].Lines[0].Lots[0].Stack.Qty != 2 {
		t.Fatal("exported quest parts and rewards still alias")
	}
}
