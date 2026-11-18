// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/application/game"
)

func tradeLines(rows []game.TradeRowView) []dialogs.TradeLine {
	var out []dialogs.TradeLine
	for _, l := range dialogs.TradeLines(rows) {
		out = append(out, dialogs.TradeLine{Head: l.Head, Row: l.Row})
	}
	return out
}
