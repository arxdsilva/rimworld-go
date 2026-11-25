// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package replay records a simulation run and verifies it by re-running it
// (#8). A Recorder wraps a Sim, logs every submitted command with the tick
// it was submitted at, and takes checkpoints of the Sim's state hash; Verify
// rebuilds the Sim from the recorded Options, re-submits the commands and
// compares each checkpoint. The versioned JSON file format (Version, File,
// Encode, Decode) also lives here.
//
// Glossary: a checkpoint is (tick, StateHash) taken after the clock reaches
// tick and before that tick's commands are submitted.
//
// Sim is a consumer-owned port; the game's *game.Game satisfies it. The
// package reads no wall clock and no global RNG.
package replay
