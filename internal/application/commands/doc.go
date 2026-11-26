// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package commands orders the typed commands that enter the simulation. Its
// Queue stamps each submitted command with a unique, increasing sequence
// number and releases due commands in (Tick, Seq) order, so the command phase
// is deterministic for a given seed and command stream (#8). The queue is
// owned by the single simulation owner and is not safe for concurrent use.
//
// Glossary: a command's envelope is its (Tick, Seq); a command is due when
// its Tick is at or before the current tick.
package commands
