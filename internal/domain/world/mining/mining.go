// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package mining owns the published MIN-1 material and worker stat rules.
package mining

import "rimworld-go/internal/domain/world/geology"

// WorkTicksForStone is wall labour at 100% mining speed.
// wiki: Mining Speed, Granite, Limestone, Slate, Marble, Sandstone.
func WorkTicksForStone(s geology.Stone) int {
	switch s {
	case geology.Sandstone:
		return 600
	case geology.Marble:
		return 675
	case geology.Slate:
		return 750
	case geology.Limestone:
		return 1050
	case geology.Granite:
		return 1350
	}
	return 0
}

// WorkTicksForOre is ore wall labour at 100% mining speed (wiki: Ore).
func WorkTicksForOre(o geology.Ore) int {
	switch o {
	case geology.Steel, geology.Silver, geology.Gold, geology.Jade:
		return 2250
	case geology.Machinery:
		return 3000
	case geology.Uranium:
		return 6000
	case geology.Plasteel:
		return 12000
	}
	return 0
}

// Speed is the published Mining Speed curve, before capacities (permille).
func Speed(level int) int { return 40 + 120*min(max(level, 0), 20) }

// SightSpeed applies Sight's 50% importance with a 100% capacity cap.
func SightSpeed(sight int32) int { return 500 + int(min(max(sight, 0), 1000))/2 }

// Yield is Mining Yield by skill and live capacity (all factors permille).
// wiki: Mining Yield. The capacity importance is 30% / 20%.
func Yield(level int, manip, sight int32) int {
	table := [...]int{600, 700, 800, 850, 900, 925, 950, 975, 1000, 1010, 1020, 1030, 1040, 1050, 1060, 1070, 1080, 1090, 1100, 1120, 1130}
	m := 700 + int(min(max(manip, 0), 1000))*3/10
	v := 800 + int(min(max(sight, 0), 1000))/5
	return min(1250, table[min(max(level, 0), 20)]*m/1000*v/1000)
}

// Amount retains the original floor and positive-stack rounding contract.
func Amount(qty, yield int) int {
	if qty <= 0 || yield <= 0 {
		return 0
	}
	return max(1, qty*yield/1000)
}
