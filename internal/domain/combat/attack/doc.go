// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package attack holds the pure numbers of a ranged or melee attack:
// accuracy bands over distance, burst fire, warmup and cooldown, and the
// hit-chance product. Probabilities are permille (1000 = 100 percent) and
// all math is integer, so results are deterministic. Values follow the
// published RimWorld wiki (ENG-003, extended to defence).
package attack
