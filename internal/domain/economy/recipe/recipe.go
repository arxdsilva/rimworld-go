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
	ButcherDustbuck
	ButcherRidgewolf
	DryFood
	MakeParka
	MakeTuque
	MakeDuster
	CampfireMeal
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
		Extra: item.Leather, ExtraQty: 5},
	ButcherRidgewolf: {ID: ButcherRidgewolf, Name: "butcher_ridgewolf", Label: "Butcher ridgewolf", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseRidgewolf, Qty: 1}, Output: item.Meat, OutQty: 6, Work: 200, Skill: work.Cook,
		Extra: item.Leather, ExtraQty: 4},
	// Cooking over an open fire is half again as slow as at a stove.
	CampfireMeal: {ID: CampfireMeal, Name: "simple_meal_campfire", Label: "Cook simple meal over the fire",
		Bench: construction.Campfire, Input: Input{AnyRaw: true, Qty: 2}, Output: item.Meal, OutQty: 1,
		Work: 180, Skill: work.Cook},
	// Tailoring: a parka takes the hides of about three dustbucks.
	MakeParka: {ID: MakeParka, Name: "make_parka", Label: "Sew leather parka", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 14}, Output: item.Parka, OutQty: 1, Work: 900, Skill: work.Craft},
	MakeTuque: {ID: MakeTuque, Name: "make_tuque", Label: "Sew leather tuque", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 4}, Output: item.Tuque, OutQty: 1, Work: 300, Skill: work.Craft},
	MakeDuster: {ID: MakeDuster, Name: "make_duster", Label: "Sew leather duster", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 10}, Output: item.Duster, OutQty: 1, Work: 700, Skill: work.Craft},
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

// For lists recipes a bench kind can run.
func For(bench construction.Kind) []Def {
	var out []Def
	for _, d := range defs {
		if d.Bench == bench {
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
		return stock < b.Target
	default:
		return b.Done < b.Target
	}
}
