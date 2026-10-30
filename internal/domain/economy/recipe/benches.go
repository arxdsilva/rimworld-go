package recipe

import (
	"slices"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
)

// PRD-3 bench mapping (ENG-003: wiki recipe pages; costs and work are
// the wiki's scaled the way the club's and knife's are, material / 10
// rounded, components as the wiki, work / 10).
//
// A recipe runs at its Bench and every bench in Also; an electric twin
// runs its base bench's recipes (Base). Recipes the wiki moved to a new
// bench keep their old one too, so existing benches and saved bills
// still work: the club and knife also at the crafting spot and the
// smithies (wiki: Club, Knife), the spear at the smithies (wiki: Spear),
// the crude rifle at the machining table (wiki: Bolt-action rifle),
// components and the servo tier at the fabrication bench (wiki:
// Component, Bionic), steel prosthetics at the machining table (wiki:
// Prosthetic leg) and butchering at the butcher spot (wiki: Butcher
// spot).

// The extra bench sets (read only, shared by the rows that use them).
var (
	smithy        = []construction.Kind{construction.FueledSmithy}
	spotAndSmithy = []construction.Kind{construction.CraftingSpot, construction.FueledSmithy}
	machining     = []construction.Kind{construction.MachiningTable}
	fabrication   = []construction.Kind{construction.FabricationBench}
)

// Base is the bench kind whose recipes a bench of kind k runs: an
// electric bench its plain or fueled twin (construction.RecipeBench, and
// the electric smithy the fueled one), any other kind itself.
func Base(k construction.Kind) construction.Kind {
	if k == construction.ElectricSmithy {
		return construction.FueledSmithy
	}
	return construction.RecipeBench(k)
}

// At reports a bench of kind k runs recipe d (its Bench or one in Also,
// through Base).
func (d Def) At(k construction.Kind) bool {
	k = Base(k)
	return d.Bench == k || slices.Contains(d.Also, k)
}

// smith is a stuffed melee weapon at the smithy (wiki: Mace, Gladius,
// Longsword: metal or wood, research Smithing or Long blades): stuff
// units of its material, steel by default.
func smith(id ID, name, label string, out item.Kind, stuff, w int) Def {
	return Def{ID: id, Name: name, Label: label, Bench: construction.FueledSmithy,
		Input:  Input{Kind: item.DefaultStuff(out), Qty: stuff, Stuff: item.MadeFrom(out)},
		Output: out, OutQty: 1, Work: w, Skill: work.Craft, Learn: skill.Crafting}
}

// wooden is a wooden ranged weapon (wiki: Short bow, Recurve bow,
// Greatbow at the crafting spot or a smithy; Pila at a smithy).
func wooden(id ID, name, label string, out item.Kind, bench construction.Kind, wood, w int, minSkill uint8) Def {
	d := Def{ID: id, Name: name, Label: label, Bench: bench, Input: Input{Kind: item.Wood, Qty: wood},
		Output: out, OutQty: 1, Work: w, Skill: work.Craft, Learn: skill.Crafting, MinSkill: minSkill}
	if bench == construction.CraftingSpot {
		d.Also = smithy
	}
	return d
}

// gun is a firearm at the machining table (wiki: Weapons, each gun's
// page): steel and components.
func gun(id ID, name, label string, out item.Kind, steel, comps, w int, minSkill uint8) Def {
	in := []Input{{Kind: item.Steel, Qty: steel}, {Kind: item.Components, Qty: comps}}
	return Def{ID: id, Name: name, Label: label, Bench: construction.MachiningTable, Input: in[0], Inputs: in,
		Output: out, OutQty: 1, Work: w, Skill: work.Craft, Learn: skill.Crafting, MinSkill: minSkill}
}

// sculpt is a sculpture carved at the sculpting bench from its kind's
// material count (construction Def Wood: wiki Sculpture 50/100/400
// stuff, scaled / 9 like every building) in wood, metal or stone.
// Sculpting trains Crafting: there is no Artistic skill.
func sculpt(id ID, name, label string, k construction.Kind) Def {
	cd := construction.DefOf(k)
	return Def{ID: id, Name: name, Label: label, Bench: construction.ArtBench,
		Input:  Input{Kind: item.Wood, Qty: cd.Wood, Stuff: item.Woody | item.Metallic | item.Stony},
		OutQty: 1, Work: cd.Work, Skill: work.Craft, Learn: skill.Crafting, Builds: k}
}

// SmeltPct is the share of its steel a smelted item gives back (wiki:
// Electric smelter, 25 %).
const SmeltPct = 25

// steelIn is the steel recipe d puts into one product made of material
// stuff (0 when none).
func (d Def) steelIn(stuff item.Kind) int {
	if d.Stuffed() {
		if stuff == item.Steel {
			return d.Input.Qty
		}
		return 0
	}
	for _, in := range d.AppendInputs(nil) {
		if in.Kind == item.Steel && in.Stuff == 0 && !in.AnyRaw && !in.AnyDryable && !in.AnyStoneBlocks && !in.Smelt &&
			!in.SmeltWear && !in.AnyVeg && !in.AnyMeat {
			return in.Qty
		}
	}
	return 0
}

// SmeltYield is the steel smelting stack s gives back: SmeltPct of the
// steel its recipe takes (at least 1), for a weapon or apparel only (0:
// no steel in it, or neither). A stack of several gives it for each.
func SmeltYield(s item.Stack) int {
	if !s.Kind.IsWeapon() && !s.Kind.IsApparel() {
		return 0
	}
	for _, d := range defs {
		if d.Output == s.Kind && d.Builds == 0 && !d.Legacy && d.OutQty > 0 {
			if n := d.steelIn(s.StuffOfStack()); n > 0 {
				return max(1, n*SmeltPct/100) * max(1, s.Qty)
			}
		}
	}
	return 0
}

// smeltable reports a kind some stack of could be smelted (a weapon, or
// with wear apparel, a recipe makes from steel, in some material).
func smeltable(k item.Kind, wear bool) bool {
	if wear && !k.IsApparel() || !wear && !k.IsWeapon() {
		return false
	}
	for _, d := range defs {
		if d.Output == k && d.Builds == 0 && (d.steelIn(item.Steel) > 0) {
			return true
		}
	}
	return false
}
