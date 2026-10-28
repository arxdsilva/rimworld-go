package dialogs

import "rimworld-go/internal/application/game"

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

var (
	_ TradeSource = (*game.Game)(nil)
	_ CommsSource = (*game.Game)(nil)
)
