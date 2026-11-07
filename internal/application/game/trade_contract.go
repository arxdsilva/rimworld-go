package game

import "rimworld-go/internal/application/game/tradeview"

// PLAY-024 Phase 0 contract (lead-owned): the read models the trade and
// comms dialogs consume, in game/tradeview. The producers are
// trade_session.go and comms.go.
type (
	TradeView        = tradeview.Trade
	TradeRowView     = tradeview.Row
	CommsView        = tradeview.Comms
	CommsFactionView = tradeview.Faction
	CommsActionView  = tradeview.Action
	OrbitalView      = tradeview.Orbital
)
