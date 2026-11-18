// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package grid

import "math/bits"

func popcount(w uint64) int      { return bits.OnesCount64(w) }
func trailingZeros(w uint64) int { return bits.TrailingZeros64(w) }
