// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package windfall runs the helpful incidents of 25.2 (#13) for the
// simulation: the escape pod crash (a hurt, faction-less stranger the
// colony may rescue and recruit), the ore meteor (a small rock holding
// ore), the supply drop (forbidden cargo) and the sky song (a timed
// colony-wide mood lift). The tables and tuning are the domain's
// (domain/incident windfall.go, song.go); the game reaches in through
// consumer-owned, value-only roles (deps.go; .agents/specs/SOLID_GO.md
// §7, §10). A Windfall is stateless and built per call.
package windfall
