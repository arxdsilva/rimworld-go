// Package scenario is how a colony starts: who arrives, how, with what,
// and the special rules that last the whole game. Built-in scenarios plus
// custom ones from the editor, encoded as versioned JSON (NG-1 §3.1, §4.4).
package scenario

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
)

// Arrival is how the colonists reach the map.
type Arrival uint8

const (
	DropPods Arrival = iota
	Standing
	WalkIn
)

// PartKind is an optional scenario part (append only, saved in files).
type PartKind uint8

const (
	PartNone PartKind = iota
	PartForcedTrait
	PartAgeRange
	PartForcedCondition
	PartNoClothes
	PartStartItem
	PartScatterItem
	PartStartAnimal
	PartCondition
	PartScheduledIncident
	PartDisabledIncident
	PartDisallowBuilding
	PartResearch
	PartResearchCost
	PartIntro
	NumPartKinds
)

// Who a forced part applies to.
type Who uint8

const (
	WhoAll Who = iota
	WhoOneRandom
	WhoNRandom
)

// Condition is a permanent map/world condition.
type Condition uint8

const (
	NoCondition Condition = iota
	AshfallSky
	EndlessChill
	LongSummer
	StillAir
)

// Part is one optional rule (a tagged struct; unused fields are zero).
type Part struct {
	Kind      PartKind
	Trait     trait.ID
	Chance    uint8
	Who       Who
	N         uint8
	MinAge    uint8
	MaxAge    uint8
	PawnCond  pawngen.Condition
	Item      item.Kind
	Qty       int32
	Near      bool
	Species   animal.Species
	Cond      Condition
	Incident  string
	FirstDay  uint16
	EveryDays uint16
	Building  construction.Kind
	Project   research.Project
	Percent   uint16
	Text      string
}

// Def is a scenario.
type Def struct {
	Key, Name, Summary, Desc string
	Builtin                  bool
	Seed                     uint64
	Faction                  faction.Type
	People, ChooseFrom       int
	Arrival                  Arrival
	Tech                     faction.Tech
	Hardness                 uint8
	Parts                    []Part
}

// Severity of a validation problem.
type Severity uint8

const (
	Error Severity = iota
	Warning
)

// Problem is a validation finding; Part is the index into Parts, -1 for the
// mandatory fields.
type Problem struct {
	Part     int
	Severity Severity
	Msg      string
}

// Stack is a starting or scattered item stack.
type Stack struct {
	Kind item.Kind
	Qty  int
	Near bool
}

// Sched is a scheduled incident.
type Sched struct {
	Incident            string
	FirstDay, EveryDays int
}

// Effects is a scenario flattened for the game.
type Effects struct {
	People, ChooseFrom int
	Arrival            Arrival
	Tech               faction.Tech
	Faction            faction.Type
	StartItems         []Stack
	Scatter            []Stack
	Animals            []animal.Species
	Research           []research.Project
	ResearchCostPct    int
	Conditions         []Condition
	Scheduled          []Sched
	Disabled           map[string]bool
	Disallowed         map[construction.Kind]bool
	Rules              pawngen.Rules
	Intro              string
}

// Builtins are the four built-in scenarios in display order.
func Builtins() []Def { return nil } // NG-1 stub (RULES)

// Validate checks d (errors block, warnings don't).
func (d Def) Validate() []Problem { return nil } // NG-1 stub (RULES)

// Apply flattens d for the game.
func (d Def) Apply() Effects { return Effects{} } // NG-1 stub (RULES)

// Encode and Decode are the versioned JSON file format.
func Encode(d Def) ([]byte, error) { return nil, nil }   // NG-1 stub (RULES)
func Decode(b []byte) (Def, error) { return Def{}, nil } // NG-1 stub (RULES)

// Label names a part kind; Kinds lists the addable kinds by category.
func (k PartKind) Label() string { return "" } // NG-1 stub (RULES)
