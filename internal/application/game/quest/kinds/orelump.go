package kinds

import (
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/trade"
	qd "rimworld-go/internal/domain/quest"
)

// OreLump is the long-range scanner's ore lump site (QST-045..049, 116).
// The cadence never offers it (Weight 0): the scanner calls OfferAt right
// after it lays a mineral site. The quest mirrors that site: one site part
// on its tile holding the lump, accepted at once, 30 days to bring a
// caravan there. The scanner's travel site stays the world object and the
// game drops it on expiry, so this kind never places or removes a site.
type OreLump struct{}

// Def is the kind's rules.
func (OreLump) Def() qd.Def { return qd.DefOf(qd.OreLump) }

// CanOffer: only for a scanner tile that holds a deposit.
func (OreLump) CanOffer(c quest.Ctx) bool {
	if c.Tile < 0 {
		return false
	}
	_, ok := c.R.Sites.Deposit(c.Tile)
	return ok
}

// Build: one site part carrying the lump; no giver, no reward options (the
// ore is the reward).
func (OreLump) Build(c quest.Ctx, q *qd.Quest) bool {
	if c.Tile < 0 {
		return false
	}
	lump, ok := c.R.Sites.Deposit(c.Tile)
	if !ok || lump.Qty <= 0 {
		return false
	}
	q.Giver, q.Target = 0, 0
	q.Rewards = nil
	q.Parts = []qd.Part{{Kind: qd.PartSite, Tile: c.Tile, Lines: []trade.Line{lump}}}
	return true
}

// Accepted: nothing to spawn; the scanner owns the site.
func (OreLump) Accepted(quest.Ctx, *qd.Quest) {}

// Signal: a caravan arriving on the site tile succeeds the quest.
func (OreLump) Signal(_ quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	if s.Kind != quest.SigCaravanArrived || len(q.Parts) == 0 || q.Parts[0].Done || s.Tile != q.Parts[0].Tile {
		return quest.VerdictNone
	}
	q.Parts[0].Done = true
	return quest.VerdictSucceed
}

// PartDue: the site part has no timer.
func (OreLump) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict { return quest.VerdictNone }

// Ended: nothing to clean up; the game drops the site itself.
func (OreLump) Ended(quest.Ctx, *qd.Quest) {}

// Detail: the mineral, the amount and the tile a caravan must reach.
func (OreLump) Detail(q qd.Quest) []quest.Line {
	if len(q.Parts) == 0 || len(q.Parts[0].Lines) == 0 {
		return nil
	}
	p := q.Parts[0]
	l := p.Lines[0]
	return []quest.Line{
		{Key: "quest.orelump.lump", Args: []quest.Arg{quest.Item(l.Kind), quest.Num(l.Qty)}},
		{Key: "quest.orelump.reach", Args: []quest.Arg{quest.Tile(p.Tile)}},
	}
}
