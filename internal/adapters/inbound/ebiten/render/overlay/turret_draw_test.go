// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestTurretInspectorStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		b    game.BuildingView
		want string
	}{
		{game.BuildingView{Kind: SentryTurretKind, Powered: false, ShotAge: 10}, "No power"},
		{game.BuildingView{Kind: SentryTurretKind, Powered: true, ShotAge: -1}, "Idle"},
		{game.BuildingView{Kind: SentryTurretKind, Powered: true, ShotAge: 30}, "Target locked"},
		{game.BuildingView{Kind: SentryTurretKind, Powered: true, ShotAge: 500}, "Idle"},
		{game.BuildingView{Kind: "wall"}, ""},
	}
	for _, c := range cases {
		if got := TurretLine(c.b); got != c.want {
			t.Errorf("%+v: %q, want %q", c.b, got, c.want)
		}
	}
}
