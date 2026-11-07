// Package quest is the quest model (QST-1, .agents/gaps/QUESTS_GAP_ANALYSIS.md):
// a quest's states and timers, the kind definitions, threat points,
// reward budgets and options, the offer cadence and the peace-talk odds.
//
// It is pure: no RNG source and no clock. Callers pass ticks and rolls, so
// the same inputs always give the same quest.
package quest

import "rimworld-go/internal/domain/economy/trade"

// ID is a quest's id: issued from 1 upward, never reused. 0 is none.
type ID uint32

// State is where a quest is in its life. Saved: append-only.
type State uint8

const (
	// Offered: in the Available tab, waiting for Accept until AcceptBy.
	Offered State = iota
	// Active: accepted (or accepted itself); runs until Deadline, if any.
	Active
	// Succeeded, Failed and Expired are terminal: nothing moves them.
	Succeeded
	Failed
	// Expired: an offer whose accept window ran out (QST-002).
	Expired
)

// Terminal reports whether s never changes again.
func (s State) Terminal() bool { return s >= Succeeded }

// Reason says why a quest failed or expired. Saved: append-only.
type Reason uint8

const (
	ReasonNone Reason = iota
	// Timeout: the completion window ran out (QST-009), or an offer's
	// accept window did (Expired).
	Timeout
	// GiverHostile: the giver's faction turned hostile (QST-010).
	GiverHostile
	// TargetNotHostile: the faction to fight is no longer hostile (QST-011).
	TargetNotHostile
	// PawnLost: the pawn the quest is about died or left (2d, 2f).
	PawnLost
	// SiteLost: the quest's site was destroyed or abandoned.
	SiteLost
)

// Event is what a quest's next timer does (Due).
type Event uint8

const (
	EvNone Event = iota
	// EvAcceptEnd: an offer's accept window ends (Expire).
	EvAcceptEnd
	// EvDeadline: an active quest's completion window ends.
	EvDeadline
	// EvPart: a part's own timer (Part.At), e.g. a threat's arrival.
	EvPart
)

// Quest is one quest, as kept and saved. Factions are "1 + index, 0 none",
// the repo's saved-reference convention (maps.Meta, story.go).
type Quest struct {
	ID     ID
	Kind   KindID
	State  State
	Reason Reason
	// Giver is the faction that offers it; Target the faction to fight.
	Giver, Target int
	// Stars is the difficulty, 1..3 (QST-026, QST-031).
	Stars uint8
	// Points are the threat points the quest was built with, before stars.
	Points int
	// OfferedAt, AcceptedAt and EndedAt are ticks (0: not yet).
	OfferedAt, AcceptedAt, EndedAt uint64
	// AcceptBy ends an offer's window; Deadline an active quest's (0: none).
	AcceptBy, Deadline uint64
	Parts              []Part
	// Rewards are the options; Chosen is the index accepted (-1: none yet).
	Rewards []Reward
	Chosen  int8
	// Paid: the chosen reward was given. A paid reward is never taken
	// back (QST-008).
	Paid bool
	// Dismissed hides the quest under Historical; it changes no state or
	// timer, and a dismissed offer can still be accepted (QST-006).
	Dismissed bool
	// Seed is the quest's own seed: sites and pawns derive from it.
	Seed uint64
}

// PartKind tags a Part. Saved: append-only.
type PartKind uint8

const (
	PartNone PartKind = iota
	// PartSite: a world tile with a site map (stash, refugee, camp, cell,
	// ore lump, talks).
	PartSite
	// PartPawn: a pawn the quest is about (joiner, refugee, prisoner).
	PartPawn
	// PartThreat: a threat that arrives at Part.At with Part.Points.
	PartThreat
	// PartDelivery: goods to bring to a settlement (trade request).
	PartDelivery
	// PartTalk: a talk at a settlement or site (peace talks).
	PartTalk
)

// Part is one piece of a quest. One flat struct, so it saves simply; each
// kind reads only the fields its PartKind uses.
type Part struct {
	Kind PartKind
	// Tile is a world tile (site, delivery, talk); -1 none.
	Tile int
	// Map is the site map's id once open (0: none).
	Map uint32
	// Layout is the site's layout (kind-defined; 0 plain).
	Layout uint8
	// Pawn is a bound pawn's entity id (0: not spawned yet).
	Pawn uint64
	// Seed generates the part's pawn or site.
	Seed uint64
	// Lines are goods: requested (delivery) or placed (stash).
	Lines []trade.Line
	// Threat is the threat's kind (kind-defined); Points its points.
	Threat uint8
	Points int
	// At is the part's own timer tick (0: none), e.g. a threat's arrival.
	At uint64
	// Count is a kind's counter, e.g. a camp's hostiles at populate.
	Count int
	// Done marks a part finished (a threat fired, a site reached).
	Done bool
}

// Reward is one "accept for" option: everything it pays (QST-128). Most
// options carry one thing; the hostile camp may pair goodwill with its
// site loot (QST-082).
type Reward struct {
	// Lines are item rewards (silver is an item).
	Lines []trade.Line
	// Goodwill with the giver.
	Goodwill int
	// Pawn is the seed of a pawn who joins (0: none).
	Pawn uint64
	// SiteLoot: the loot at the quest's site is part of the deal.
	SiteLoot bool
	// Value is the option's market value, as shown.
	Value int
}

// GoodwillOnly reports whether r pays goodwill and nothing else.
func (r Reward) GoodwillOnly() bool {
	return r.Goodwill != 0 && len(r.Lines) == 0 && r.Pawn == 0 && !r.SiteLoot
}
