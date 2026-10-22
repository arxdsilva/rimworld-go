package recipe

import (
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// ID identifies a recipe.
type ID uint8

const (
	SimpleMeal ID = iota
	Club
	// ButcherDustbuck and ButcherRidgewolf are legacy (older saves' bills):
	// ButcherAnimal butchers every species.
	ButcherDustbuck
	ButcherRidgewolf
	DryFood
	MakeParka
	MakeTuque
	MakeDuster
	CampfireMeal
	MakeRifle
	// Knife (stone) and Spear (wood): hand-made melee weapons.
	Knife
	Spear
	// Cremate burns a human corpse on a funeral pyre.
	Cremate
	// ButcherAnimal butchers any animal corpse; its yields come from the
	// species (SIM-019).
	ButcherAnimal
	numRecipes
)

// Input is what a recipe consumes. AnyRaw accepts any raw food kind;
// AnyDryable any perishable food that isn't dried yet. Qty is the least a
// stack must hold; Max, when set, takes up to that many from one stack.
type Input struct {
	Kind       item.Kind
	AnyRaw     bool
	AnyDryable bool
	Qty        int
	Max        int
}

// Take is how many items the input takes from a stack of qty.
func (in Input) Take(qty int) int {
	if in.Max > 0 {
		return min(qty, in.Max)
	}
	return in.Qty
}

// Def is one recipe.
type Def struct {
	ID     ID
	Name   string
	Label  string
	Bench  construction.Kind
	Input  Input
	Output item.Kind
	OutQty int
	Work   int
	Skill  work.Type // work type that does it
	// Extra is a by-product made alongside Output (leather from butchering).
	Extra    item.Kind
	ExtraQty int
	// BySpecies: the yields (Output meat, Extra leather) come from the
	// carcass's species, not OutQty/ExtraQty. Legacy recipes are kept for
	// older saves only and offered nowhere (Upgrade).
	BySpecies bool
	Legacy    bool
	// Dry recipes don't produce on the spot: the work lays the batch out on
	// the rack, which dries it over time into a dried stack of the same kind.
	Dry bool
}

var defs = [numRecipes]Def{
	SimpleMeal: {ID: SimpleMeal, Name: "simple_meal", Label: "Cook simple meal", Bench: construction.Stove,
		Input: Input{AnyRaw: true, Qty: 2}, Output: item.Meal, OutQty: 1, Work: 120, Skill: work.Cook},
	Club: {ID: Club, Name: "club", Label: "Make wooden club", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 4}, Output: item.Club, OutQty: 1, Work: 240, Skill: work.Craft},
	ButcherDustbuck: {ID: ButcherDustbuck, Name: "butcher_dustbuck", Label: "Butcher dustbuck", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseDustbuck, Qty: 1}, Output: item.Meat, OutQty: 12, Work: 200, Skill: work.Cook,
		Extra: item.Leather, ExtraQty: 5, Legacy: true},
	ButcherRidgewolf: {ID: ButcherRidgewolf, Name: "butcher_ridgewolf", Label: "Butcher ridgewolf", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseRidgewolf, Qty: 1}, Output: item.Meat, OutQty: 6, Work: 200, Skill: work.Cook,
		Extra: item.Leather, ExtraQty: 4, Legacy: true},
	ButcherAnimal: {ID: ButcherAnimal, Name: "butcher_animal", Label: "Butcher animal", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseAnimal, Qty: 1}, Output: item.Meat, Work: 200, Skill: work.Cook,
		Extra: item.Leather, BySpecies: true},
	// Cooking over an open fire is half again as slow as at a stove.
	CampfireMeal: {ID: CampfireMeal, Name: "simple_meal_campfire", Label: "Cook simple meal over the fire",
		Bench: construction.Campfire, Input: Input{AnyRaw: true, Qty: 2}, Output: item.Meal, OutQty: 1,
		Work: 180, Skill: work.Cook},
	// Tailoring: a parka takes the hides of about three deer.
	MakeParka: {ID: MakeParka, Name: "make_parka", Label: "Sew leather parka", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 14}, Output: item.Parka, OutQty: 1, Work: 900, Skill: work.Craft},
	MakeTuque: {ID: MakeTuque, Name: "make_tuque", Label: "Sew leather tuque", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 4}, Output: item.Tuque, OutQty: 1, Work: 300, Skill: work.Craft},
	MakeDuster: {ID: MakeDuster, Name: "make_duster", Label: "Sew leather duster", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 10}, Output: item.Duster, OutQty: 1, Work: 700, Skill: work.Craft},
	// A crude rifle: a long, careful job (needs Gunsmithing research).
	MakeRifle: {ID: MakeRifle, Name: "make_rifle", Label: "Make crude rifle", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 12}, Output: item.Rifle, OutQty: 1, Work: 1500, Skill: work.Craft},
	Knife: {ID: Knife, Name: "knife", Label: "Make stone knife", Bench: construction.Workbench,
		Input: Input{Kind: item.Stone, Qty: 3}, Output: item.Knife, OutQty: 1, Work: 200, Skill: work.Craft},
	Spear: {ID: Spear, Name: "spear", Label: "Make wooden spear", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 5}, Output: item.Spear, OutQty: 1, Work: 320, Skill: work.Craft},
	Cremate: {ID: Cremate, Name: "cremate", Label: "Cremate corpse", Bench: construction.Pyre,
		Input: Input{Kind: item.CorpseHuman, Qty: 1}, Work: 300, Skill: work.Craft,
		Extra: item.Wood, ExtraQty: 2},
	DryFood: {ID: DryFood, Name: "dry_food", Label: "Dry food", Bench: construction.DryingRack,
		Input: Input{AnyDryable: true, Qty: 1, Max: RackCapacity}, Work: 90, Skill: work.Cook, Dry: true},
}

// RackCapacity is how many items one drying rack holds.
const RackCapacity = 25

// DefOf returns the recipe definition.
func DefOf(id ID) Def {
	if id >= numRecipes {
		return Def{}
	}
	return defs[id]
}

// All lists every recipe.
func All() []Def { return defs[:] }

// Upgrade maps a legacy recipe to the one that replaced it.
func Upgrade(id ID) ID {
	if id == ButcherDustbuck || id == ButcherRidgewolf {
		return ButcherAnimal
	}
	return id
}

// For lists recipes a bench kind can run (legacy ones excluded).
func For(bench construction.Kind) []Def {
	var out []Def
	for _, d := range defs {
		if d.Bench == bench && !d.Legacy {
			out = append(out, d)
		}
	}
	return out
}

// Accepts reports whether stack kind k can feed input in.
func (in Input) Accepts(k item.Kind) bool {
	switch {
	case in.AnyRaw:
		return k.Raw()
	case in.AnyDryable:
		return k.Dryable()
	}
	return k == in.Kind
}

// AcceptsStack is Accepts plus per-stack state: a dry recipe skips stacks
// already dried or spoiled.
func (in Input) AcceptsStack(s item.Stack) bool {
	if in.AnyDryable {
		return s.Dryable()
	}
	return in.Accepts(s.Kind)
}

// Mode is how many times a bill repeats.
type Mode uint8

const (
	DoCount   Mode = iota // do Target times, then stop
	Forever               // repeat forever
	UntilHave             // repeat while the colony has fewer than Target outputs
)

func (m Mode) String() string {
	switch m {
	case Forever:
		return "forever"
	case UntilHave:
		return "until you have"
	default:
		return "do"
	}
}

// Bill is a standing work order on a bench.
type Bill struct {
	Recipe    ID
	Mode      Mode
	Target    int
	Done      int
	Suspended bool
	// Waiting: an "until you have" bill that reached its target and
	// rests until the stock falls to ResumeAt (absent in older saves).
	Waiting bool
}

// ResumeAt is the stock an "until you have" bill waits for after reaching
// its target, so one meal eaten doesn't send a cook back for one meal.
func (b Bill) ResumeAt() int {
	return b.Target - max(1, b.Target/5)
}

// Update latches the "until you have" wait: it starts at the target and
// ends once the stock falls to ResumeAt.
func (b Bill) Update(stock int) Bill {
	if b.Mode != UntilHave {
		b.Waiting = false
		return b
	}
	switch {
	case !b.Waiting && stock >= b.Target:
		b.Waiting = true
	case b.Waiting && stock <= b.ResumeAt():
		b.Waiting = false
	}
	return b
}

// Wants reports whether the bill should run now, given the colony's
// current stock of the output.
func (b Bill) Wants(stock int) bool {
	if b.Suspended {
		return false
	}
	switch b.Mode {
	case Forever:
		return true
	case UntilHave:
		return !b.Waiting && stock < b.Target
	default:
		return b.Done < b.Target
	}
}
