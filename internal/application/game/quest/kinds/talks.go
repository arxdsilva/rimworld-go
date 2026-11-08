package kinds

import (
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
)

// Talks is the peace talks with a hostile faction (QST-075..079, 096, 119).
//
// A hostile faction that could be friends (never pirates or a fixed-hostile
// faction) asks for talks at a site 6-25 tiles from home (decision 8). The
// quest accepts itself and pays no reward rows (Q16): the talks' outcome is
// the reward. It lives a hidden 12-28 days (its one part's timer). The
// talks are held once, when a caravan arrives on the tile or is ordered to
// negotiate there; its best Social member talks (qd.Negotiate).
//
// Every held talk ends the quest as Succeeded, whatever the outcome: the
// quest's task is to hold the talks, and their result (good or bad) is the
// quest's reward. Historical tells the outcomes apart through Detail. The
// quest fails only when the talks never happen: Timeout when the lifetime
// runs out, SiteLost when the site is lost (or could not be placed, or the
// host faction is defeated).
//
// The talk part keeps the record: Count is talkPending, talkNoSite or the
// held result + 1; Points the goodwill change; Lines the gift.
type Talks struct{}

const (
	talkMinTiles, talkMaxTiles = 6, 25
	talkMinDays, talkMaxDays   = 12, 28

	// talkPending: the talks have not been held; talkNoSite: the site was
	// refused, the quest fails at its next timer. A held talk is result + 1.
	talkPending = 0
	talkNoSite  = -1
)

// giftPool is the valuable a triumph's gift leads with (ours); silver pays
// the rest of its value.
func giftPool() [4]item.Kind {
	return [...]item.Kind{item.Gold, item.Jade, item.Components, item.IndustrialMedicine}
}

func (Talks) Def() qd.Def { return qd.DefOf(qd.Talks) }

// CanOffer: some faction may host talks.
func (Talks) CanOffer(c quest.Ctx) bool { return len(talkHosts(c)) > 0 }

// Build picks the host among the candidates, the site tile and the hidden
// lifetime. No free tile: nothing is offered.
func (Talks) Build(c quest.Ctx, q *qd.Quest) bool {
	hosts := talkHosts(c)
	if len(hosts) == 0 {
		return false
	}
	host := hosts[c.Intn(len(hosts))]
	seed := c.Seed()
	tile, ok := c.R.World.NearTile(seed, talkMinTiles, talkMaxTiles)
	if !ok {
		return false
	}
	days := uint64(talkMinDays + c.Intn(talkMaxDays-talkMinDays+1))
	q.Giver, q.Target = host+1, 0
	q.Rewards = nil
	q.Parts = []qd.Part{{Kind: qd.PartTalk, Tile: tile, Seed: seed, At: c.Now + days*clock.TicksPerDay}}
	return true
}

// talkHosts are the faction indexes that may host talks (QST-075):
// hostile, able to be friends, not defeated, not besieged. In index order.
func talkHosts(c quest.Ctx) []int {
	var out []int
	for _, f := range c.R.World.Factions() {
		if f.Relation == faction.Hostile && f.CanBeFriendly && !f.Defeated && !f.Besieged {
			out = append(out, f.Index)
		}
	}
	return out
}

// Accepted places the talks site, owned by the host. A tile that refuses
// it fails the quest at the next timer (SiteLost) instead of leaving talks
// nobody can reach.
func (Talks) Accepted(c quest.Ctx, q *qd.Quest) {
	p := talkPart(q)
	if p == nil {
		return
	}
	s := quest.Site{Quest: q.ID, Kind: q.Kind, Tile: p.Tile, Seed: p.Seed, Faction: q.Giver}
	if !c.R.Sites.Place(s) {
		p.Count, p.At = talkNoSite, c.Now+1
	}
}

// Signal holds the talks when a caravan reaches the tile or is ordered to
// negotiate there, and fails the quest when the site or its host is lost.
func (Talks) Signal(c quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	p := talkPart(q)
	if p == nil || p.Count != talkPending {
		return quest.VerdictNone
	}
	switch s.Kind {
	case quest.SigCaravanArrived:
		if s.Tile == p.Tile {
			return talk(c, q, p, s.Caravan)
		}
	case quest.SigNegotiate:
		if t, ok := c.R.Items.CaravanAt(s.Caravan); ok && t == p.Tile {
			return talk(c, q, p, s.Caravan)
		}
	case quest.SigSiteLost:
		if s.Tile == p.Tile {
			return quest.VerdictFail(qd.SiteLost)
		}
	case quest.SigFactionDefeated:
		if s.Faction == q.Giver {
			return quest.VerdictFail(qd.SiteLost)
		}
	}
	return quest.VerdictNone
}

// talk holds the talks with caravan: nothing when nobody in it can talk.
// The negotiator gains the Social XP whatever happens (QST-076); goodwill
// moves inside the outcome's range (QST-077); a triumph's gift goes into
// the caravan (QST-078); a disaster ambushes it at once (QST-079).
func talk(c quest.Ctx, q *qd.Quest, p *qd.Part, caravan uint64) quest.Verdict {
	if caravan == 0 {
		return quest.VerdictNone
	}
	who, level, ok := c.R.Pawns.BestSocial(caravan)
	if !ok {
		return quest.VerdictNone
	}
	out := qd.Negotiate(level, c.Intn(1000), c.Intn(1000))
	host := q.Giver - 1
	if out.Goodwill != 0 {
		c.R.Goodwill.Adjust(host, out.Goodwill)
	}
	if out.Gift > 0 {
		p.Lines = giftLines(c, out.Gift)
		c.R.Items.IntoCaravan(caravan, p.Lines)
	}
	c.R.Pawns.GrantXP(who, skill.Social, qd.TalkXP)
	if out.Ambush {
		c.R.Threats.Ambush(caravan, host, c.ThreatPoints(*q), c.Seed())
	}
	p.Count, p.Points, p.Done = int(out.Result)+1, out.Goodwill, true
	return quest.VerdictSucceed
}

// giftLines are goods worth value: a 40-80 % share in one pool valuable,
// the rest in silver, in lines of at most a stack.
func giftLines(c quest.Ctx, value int) []trade.Line {
	pool := giftPool()
	k := pool[c.Intn(len(pool))]
	var lines []trade.Line
	if qty := value * (40 + c.Intn(41)) / 100 / k.Value(); qty > 0 {
		lines = appendLines(lines, k, qty)
	}
	if rest := value - qd.Value(lines); rest > 0 {
		v := item.Silver.Value()
		lines = appendLines(lines, item.Silver, (rest+v-1)/v)
	}
	return lines
}

// appendLines appends qty of k in lines of at most k's stack limit.
func appendLines(lines []trade.Line, k item.Kind, qty int) []trade.Line {
	limit := max(k.StackLimit(), 1)
	for qty > 0 {
		n := min(qty, limit)
		lines = append(lines, trade.Line{Kind: k, Qty: n})
		qty -= n
	}
	return lines
}

// PartDue is the lifetime's end: the talks were never held (Timeout), or
// the site was refused (SiteLost).
func (Talks) PartDue(c quest.Ctx, q *qd.Quest, part int) quest.Verdict {
	p := talkPart(q)
	switch {
	case p == nil || p != &q.Parts[part] || p.Done:
		return quest.VerdictNone
	case p.Count == talkNoSite:
		return quest.VerdictFail(qd.SiteLost)
	}
	return quest.VerdictFail(qd.Timeout)
}

// Ended takes the talks site off the planet.
func (Talks) Ended(c quest.Ctx, q *qd.Quest) {
	if p := talkPart(q); p != nil && p.Count != talkNoSite {
		c.R.Sites.Remove(p.Tile, q.ID)
	}
}

// talkPart is q's talk part (nil: none).
func talkPart(q *qd.Quest) *qd.Part {
	for i := range q.Parts {
		if q.Parts[i].Kind == qd.PartTalk {
			return &q.Parts[i]
		}
	}
	return nil
}

// talkResultKeys name the outcomes, best first (qd.TalkResult order).
func talkResultKeys() [5]string {
	return [...]string{
		"quest.talks.triumph", "quest.talks.success", "quest.talks.flounder",
		"quest.talks.backfire", "quest.talks.disaster",
	}
}

// Detail: the host (faction ref, 1 + index) and the tile; before the talks
// that a caravan must reach it, after them the outcome, the goodwill change
// and the gift's value.
func (Talks) Detail(q qd.Quest) []quest.Line {
	p := talkPart(&q)
	if p == nil {
		return nil
	}
	tile := quest.Tile(p.Tile)
	lines := []quest.Line{
		{Key: "quest.talks.host", Args: []quest.Arg{quest.Faction(q.Giver)}},
		{Key: "quest.talks.site", Args: []quest.Arg{tile}},
	}
	keys := talkResultKeys()
	switch {
	case p.Count == talkNoSite:
		return append(lines, quest.Line{Key: "quest.talks.nosite"})
	case p.Count > 0 && p.Count <= len(keys):
		lines = append(lines, quest.Line{Key: keys[p.Count-1], Args: []quest.Arg{quest.Num(p.Points)}})
		if len(p.Lines) > 0 {
			lines = append(lines, quest.Line{Key: "quest.talks.gift", Args: []quest.Arg{quest.Num(qd.Value(p.Lines))}})
		}
		return lines
	case q.State == qd.Active:
		return append(lines, quest.Line{Key: "quest.talks.caravan", Args: []quest.Arg{tile}})
	}
	return lines
}
