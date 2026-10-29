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
	Conditions   []ConditionView
	Care         uint8
	SelfTend     bool
}

// ConditionView is one condition row of the Health tab: severity,
// immunity and tend quality in percent, Due when it needs tending now,
// and the tooltip text. MED-02: TendPct is the last tend's quality (0:
// never tended), DueInHours the whole hours until it needs its next tend
// (0 when Due or it takes no tending), Losing whether it is on course to
// kill before immunity wins.
type ConditionView struct {
	Label                   string
	SevPct, ImmPct, TendPct int
	Due                     bool
	Tip                     string
	DueInHours              int
	Losing                  bool
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
	// Tip explains the value: each contributing part, weight and condition
	// on its own line under the capacity's name, ending in "= NN%".
	Tip string
}

// PartView is one hurt, missing, scarred or artificial body part.
type PartView struct {
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

// OperationView is one queued operation.
type OperationView struct {
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
	ID      uint64
	Members []string
	// Health is each member's short health note, parallel to Members
	// ("bleeding", "flu", "food poisoning"; "" when well; WS-M).
	Health     []string
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
	hv, ok := g.healthView(id)
	if ok { // care/policy.go (MED-03): the care level and the self-tend toggle
		hv.Care, hv.SelfTend = uint8(g.care.Level(id)), !g.care.Policy.SelfTendOff(id)
	}
	return hv, ok
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
