package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/application/game"
)

func tradeLines(rows []game.TradeRowView) []tradeLine {
	var out []tradeLine
	for _, l := range dialogs.TradeLines(rows) {
		out = append(out, tradeLine{head: l.Head, row: l.Row})
	}
	return out
}
