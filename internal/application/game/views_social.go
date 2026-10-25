package game

import "rimworld-go/internal/domain/sim/entity"

// Read models of the social and host systems (ENG-003 C3). The UI reads
// them through the methods below; they are copies, never live state. Until
// a milestone fills one it reports nothing (ok false, or empty).

// SocialView is one colonist's Social tab: relations and the social log.
type SocialView struct {
	Relations []RelationRowView
	Log       []SocialLogLineView
}

// RelationRowView is one person the colonist has an opinion of or a
// relation to. OpinionTo is the colonist's opinion of Other, OpinionFrom
// Other's of the colonist (points); the terms break them down.
type RelationRowView struct {
	Other                  uint64
	Name, Label, Status    string
	OpinionTo, OpinionFrom int
	TermsTo, TermsFrom     []OpinionTermView
	RomanceNote            string
}

// OpinionTermView is one line of an opinion breakdown (points).
type OpinionTermView struct {
	Label string
	Value int
}

// SocialLogLineView is one social log line; Icon is a ui_icon/ key suffix
// ("social_chitchat"); Weighty lines also go to the History.
type SocialLogLineView struct {
	Tick    uint64
	Text    string
	Icon    string
	Weighty bool
}

// HealthView is one colonist's Health tab.
type HealthView struct {
	Capacities []CapacityView
	Parts      []PartView
	Pain       int
	Operations []OperationView
	Drugs      []DrugLineView
	// Options are the operations the picker offers, each with why it
	// can't happen yet (SIM-049).
	Options []OperationOptionView
}

// OperationOptionView is one operation the Health tab can queue:
// Disabled options can still be queued (a bill may make the item later).
type OperationOptionView struct {
	Label    string
	Kind     uint8 // body.OpKind
	Part     uint8 // body.Part
	Item     uint8 // item.Kind (0: none)
	Disabled bool
	Reason   string
}

// CapacityView is one capacity ("Moving", 85 percent).
type CapacityView struct {
	Label string
	Pct   int
}

// PartView is one hurt, missing, scarred or artificial body part.
type PartView struct {
	Label   string
	Status  string
	HP      int
	Part    int    // body.Part (SIM-049)
	Implant string // fitted implant's name, "" if none
}

// OperationView is one queued operation.
type OperationView struct {
	Label  string
	Slot   int
	Status string // what it waits for ("no peg leg in stock"; SIM-049)
	// Blocked: the wait needs the player (no stock, research, reach, a
	// second doctor), not just time.
	Blocked bool
}

// DrugLineView is one drug effect, tolerance or addiction line.
type DrugLineView struct {
	Label string
	Pct   int
}

// PrisonerView is one prisoner's Prisoner tab; Mode is a prisoner.Mode.
type PrisonerView struct {
	Mode            uint8
	Resistance      int
	Warden          string
	ForPrisonersBed bool
}

// WorldMapView is the World tab: the colony's tile and its caravans.
type WorldMapView struct {
	Tile     int
	Caravans []CaravanView
}

// CaravanView is one caravan; Progress is the percent to its next tile.
type CaravanView struct {
	ID         uint64
	Members    []string
	Tile, Dest int
	Progress   int
	State      string
}

// AreaView is one allowed area.
type AreaView struct {
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

// HistoryEntryView is one History line.
type HistoryEntryView struct {
	Tick  uint64
	Text  string
	Kind  string
	Pawns []uint64
}

// Social is colonist id's Social tab (ok false: nothing to show yet).
func (g *Game) Social(id entity.EntityID) (SocialView, bool) {
	return g.socialView(id)
}

// Health is colonist id's Health tab (ok false: nothing to show yet).
func (g *Game) Health(id entity.EntityID) (HealthView, bool) {
	return g.healthView(id)
}

// Prisoner is prisoner id's tab (ok false: not a prisoner).
func (g *Game) Prisoner(id entity.EntityID) (PrisonerView, bool) {
	return g.prisonerView(id)
}

// WorldMap is the World tab.
func (g *Game) WorldMap() WorldMapView {
	return g.worldMapView()
}

// Areas lists the colony's allowed areas.
func (g *Game) Areas() []AreaView {
	return g.areaViews()
}

// History lists the colony's history, oldest first.
func (g *Game) History() []HistoryEntryView {
	return g.historyViews()
}
