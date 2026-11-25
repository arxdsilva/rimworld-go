// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package scenario builds deterministic benchmark fixtures for the engine
// scale work (NG-1 §6.2): grown colonies assembled only through the
// game's public dev hooks and typed commands, warmed up, and returned as
// save bytes so every benchmark Loads an identical starting state. The
// colonist count is a parameter (DefaultColonists = 20 is the fixed layout,
// MinColonists = 3 the start kit, more stand outside the colony site); it
// is part of the memo key and, other than 20, of the cache file name.
package scenario
