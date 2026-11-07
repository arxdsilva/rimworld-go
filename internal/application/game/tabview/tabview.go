// Package tabview is the read models of the inspect tabs fed by the social
// and host systems (ENG-003 C3): Social, Health, Prisoner, World, Areas and
// History. The game fills them and names them game.SocialView,
// game.HealthView and so on; the UI reads them through the game's methods.
// They are copies, never live state; until a milestone fills one it reports
// nothing (ok false, or empty). Plain data: no state, no host.
package tabview

import (
	"rimworld-go/internal/application/game/dose"
	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/application/game/travel"
)

// Social is one colonist's Social tab: relations and the social log.
type Social struct {
	Relations []RelationRow
	Log       []SocialLogLine
}

// RelationRow is one person the colonist has an opinion of or a
// relation to. OpinionTo is the colonist's opinion of Other, OpinionFrom
// Other's of the colonist (points); the terms break them down.
type RelationRow struct {
	Other                  uint64
	Name, Label, Status    string
	OpinionTo, OpinionFrom int
	TermsTo, TermsFrom     []OpinionTerm
	RomanceNote            string
}

// OpinionTerm is one line of an opinion breakdown (points).
type OpinionTerm struct {
	Label string
	Value int
}

// SocialLogLine is one social log line; Icon is a ui_icon/ key suffix
// ("social_chitchat"); Weighty lines also go to the History.
type SocialLogLine struct {
	Tick    uint64
	Text    string
	Icon    string
	Weighty bool
}

// Health is one colonist's Health tab.
type Health struct {
	Capacities []Capacity
	Parts      []Part
	Pain       int
	Operations []Operation
	Drugs      []dose.Line
	// Options are the operations the picker offers, each with why it
	// can't happen yet (SIM-049).
	Options []OperationOption
	// MED-00 contract, filled by the medical phases: State is the headline
	// ("Bleeding", "Infection: losing"; "" when fine); BloodLossPct the
	// blood lost and BleedPerDay the bleeding rate (percent of blood per
	// day); DeathInHours the hours left before bleeding out or a fatal
	// condition (-1: none coming); Conditions the condition rows; Care the
	// medical care policy (medcare.Policy) and SelfTend whether the pawn
	// may tend itself.
	State        string
	BloodLossPct int
	BleedPerDay  int
	DeathInHours int
	Conditions   []roster.ConditionView
	Care         uint8
	SelfTend     bool
}

// OperationOption is one operation the Health tab can queue:
// Disabled options can still be queued (a bill may make the item later).
type OperationOption struct {
	Label    string
	Kind     uint8 // body.OpKind
	Part     uint8 // body.Part
	Item     uint8 // item.Kind (0: none)
	Disabled bool
	Reason   string
}

// Capacity is one capacity ("Moving", 85 percent).
type Capacity struct {
	Label string
	Pct   int
	// Tip explains the value: each contributing part, weight and condition
	// on its own line under the capacity's name, ending in "= NN%".
	Tip string
}

// Part is one hurt, missing, scarred or artificial body part.
type Part struct {
	Label   string
	Status  string
	HP      int
	Part    int    // body.Part (SIM-049)
	Implant string // fitted implant's name, "" if none
	Bleed   int    // the part's bleeding, percent of blood per day (MED-01)
	// TendTip lists the part's open wounds with their tend quality, one
	// a line ("cut, 6 HP: tended 74%"; "" when none; MED-02).
	TendTip string
}

// Operation is one queued operation.
type Operation struct {
	Label  string
	Slot   int
	Status string // what it waits for ("no peg leg in stock"; SIM-049)
	// Blocked: the wait needs the player (no stock, research, reach, a
	// second doctor), not just time.
	Blocked bool
	// ChancePct is the success chance and ChanceTip its factor breakdown
	// (MED-00 contract; filled by MED-06).
	ChancePct int
	ChanceTip string
}

// Prisoner is one prisoner's Prisoner tab; Mode is a prisoner.Mode.
type Prisoner struct {
	Mode            uint8
	Resistance      int
	Warden          string
	ForPrisonersBed bool
}

// WorldMap is the World tab: the colony's tile and its caravans.
type WorldMap struct {
	Tile     int
	Caravans []travel.CaravanView
}

// Area is one allowed area.
type Area struct {
	Index int
	Name  string
	Cells int
	// PLAY-015: the stable id (AssignArea's Index), overlay colour, and
	// the member bitset (row-major over the map, shared read-only; Ver
	// changes whenever the area does).
	ID    uint16
	Color [4]uint8
	Bits  []uint64
	Ver   uint64
}

// HistoryEntry is one History line.
type HistoryEntry struct {
	Tick  uint64
	Text  string
	Kind  string
	Pawns []uint64
}
