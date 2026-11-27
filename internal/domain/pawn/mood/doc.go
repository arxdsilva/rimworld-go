// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package mood is pawn thoughts and the mood they add up to.
//
// Book is the only path to a thought's tuning (label, value, duration, stack
// limit and factor): core's numbers with plugin patches applied (Core, NewBook).
// A kind whose value or line a call site computes must be listed in
// CodeValued, which plugins cannot patch.
package mood
