// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package setupview

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/faction"
)

func TestCloneFactionsKeepsIndependentWarAndTradeState(t *testing.T) {
	t.Parallel()
	in := []faction.Faction{{ID: 2, Type: faction.Drifters,
		Enemies: []int{1, 3}, Settlements: []int{44}, SettlementNames: []string{"A"},
		History: []int16{-80}, Events: []string{"trade"}, TradeRemainder: 357,
		DriftDay: 4}}
	got := CloneFactions(in)
	if !reflect.DeepEqual(in, got) {
		t.Fatalf("cloned faction lost saved state: %+v", got)
	}
	got[0].Enemies[0] = 9
	got[0].Settlements[0] = 77
	got[0].SettlementNames[0] = "B"
	got[0].History[0] = -20
	got[0].Events[0] = "raid"
	if in[0].Enemies[0] != 1 || in[0].Settlements[0] != 44 ||
		in[0].SettlementNames[0] != "A" || in[0].History[0] != -80 ||
		in[0].Events[0] != "trade" || got[0].TradeRemainder != 357 ||
		got[0].DriftDay != 4 {
		t.Fatal("cloned factions share independent map and diplomatic state")
	}
}
