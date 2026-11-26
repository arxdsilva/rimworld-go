// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// TradeSource is the open trade session, if any. The simulation offers it
// optionally (SOLID_GO.md §5); the Host resolves it.
type TradeSource interface {
	TradeView() (game.TradeView, bool)
}

// CommsSource is the open comms session at a radio console, if any. The
// simulation offers it optionally; the Host resolves it.
type CommsSource interface {
	CommsView() (game.CommsView, bool)
}

// CaravanSource is what the Form caravan dialog plans with: the
// settlements a caravan may visit and how long a caravan of members
// carrying load would take to reach tile dest, and why it cannot (#14).
type CaravanSource interface {
	WorldSites() []game.WorldSiteView
	FormEstimate(members []entity.EntityID, dest int, load int32) (ticks int, why string)
}

var (
	_ TradeSource   = (*game.Game)(nil)
	_ CommsSource   = (*game.Game)(nil)
	_ CaravanSource = (*game.Game)(nil)
)
