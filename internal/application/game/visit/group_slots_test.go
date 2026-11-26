// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package visit

import "testing"

func TestRoomForNaturalVisitorsCountsOnlyIndependentParties(t *testing.T) {
	groups := []Group{
		{Kind: KindVisitor}, {Kind: KindTraveler}, {Kind: KindSmallTrader},
		{Kind: KindAid}, {Kind: KindManaged}, {Kind: KindQuest},
	}
	if !RoomForNaturalVisitors(groups) {
		t.Fatal("nonvisitor groups exhausted the natural visitor limit")
	}
	groups = append(groups, Group{Kind: KindVisitor})
	if RoomForNaturalVisitors(groups) {
		t.Fatal("four natural visitor groups exceeded the visitor slot budget")
	}
}
