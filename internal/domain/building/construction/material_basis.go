// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package construction

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
)

// upstreamV20Costs records only the changed material definitions from the
// verified v20 writer 4e28e30ba6d23b8f17c5a5d7487e9006fe8769e4.
// Stable kind IDs and unchanged material slots keep their native meaning.
var upstreamV20Costs = map[Kind]Def{
	Floor:                 {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Table:                 {Wood: 3, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	ResearchBench:         {Wood: 8, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	ElectricResearchBench: {Wood: 17, Components: 10, Extra: [3]Cost{}, Salvage: 0},
	EndTable:              {Wood: 3, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Dresser:               {Wood: 6, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Stool:                 {Wood: 3, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Armchair:              {Wood: 12, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Shelf:                 {Wood: 2, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	MultiAnalyzer:         {Wood: 4, Components: 8, Extra: [3]Cost{}, Salvage: 0},
	OrnateDoor:            {Wood: 8, Components: 0, Extra: [3]Cost{{item.Gold, 6}}, Salvage: 0},
	Column:                {Wood: 2, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	AnimalFlap:            {Wood: 3, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	AnimalSleepingBox:     {Wood: 3, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Table2x2:              {Wood: 6, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Table2x4:              {Wood: 11, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Table3x3:              {Wood: 11, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Couch:                 {Wood: 22, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	PlantPot:              {Wood: 2, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	SmallShelf:            {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	WallTorch:             {Wood: 2, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	WallLamp:              {Wood: 2, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	FloodLight:            {Wood: 6, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Urn:                   {Wood: 2, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	LargeStele:            {Wood: 14, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	GrandStele:            {Wood: 28, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	EggBox:                {Wood: 2, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	StrawMatting:          {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	PavedTile:             {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	SteelTile:             {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	SilverTile:            {Wood: 8, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	GoldTile:              {Wood: 8, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	SterileTile:           {Wood: 1, Components: 0, Extra: [3]Cost{{item.Silver, 1}}, Salvage: 0},
	StoneTile:             {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	Flagstone:             {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	CarpetFloor:           {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	WaterproofConduit:     {Wood: 1, Components: 0, Extra: [3]Cost{}, Salvage: 0},
	ChemfuelGenerator:     {Wood: 11, Components: 3, Extra: [3]Cost{}, Salvage: 0},
	Watermill:             {Wood: 31, Components: 3, Extra: [3]Cost{{item.Steel, 9}}, Salvage: 0},
	DeepDrill:             {Wood: 11, Components: 2, Extra: [3]Cost{}, Salvage: 0},
	GroundScanner:         {Wood: 17, Components: 4, Extra: [3]Cost{{item.AdvancedComponents, 1}}, Salvage: 0},
	MoisturePump:          {Wood: 8, Components: 4, Extra: [3]Cost{}, Salvage: 0},
	ElectricCrematorium:   {Wood: 17, Components: 2, Extra: [3]Cost{{item.Steel, 2}}, Salvage: 0},
}

// HistoricalBasis is zero for unchanged kinds. A v20 writer used the
// recorded old catalogue; earlier writers may predate that catalogue.
func HistoricalBasis(k Kind, version int) item.MaterialBasis {
	if version >= 21 {
		return item.MaterialBasis{}
	}
	if _, ok := upstreamV20Costs[k]; !ok {
		return item.MaterialBasis{}
	}
	era := item.MaterialV20
	if version < 20 {
		era = item.MaterialUnverified
	}
	return item.MaterialBasis{Primary: era, Secondary: era}
}

// PaidPrimaryRefund keeps the existing deconstruction fractions while
// respecting the catalogue that supplied the primary material.
func PaidPrimaryRefund(b Building) int {
	d := DefOf(b.Kind)
	old, changed := upstreamV20Costs[b.Kind]
	if !changed || b.MaterialBasis.Primary == item.MaterialCurrent {
		if d.Layer == LayerFloor {
			if b.Kind == Flagstone {
				return 0
			}
			return d.Wood / 2
		}
		return RefundFor(b.Kind, b.Stuff)
	}
	if b.MaterialBasis.Primary == item.MaterialUnverified && old.Wood != d.Wood {
		return 0
	}
	if d.Layer == LayerFloor {
		if b.Kind == Flagstone {
			return 0
		}
		return old.Wood / 2
	}
	if old.Salvage > 0 {
		return old.Salvage
	}
	if d.Carpet {
		return old.Wood
	}
	return old.Wood / 2
}

// PaidSecondary returns the historical component and extra material basis.
// Unknown earlier costs cannot authorize an increased refund.
func PaidSecondary(b Building) (int, [3]Cost) {
	d := DefOf(b.Kind)
	old, changed := upstreamV20Costs[b.Kind]
	if !changed || b.MaterialBasis.Secondary == item.MaterialCurrent {
		return d.Components, d.Extra
	}
	if b.MaterialBasis.Secondary == item.MaterialUnverified {
		if old.Components != d.Components {
			old.Components = 0
		}
		for i, c := range old.Extra {
			if c != d.Extra[i] {
				old.Extra[i] = Cost{}
			}
		}
	}
	return old.Components, old.Extra
}

// RebaseFrame changes absolute requirements only; delivered stock, work,
// identity and reservations are not reset. Reinstall/secondary swaps are
// not purchases of the structure's material a second time.
func RebaseFrame(fr Frame) Frame {
	if fr.Reinstall != 0 {
		fr.Need, fr.CompNeed = 0, 0
		return fr
	}
	fr.Need = CostFor(fr.Kind, fr.Stuff)
	if fr.Swaps != 0 && !fr.SwapNewRecipe {
		fr.CompNeed = 0
	} else {
		fr.CompNeed = ComponentCost(fr.Kind)
	}
	return fr
}

// SetSwapRecipe records whether an in-place replacement buys the new
// secondary recipe. Preserve delivered material and progress on import.
func SetSwapRecipe(fr Frame, old Building) Frame {
	fr.SwapNewRecipe = fr.Swaps != 0 && old.Kind != fr.Kind
	if fr.SwapNewRecipe {
		fr.CompNeed = ComponentCost(fr.Kind)
	} else if fr.Swaps != 0 {
		fr.CompNeed = 0
	}
	return fr
}

// MigrateMaterialBasis restores the construction owner on every loaded
// map. A pending swap whose source is gone becomes a new build. The
// actual source kind, not an old missing flag, determines what it owes.
func MigrateMaterialBasis(version int, frames *component.Store[Frame], buildings *component.Store[Building], stacks *component.Store[item.Stack], frameIDs, buildingIDs, stackIDs []entity.EntityID) {
	for _, id := range frameIDs {
		fr, ok := frames.Get(id)
		if !ok {
			continue
		}
		if fr.Swaps != 0 {
			if old, ok := buildings.Get(fr.Swaps); ok {
				fr = SetSwapRecipe(fr, old)
			} else {
				fr.Swaps, fr.SwapNewRecipe = 0, false
				fr.CompNeed = ComponentCost(fr.Kind)
			}
		}
		if version < 21 {
			fr = RebaseFrame(fr)
		}
		frames.Set(id, fr)
	}
	if version >= 21 {
		return
	}
	for _, id := range buildingIDs {
		if b, ok := buildings.Get(id); ok && b.MaterialBasis == (item.MaterialBasis{}) {
			b.MaterialBasis = HistoricalBasis(b.Kind, version)
			buildings.Set(id, b)
		}
	}
	for _, id := range stackIDs {
		if st, ok := stacks.Get(id); ok && st.Kind == item.MinifiedBuilding && st.MaterialBasis == (item.MaterialBasis{}) {
			st.MaterialBasis = HistoricalBasis(Kind(st.Building), version)
			stacks.Set(id, st)
		}
	}
}
