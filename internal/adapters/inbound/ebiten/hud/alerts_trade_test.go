// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package hud

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
)

func TestTradeAlertNamesTheMissingPoweredBeacon(t *testing.T) {
	t.Parallel()
	tr := lang.English{}
	cases := []struct {
		o    game.OrbitalView
		want string
	}{
		{game.OrbitalView{Present: true, HoursLeft: 7, CallReason: game.ReasonNoBeacon}, "Trade ship in orbit (7h left): no powered beacon"},
		{game.OrbitalView{Present: true, HoursLeft: 7, CallOK: true}, "Trade ship in orbit (7h left)"},
		{game.OrbitalView{Present: true, HoursLeft: 7, CallReason: "no operator"}, "Trade ship in orbit (7h left)"},
	}
	for _, c := range cases {
		got := tradeAlerts(tr, c.o, false)
		if len(got) != 1 || got[0].Label != c.want {
			t.Errorf("%+v: %+v, want label %q", c.o, got, c.want)
		}
	}
}
