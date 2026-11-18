// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package scalegolden is the scale golden: six seeded runs pinned by state
// hash, run under -race in their own test binary. It holds no code: the
// test drives the game package through its exported API (NewColony, Step,
// StateHash), and its 20 000-tick runs stay out of the game package's
// test binary and CI budget.
package scalegolden
