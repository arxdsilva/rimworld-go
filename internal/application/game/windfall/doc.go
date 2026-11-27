// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package windfall is the stateless 25.2 windfalls over a consumer-owned
// Host (#13): the ore meteor, the supply drop and the sky song.
//
// It is a consumer sub-package of internal/application/game
// (.agents/specs/SOLID_GO.md §10): it never imports the game package.
// Everything it needs from the simulation comes through the consumer-owned
// roles in deps.go; the game package builds a Windfall per call over its
// host. The tuning (timing, tables, picks) lives in internal/domain/incident.
//
// Windfall is deterministic: Fits never draws from the Dice, and Fire
// checks its precondition before it draws.
package windfall
