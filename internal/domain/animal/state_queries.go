// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

// Penned reports whether a tame animal belongs in a livestock pen.
func Penned(a *State) bool { return a.Tame && RosterOf(a.Species).Penned }
