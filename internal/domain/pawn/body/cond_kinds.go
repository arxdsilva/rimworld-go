package body

import "fmt"

// Condition kinds and their definitions (MED-00).
//
// Registration is init-free and deterministic. A phase that adds
// conditions:
//  1. appends its kinds to the const block below, before NumCondKinds
//     (append only: the number is saved);
//  2. writes their definitions as a []CondEntry table in its own file
//     (cond_<phase>.go), e.g. var condsDisease = []CondEntry{{Flu, ...}};
//  3. names that table once in condTables.
//
// condDefs is a fixed array built from condTables during package variable
// initialisation, which the Go spec orders by dependency: no init funcs,
// no maps, no file-order effects. A kind defined twice, CondNone or an
// out-of-range kind panics at start-up, so any test of the package catches
// it; TestEveryCondKindHasDef catches a kind left without a definition.
const (
	// CondNone is no condition.
	CondNone CondKind = iota
	// CondInfection is a wound infection on a part (MED-02, cond_infection.go).
	CondInfection
	// NumCondKinds counts the kinds (keep last).
	NumCondKinds
)

// CondEntry is one kind's definition in a phase's table.
type CondEntry struct {
	Kind CondKind
	Def  CondDef
}

// condTables lists every phase's definition table (one line per phase).
var condTables = [][]CondEntry{condsInfection}

// condDefs is every kind's definition, indexed by kind.
var condDefs = buildCondDefs(condTables)

// buildCondDefs merges the tables into one array indexed by kind.
func buildCondDefs(tables [][]CondEntry) [NumCondKinds]CondDef {
	var out [NumCondKinds]CondDef
	seen := [NumCondKinds]bool{}
	for _, t := range tables {
		for _, e := range t {
			if e.Kind == CondNone || e.Kind >= NumCondKinds {
				panic(fmt.Sprintf("body: condition kind %d out of range", e.Kind))
			}
			if seen[e.Kind] {
				panic(fmt.Sprintf("body: condition kind %d defined twice", e.Kind))
			}
			seen[e.Kind] = true
			out[e.Kind] = e.Def
		}
	}
	return out
}
