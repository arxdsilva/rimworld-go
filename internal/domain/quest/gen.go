package quest

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/clock"
)

// The generator's rules (QST-020..039, QST-080..096). Signatures are fixed;
// rolls are integers drawn by the caller; a func(n) returns a value in
// [0, n).

// WindowTicks is the offer cadence: one offer per 10-day window (QST-020).
// MinDay: no offer before day 6 (QST-022, ours).
const (
	WindowDays = 10
	MinDay     = 6
)

// OfferTick is the tick, relative to landing, of window w's one offer:
// window start + roll (roll in [0, WindowDays days)), never before MinDay.
//
// The roll is taken modulo the window. Window 0 maps it linearly onto
// [MinDay, WindowDays) so its offer stays uniform and inside the window;
// a negative w is window 0. Every window has exactly one offer, so two
// offers are less than 20 days apart (19.9 at the reference's rounding).
func OfferTick(w int, roll uint64) uint64 {
	const span = uint64(WindowDays * clock.TicksPerDay)
	if w < 0 {
		w = 0
	}
	roll %= span
	if w == 0 {
		const first = uint64(MinDay * clock.TicksPerDay)
		return first + roll*(span-first)/span
	}
	return uint64(w)*span + roll
}

// WindowOf is the window index of a tick relative to landing.
func WindowOf(rel uint64) int {
	return int(rel / uint64(WindowDays*clock.TicksPerDay))
}

// RollStars picks 1..3 stars from roll in [0, 100) (QST-031, ours:
// 50/35/15).
// A roll below 0 counts as 0, one of 100 or more as 99.
func RollStars(roll int) uint8 {
	switch {
	case roll < 50:
		return 1
	case roll < 85:
		return 2
	}
	return 3
}

// ThreatPoints is a quest threat's points: points × stars × permille/1000
// (permille in [700, 1300], QST-026/027), after the floor, with no cap.
// Stars outside 1..3 and a permille outside [700, 1300] are clamped;
// negative points are 0.
func ThreatPoints(points int, stars uint8, permille int) int {
	if points <= 0 {
		return 0
	}
	s := min(max(int(stars), 1), 3)
	permille = min(max(permille, 700), 1300)
	return points * s * permille / 1000
}

// JoinerWeight scales a joining kind's weight by colony size, in permille,
// from the colonist-event curve (0: 8.0, 1: 2.0, 5: 1.0, 9: 0.4, 12: 0)
// normalised so 5 colonists is 1000; 0 at 12 or more (QST-036).
// Linear between the anchors: 0 → 8000, 1 → 2000, 5 → 1000, 9 → 400,
// 12 → 0. A negative count is 0 colonists.
func JoinerWeight(colonists int) int {
	type anchor struct{ n, w int }
	curve := [...]anchor{{0, 8000}, {1, 2000}, {5, 1000}, {9, 400}, {12, 0}}
	if colonists <= 0 {
		return curve[0].w
	}
	for i := 1; i < len(curve); i++ {
		a, b := curve[i-1], curve[i]
		if colonists <= b.n {
			return a.w + (b.w-a.w)*(colonists-a.n)/(b.n-a.n)
		}
	}
	return 0
}

// RewardBudget is a quest's reward value (QST-035, QST-085, ours): points
// de-scaled by threat scale, times rewardPct.
//
// Ours: budget = points × 2.2 × 100 / max(threatScalePct, 10) ×
// rewardPct / 100, in integers. The 2.2 is calibrated on shot Q24 (+22
// goodwill at 500 points, threat scale 100 %, rewards 100 %: a budget of
// 1,100 at 1 goodwill per 50). The floor of 10 % keeps a near-zero threat
// scale from blowing the budget up. Negative inputs give 0.
func RewardBudget(points, threatScalePct, rewardPct int) int {
	if points <= 0 || rewardPct <= 0 {
		return 0
	}
	const budgetPct = 220
	return points * budgetPct * rewardPct / (100 * max(threatScalePct, 10))
}

// GoodwillFor is the goodwill a goodwill-only option pays for budget
// (QST-093, ours: 1 per 50, at most 25, at least 1).
func GoodwillFor(budget int) int {
	return min(max(budget/goodwillRate, 1), maxGoodwill)
}

// goodwillRate is the silver of budget per goodwill point and maxGoodwill
// the most one option pays (QST-093, ours).
const (
	goodwillRate = 50
	maxGoodwill  = 25
)

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
// with Def.Choice offers 3 options half the time (ours). The rest of the
// rules are in reward.go.
func PickRewards(in RewardIn, roll func(n int) int) []Reward {
	n := 1
	if in.Def.Choice && roll(2) == 1 {
		n = 3
	}
	return pickOptions(in, n, roll)
}

// Value is lines' market value.
func Value(lines []trade.Line) int {
	v := 0
	for _, l := range lines {
		v += l.Kind.Value() * l.Qty
	}
	return v
}
