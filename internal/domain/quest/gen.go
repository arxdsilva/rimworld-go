package quest

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
)

// The generator's rules (QST-020..039, QST-080..096). Signatures are fixed;
// D-dom writes the bodies and tests. Rolls are integers drawn by the
// caller; a func(n) returns a value in [0, n).

// WindowTicks is the offer cadence: one offer per 10-day window (QST-020).
// MinDay: no offer before day 6 (QST-022, ours).
const (
	WindowDays = 10
	MinDay     = 6
)

// OfferTick is the tick, relative to landing, of window w's one offer:
// window start + roll (roll in [0, WindowDays days)), never before MinDay.
func OfferTick(w int, roll uint64) uint64 { return 0 }

// WindowOf is the window index of a tick relative to landing.
func WindowOf(rel uint64) int { return 0 }

// RollStars picks 1..3 stars from roll in [0, 100) (QST-031, ours:
// 50/35/15).
func RollStars(roll int) uint8 { return 1 }

// ThreatPoints is a quest threat's points: points × stars × permille/1000
// (permille in [700, 1300], QST-026/027), after the floor, with no cap.
func ThreatPoints(points int, stars uint8, permille int) int { return points }

// JoinerWeight scales a joining kind's weight by colony size, in permille,
// from the colonist-event curve (0: 8.0, 1: 2.0, 5: 1.0, 9: 0.4, 12: 0)
// normalised so 5 colonists is 1000; 0 at 12 or more (QST-036).
func JoinerWeight(colonists int) int { return 0 }

// RewardBudget is a quest's reward value (QST-035, QST-085, ours): points
// de-scaled by threat scale, times rewardPct.
func RewardBudget(points, threatScalePct, rewardPct int) int { return 0 }

// GoodwillFor is the goodwill a goodwill-only option pays for budget
// (QST-093, ours: 1 per 50, at most 25, at least 1).
func GoodwillFor(budget int) int { return 0 }

// PoolItem is a reward candidate: a kind the giver can give.
type PoolItem struct {
	Kind item.Kind
	// Max is the most units one line may hold (0: the stack limit).
	Max int
}

// RewardIn is what PickRewards draws from.
type RewardIn struct {
	Budget int
	Def    Def
	// Pool is the candidate items (the giver's tech applied by the caller).
	Pool []PoolItem
	// Goodwill: the giver accepts goodwill as a reward (the preference,
	// QST-083, and a faction whose goodwill can move).
	Goodwill bool
	// Pawn is a pawn seed for a pawn option (0: none allowed).
	Pawn uint64
	// SiteLoot: the kind has loot at its site (2e).
	SiteLoot bool
}

// PickRewards builds 1 or 3 options (QST-080..086): random lines up to
// 5,500 of value, the rest paid in filler (gold, plasteel, uranium); a
// goodwill option pays goodwill alone unless Def.GoodwillPlusItems. A kind
// with Def.Choice offers 3 options half the time (ours).
func PickRewards(in RewardIn, roll func(n int) int) []Reward { return nil }

// Value is lines' market value.
func Value(lines []trade.Line) int { return 0 }
