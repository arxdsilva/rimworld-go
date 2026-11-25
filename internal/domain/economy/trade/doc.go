// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package trade is the pure trading model of PLAY-024: trader kinds and
// their stock, buy and sell prices in hundredths of a silver, and settling
// an offer against the colony's goods, the trader's stock and silver, and
// the negotiation terms. It is headless and deterministic: randomness comes
// in as an explicit *randx.Rand, and nothing here knows about maps, pawns
// or the clock. Individual colony stacks, saved trader lots, gift allocation,
// goods movement plans and offer-ID validation remain in this domain; the
// application performs actual item removal only after a complete settlement
// and carrier inventory preflight. Tuning lives in
// .agents/handoffs/PLAY.md#play024_plan (original numbers).
package trade
