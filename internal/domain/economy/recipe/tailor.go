package recipe

import (
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
)

// Tailoring (TEMP-4, ENG-003). wiki: Parka, Tuque, Duster, T-shirt,
// Button-down shirt, Pants, Jacket, Cowboy hat, Bowler hat, Tribalwear:
// the wiki's stuff count and work, scaled as every crafted gear here
// (material / 10 rounded up, work / 10, PRD-3), any leathery or fabric
// material (the garment's default when none is chosen), no minimum
// skill (the wiki names none); the product rolls a quality (PRD-2).

// tailor is garment out sewn at the tailoring bench from wikiStuff units
// of material in wikiWork ticks (the wiki's numbers, scaled here).
func tailor(id ID, name, label string, out item.Kind, wikiStuff, wikiWork int) Def {
	return Def{ID: id, Name: name, Label: label, Bench: construction.TailoringBench,
		Input:  Input{Kind: item.DefaultStuff(out), Qty: (wikiStuff + 9) / 10, Stuff: item.MadeFrom(out)},
		Output: out, OutQty: 1, Work: wikiWork / 10, Skill: work.Craft, Learn: skill.Crafting}
}

// tribalSpot is where tribalwear is made besides the tailoring bench.
var tribalSpot = []construction.Kind{construction.CraftingSpot}

// tribal is tribalwear: 60 stuff, 1800 ticks, at the crafting spot as
// well as the tailoring bench (wiki: Tribalwear).
func tribal() Def {
	d := tailor(MakeTribalwear, "make_tribalwear", "Sew tribalwear", item.Tribalwear, 60, 1800)
	d.Also = tribalSpot
	return d
}
