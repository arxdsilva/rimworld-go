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
}

// CapacityView is one capacity ("Moving", 85 percent).
type CapacityView struct {
	Label string
	Pct   int
}

// PartView is one hurt, missing, scarred or artificial body part.
type PartView struct {
	Label  string
	Status string
	HP     int
}

// OperationView is one queued operation.
type OperationView struct {
	Label string
	Slot  int
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
	return SocialView{}, false // TODO(M4)
}

// Health is colonist id's Health tab (ok false: nothing to show yet).
func (g *Game) Health(id entity.EntityID) (HealthView, bool) {
	return HealthView{}, false // TODO(M1)
}

// Prisoner is prisoner id's tab (ok false: not a prisoner).
func (g *Game) Prisoner(id entity.EntityID) (PrisonerView, bool) {
	return PrisonerView{}, false // TODO(M8)
}

// WorldMap is the World tab.
func (g *Game) WorldMap() WorldMapView {
	return WorldMapView{} // TODO(M9)
}

// Areas lists the colony's allowed areas.
func (g *Game) Areas() []AreaView {
	return nil // TODO(M9)
}

// History lists the colony's history, oldest first.
func (g *Game) History() []HistoryEntryView {
	return nil // TODO(M4)
}
