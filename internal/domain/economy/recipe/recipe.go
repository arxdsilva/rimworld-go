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
	numRecipes
)

// Input is what a recipe consumes. AnyRaw accepts any raw food kind.
type Input struct {
	Kind   item.Kind
	AnyRaw bool
	Qty    int
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
}

var defs = [numRecipes]Def{
	SimpleMeal: {ID: SimpleMeal, Name: "simple_meal", Label: "Cook simple meal", Bench: construction.Stove,
		Input: Input{AnyRaw: true, Qty: 2}, Output: item.Meal, OutQty: 1, Work: 120, Skill: work.Cook},
	Club: {ID: Club, Name: "club", Label: "Make wooden club", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 4}, Output: item.Club, OutQty: 1, Work: 240, Skill: work.Craft},
	ButcherDustbuck: {ID: ButcherDustbuck, Name: "butcher_dustbuck", Label: "Butcher dustbuck", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseDustbuck, Qty: 1}, Output: item.Meat, OutQty: 12, Work: 200, Skill: work.Cook},
	ButcherRidgewolf: {ID: ButcherRidgewolf, Name: "butcher_ridgewolf", Label: "Butcher ridgewolf", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseRidgewolf, Qty: 1}, Output: item.Meat, OutQty: 6, Work: 200, Skill: work.Cook},
}

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
	if in.AnyRaw {
		return k.Raw()
	}
	return k == in.Kind
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
