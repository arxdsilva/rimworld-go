// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/world/geology"
)

// Windfalls (25.2, #13): helpful incidents that land something on the
// map. An escape pod crashes a hurt stranger near home, an ore meteor
// leaves a small rock holding ore, a supply drop lands forbidden cargo.
// The tuning is our own; executing them is application/game/windfall.
const (
	// CrashHurt is the blunt blow the crashed stranger takes: enough to
	// down a visitor without killing it.
	CrashHurt = health.DownedInjury + 10
	// MeteorCellsMin and MeteorCellsMax bound the meteor's rock cells.
	MeteorCellsMin, MeteorCellsMax = 3, 6
	// MeteorTries centres are tried, each within MeteorRadius of home.
	MeteorTries  = 40
	MeteorRadius = 30
	// SupplyStacksMin and SupplyStacksMax bound the distinct kinds a
	// supply drop lands.
	SupplyStacksMin, SupplyStacksMax = 2, 3
)

// Supply is a kind a supply drop can land, with its quantity range
// (inclusive).
type Supply struct {
	Kind     item.Kind
	Min, Max int
}

// SupplyLoot is what a supply drop draws its stacks from (distinct kinds).
var SupplyLoot = [...]Supply{
	{item.SurvivalMeal, 6, 12},
	{item.HerbalMedicine, 3, 6},
	{item.Steel, 25, 60},
	{item.Components, 2, 5},
	{item.Wood, 30, 75},
}

// OreWeight is an ore a meteor can hold, with its relative chance.
type OreWeight struct {
	Ore    geology.Ore
	Weight int
}

// MeteorOres are the ores a meteor's rock holds, by weight.
var MeteorOres = [...]OreWeight{
	{geology.Steel, 6},
	{geology.Silver, 3},
	{geology.Machinery, 2},
	{geology.Gold, 1},
}
