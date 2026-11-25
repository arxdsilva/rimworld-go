// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

// PerKindStackLimit restores the pre-2026-11-01 Core kind table as an
// explicitly selected policy. The established 1000 default is unchanged.
// Special new rows follow their existing identity rules, and kinds without
// an earlier published value use a local 75-unit material cap.
func (k Kind) PerKindStackLimit() int {
	if k.StackLimit() == 1 || k == Fish {
		return k.StackLimit()
	}
	switch k {
	case Silver, Gold:
		return 500
	case Chemfuel, AshFlake, Silverdust, BattleTonic, Reagent, AshleafTea, ProphylTablet, SparkTablet, EmberOil:
		return 150
	case Components, AdvancedComponents:
		return 50
	case HerbalMedicine, IndustrialMedicine, NanomedKit, NumbrootTincture, Beer, Wort, MortarShells, IncendiaryShells, EMPShells, SmokeShells, ReinforcedBarrel, FoamShells:
		return 25
	case Food, Meal, FineMeal, LavishMeal, SurvivalMeal, NutrientPasteMeal, FertileEgg:
		return 10
	case Bones, MindCore, AntimatterWarhead:
		return 1
	}
	if int(k) < CoreKinds {
		return 75
	}
	return k.StackLimit()
}

// WithPerKindStackLimits freezes an opt-in core stack policy. Core limits
// different from the default and plugin-defined kinds retain their values.
// Supply this book in Content on new runs and when loading that content.
func (b Book) WithPerKindStackLimits() Book {
	b = b.core()
	defs := append([]Def(nil), b.defs...)
	for i := 0; i < min(CoreKinds, len(defs)); i++ {
		k := Kind(i)
		if defs[i].Stack == k.StackLimit() {
			defs[i].Stack = k.PerKindStackLimit()
		}
	}
	return NewBook(defs, b.keys)
}
