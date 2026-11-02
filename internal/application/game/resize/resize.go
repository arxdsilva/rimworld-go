// Package resize is the save migrations of building footprints: which
// kinds a save version grew, and what a building of the older size that
// no longer fits gives back (its cost in that older save). The game's
// migrateBenches applies them on load.
package resize

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// V9 reports a kind save v9 grew (RSCH-1: the simple research bench from
// 2x1 to 3x2, the hi-tech one to 5x2).
func V9(k construction.Kind) bool { return construction.RecipeBench(k) == construction.ResearchBench }

// RefundV9 is a research bench's cost in a v8 save: material, count and
// components.
func RefundV9(b construction.Building) (item.Kind, int, int) {
	if b.Kind == construction.ElectricResearchBench {
		return item.Steel, 65, 3
	}
	return b.Stuff, 8, 0
}

// V10 reports a kind save v10 grew (STK-BN, the wiki footprints): every
// 3x1 production bench (2x1 before), the 5x2 fabrication bench (3x2), the
// 3x2 fuel refinery (2x2) and the 3x2 radio console (2x1).
func V10(k construction.Kind) bool {
	d := construction.DefOf(k)
	return d.Bench && d.Size == [2]int{3, 1} ||
		k == construction.FabricationBench || k == construction.BiofuelRefinery || k == construction.RadioConsole
}

// RefundV10 is a v10 kind's cost in a v9 save: v10 changed no cost.
func RefundV10(b construction.Building) (item.Kind, int, int) {
	return b.Stuff, construction.WoodCost(b.Kind), construction.ComponentCost(b.Kind)
}

// V11 reports a kind save v11 grew (A2, wiki: Table: the 1x1 table to
// the 1x2 one; its cost did not change, so RefundV10 refunds it).
func V11(k construction.Kind) bool { return k == construction.Table }
