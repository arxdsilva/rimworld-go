package ebiten

import "rimworld-go/internal/application/game"

// PLAY-024 Phase 0 contract: what the trade and comms dialogs (trade_ui.go,
// comms_ui.go; agent U1, Phase 2) need from the game. They read these
// views and send typed commands (command.ProposeTrade with the offer from
// game.EncodeTradeOffer, CloseTrade, the Comms* kinds); they never touch
// the game's state. See MAIN_UI_SCREENS_SPECIFICATION.md §28 and §29.

// tradeSource is the open trade session, if any.
type tradeSource interface {
	TradeView() (game.TradeView, bool)
}

// commsSource is the open comms session at a radio console, if any.
type commsSource interface {
	CommsView() (game.CommsView, bool)
}

var (
	_ tradeSource = (*game.Game)(nil)
	_ commsSource = (*game.Game)(nil)
)
