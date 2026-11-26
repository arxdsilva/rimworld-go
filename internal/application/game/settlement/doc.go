// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package settlement is settlement trade (#11): each welcoming
// settlement's market (trade.Market, saved and restocked lazily when a
// trade opens) and the world-level trade session a visiting player caravan
// holds with one. The game drives it through Host and Cargo (deps.go) and
// fills the trade dialog from View (tradeview.Trade). It never imports
// game; the refusal strings it shares with colony trade are tradeview's.
package settlement
