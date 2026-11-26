// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/randx"
)

// Settlement inventories are distinct from travelling and orbital traders.
// wiki: Trade, Faction bases. Stock ranges for items listed there follow
// the public inventory table; other ranges are original tuning.
var settlementPools = [3][]entry{
	faction.Neolithic: {
		{item.Wood, 180, 300, 100}, {item.Cloth, 70, 140, 100},
		{item.Leather, 35, 90, 85}, {item.Potatoes, 40, 120, 90},
		{item.Corn, 40, 100, 85}, {item.Meat, 30, 100, 80},
		{item.HerbalMedicine, 12, 40, 100}, {item.SlateBlocks, 70, 160, 75},
		{item.Club, 2, 5, 75}, {item.Spear, 2, 5, 70},
		{item.Wool, 30, 70, 60},
	},
	faction.Industrial: {
		{item.Steel, 300, 600, 100}, {item.Wood, 200, 400, 100},
		{item.Cloth, 100, 300, 100}, {item.Components, 20, 70, 100},
		{item.IndustrialMedicine, 25, 50, 100}, {item.Gold, 30, 150, 65},
		{item.HerbalMedicine, 15, 40, 90}, {item.Rifle, 2, 5, 85},
		{item.Parka, 2, 6, 80}, {item.SurvivalMeal, 25, 90, 100},
		{item.Novel, 1, 4, 70}, {item.Textbook, 1, 3, 50},
	},
	faction.Spacer: {
		{item.Steel, 200, 450, 100}, {item.Gold, 60, 170, 100},
		{item.Components, 25, 70, 100}, {item.Cloth, 100, 250, 100},
		{item.IndustrialMedicine, 25, 50, 100},
		{item.NanomedKit, 2, 8, 85}, {item.Rifle, 2, 6, 90},
		{item.SurvivalMeal, 35, 100, 100}, {item.ServoArm, 1, 2, 40},
		{item.ServoLeg, 1, 2, 40}, {item.Textbook, 2, 5, 85},
		{item.Schematic, 1, 2, 60},
	},
}

// SettlementKind is a base's trading contract, determined by its faction
// technology. The IDs are persisted in Stock.Kind.
func SettlementKind(tech faction.Tech) TraderKind {
	switch tech {
	case faction.Neolithic:
		return TribalSettlement
	case faction.Spacer:
		return SpacerSettlement
	default:
		return IndustrialSettlement
	}
}

// GenerateSettlement generates a base's fresh stock from an independent,
// caller-derived seed. A revisit in the same restock epoch never rerolls.
func GenerateSettlement(tech faction.Tech, seed uint64) Stock {
	return generateSettlement(tech, SettlementKind(tech), randx.New(seed))
}

// generateSettlement also serves the generic trader generator for all
// persisted TraderKind values. The base kind determines stock; faction
// tech still forbids industrial goods when the generator is called for
// a neolithic faction.
func generateSettlement(tech faction.Tech, k TraderKind, rng *randx.Rand) Stock {
	d := DefOf(k)
	st := Stock{Kind: k, Silver: d.SilverMin + rng.IntN(d.SilverMax-d.SilverMin+1)}
	pool := settlementPools[int(k-TribalSettlement)]
	st.Lines = make([]Line, 0, len(pool))
	for _, e := range pool {
		q := e.min + rng.IntN(e.max-e.min+1)
		roll := rng.IntN(100)
		if tech == faction.Neolithic && neolithicBanned(e.kind) {
			continue
		}
		if roll < e.pct {
			st.Lines = append(st.Lines, Line{Kind: e.kind, Qty: q})
		}
	}
	slices.SortFunc(st.Lines, func(a, b Line) int { return a.Goods().Compare(b.Goods()) })
	return st
}
