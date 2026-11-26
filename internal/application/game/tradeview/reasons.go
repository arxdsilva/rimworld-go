// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package tradeview

// Trade refusals shared by colony and settlement sessions (game aliases
// them as game.ReasonNoTrade and so on; settlement never imports game).
const (
	ReasonNoTrade     = "they won't trade with you"
	ReasonTradeOpen   = "another trade is open"
	ReasonOfferBroken = "that offer makes no sense"
)
