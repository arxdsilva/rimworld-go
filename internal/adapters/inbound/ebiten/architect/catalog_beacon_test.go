// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package architect

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
)

func TestBeaconCardStatesItsPowerDraw(t *testing.T) {
	t.Parallel()
	d, ok := ToolByID("trade_beacon")
	if !ok {
		t.Fatal("no trade beacon card")
	}
	draw := power.SpecOf(construction.TradeBeacon).Draw
	if draw != 40 {
		t.Fatalf("beacon draw %d, want 40", draw)
	}
	if want := fmt.Sprintf("Needs %d W of power", draw); !strings.Contains(d.Desc, want) {
		t.Errorf("desc %q lacks %q", d.Desc, want)
	}
	_ = lang.English{}
}
