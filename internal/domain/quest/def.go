package quest

// KindID names a quest kind. The ids are ours (clean room). Saved:
// append-only.
type KindID uint8

const (
	KindNone KindID = iota
	// Joiner: a threatened pawn joins on accept; a threat follows (2a).
	Joiner
	// OreLump: reach a scanner's ore site in time (2b).
	OreLump
	// Stash: reach a guarded stash of goods (2c).
	Stash
	// Refugee: reach a downed pawn at a site and help them (2d).
	Refugee
	// Camp: clear a hostile camp for a faction (2e).
	Camp
	// Rescue: free a prisoner held in a camp (2f).
	Rescue
	// Request: bring goods to a faction's settlement by caravan (2g).
	Request
	// Talks: hold peace talks with a hostile faction (2h).
	Talks
	// NumKinds counts the kinds.
	NumKinds
)

// Pay is when a quest's chosen reward is given.
type Pay uint8

const (
	// OnSuccess: when the quest succeeds.
	OnSuccess Pay = iota
	// OnAccept: at once on accept (the joiner, QST-008).
	OnAccept
)

// Delivery is where a quest's reward arrives (QST-089..091).
type Delivery uint8

const (
	// Home: by pod at the home colony.
	Home Delivery = iota
	// IntoCaravan: into the caravan that finished it (2g, 2h).
	IntoCaravan
	// AtSite: the reward is what lies at the site (2c).
	AtSite
)

// Def is a kind's fixed rules. Values marked "shots" are measured at 500
// points; the rest are "ours" unless a row says otherwise.
type Def struct {
	Kind KindID
	// Weight is the kind's base pick weight (ours, QST-037).
	Weight int
	// Joins: a pawn joins through it; its weight follows the colony-size
	// curve (QST-036).
	Joins bool
	// AutoAccept: the quest is Active from the start (QST-004).
	AutoAccept bool
	// AcceptTicks is the offer's window (0 with AutoAccept).
	AcceptTicks uint64
	// DurationTicks is the completion window from accept (0: none).
	DurationTicks uint64
	// RewardAt and Delivery: when and where the reward is paid.
	RewardAt Pay
	Delivery Delivery
	// Violent kinds are not offered when quest threats are off (QST-033).
	Violent bool
	// Choice: the kind may offer three reward options (QST-080).
	Choice bool
	// GoodwillPlusItems: a goodwill option may also carry items (2e only).
	GoodwillPlusItems bool
	// FailOnGiverHostile: the quest fails if the giver turns hostile
	// (shots Q38-Q40: per kind).
	FailOnGiverHostile bool
	// HasTarget: the kind fights a hostile faction (fails when it is no
	// longer hostile, QST-011).
	HasTarget bool
}

// Defs is every kind's Def, indexed by KindID (KindNone's is zero). A new
// slice each call: there is no package state.
func Defs() []Def {
	return defs()
}

// DefOf is kind k's Def; an unknown kind gives the zero Def.
func DefOf(k KindID) Def {
	d := defs()
	if int(k) >= len(d) {
		return Def{}
	}
	return d[k]
}

// defs builds the table. D-dom fills it from the gap file's rows
// (QST-040..079 and the shots: accept and completion windows).
func defs() []Def {
	return make([]Def, NumKinds)
}
