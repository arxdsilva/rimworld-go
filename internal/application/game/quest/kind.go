package quest

import qd "rimworld-go/internal/domain/quest"

// Kind is one quest kind: its rules and its reactions. One implementation
// per kind lives in quest/kinds/ (QST-2) and is passed to New. A kind
// changes only the quest it is handed; the root moves its state, pays its
// reward, posts its letters and keeps its timers and indexes.
type Kind interface {
	Def() qd.Def
	// CanOffer: the kind may be offered now (a giver exists, a tile is
	// free, ...).
	CanOffer(c Ctx) bool
	// Build fills q's Giver, Target, Parts and Rewards (ID, Kind, Seed,
	// Stars and Points are set); false skips this window.
	Build(c Ctx, q *qd.Quest) bool
	// Accepted runs once the quest is Active (accepted, or offered by an
	// AutoAccept kind): spawn sites, pawns and threats, set part timers.
	Accepted(c Ctx, q *qd.Quest)
	// Signal is the quest's reaction to a routed signal.
	Signal(c Ctx, q *qd.Quest, s Signal) Verdict
	// PartDue runs when Parts[part].At is due. The root marks the part
	// Done afterwards unless the kind moved At later.
	PartDue(c Ctx, q *qd.Quest, part int) Verdict
	// Ended cleans up after any end (succeeded, failed, expired): remove
	// the world site, release pawns.
	Ended(c Ctx, q *qd.Quest)
	// Detail is the quest's read-model lines (lang keys and args).
	Detail(q qd.Quest) []Line
}

// Line is one read-model line: a lang key and its arguments, never final
// text.
type Line struct {
	Key  string
	Args []string
}

// Verdict is what a kind decides about its quest: VerdictNone,
// VerdictSucceed, or VerdictFail(reason).
type Verdict uint16

const (
	// VerdictNone changes nothing.
	VerdictNone Verdict = 0
	// VerdictSucceed ends the quest as a success (an Active quest only).
	VerdictSucceed Verdict = 1
	verdictFail    Verdict = 2
)

// VerdictFail ends the quest as a failure for reason r (an offer expires
// with r instead, as qd.Fail does).
func VerdictFail(r qd.Reason) Verdict { return verdictFail | Verdict(r)<<8 }

// Succeeds and Fails tell the verdicts apart; Reason is a failure's reason.
func (v Verdict) Succeeds() bool    { return v&0xff == VerdictSucceed }
func (v Verdict) Fails() bool       { return v&0xff == verdictFail }
func (v Verdict) Reason() qd.Reason { return qd.Reason(v >> 8) }

// Ctx is what a kind gets on each call: the roles, the tick, and rolls
// from the one quest stream.
type Ctx struct {
	R   Roles
	Now uint64
	q   *Quests
}

// Intn is a roll in [0, n) from the quest stream; 0 when n <= 0 (no draw).
func (c Ctx) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return c.R.Rolls.Intn(n)
}

// Seed is a fresh seed from the quest stream.
func (c Ctx) Seed() uint64 { return c.R.Rolls.Seed() }

// AcceptsGoodwill: the player takes goodwill as a reward from faction ref
// (1 + index; QST-083). On unless SetPref turned it off.
func (c Ctx) AcceptsGoodwill(ref int) bool {
	return c.q == nil || !c.q.declines(ref-1)
}

// ThreatPoints is a threat part's points for q: q.Points × q.Stars × a
// 700..1300 permille roll (QST-026, QST-027).
func (c Ctx) ThreatPoints(q qd.Quest) int {
	return qd.ThreatPoints(q.Points, q.Stars, 700+c.Intn(601))
}

// Rewards picks q's reward options (QST-080..086) from pool: the budget
// from q.Points, the threat scale and the reward percent; goodwill when
// the giver takes it; pawn is a pawn option's seed (0: none).
func (c Ctx) Rewards(q qd.Quest, pool []qd.PoolItem, pawn uint64, siteLoot bool) []qd.Reward {
	w := c.R.World
	d := qd.DefOf(q.Kind)
	if c.q != nil {
		d = c.q.defOf(q.Kind)
	}
	in := qd.RewardIn{
		Budget:   qd.RewardBudget(q.Points, w.ThreatScalePct(), w.RewardPct()),
		Def:      d,
		Pool:     pool,
		Goodwill: q.Giver != 0 && c.AcceptsGoodwill(q.Giver),
		Pawn:     pawn,
		SiteLoot: siteLoot,
	}
	return qd.PickRewards(in, c.Intn)
}
