package kinds

import (
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
)

// Trade request tuning (ours; the wiki publishes no counts, QST-070).
const (
	// requestValue is the requested goods' market value at one star and 500
	// points (the gap file's "1x = 800 silver"); it scales with stars and
	// with the colony's points, clamped to requestMin..requestMax.
	requestValue = 800
	requestRef   = 500
	requestMin   = 300
	requestMax   = 6000
	// requestGoodwill is the giver's goodwill gain on delivery (QST-095).
	requestGoodwill = 12
)

// requestKinds are the goods a colony can make or gather and a settlement
// may ask for (ours): never silver, never weapons or apparel (one per
// stack, quality varies) and nothing only a trader sells.
var requestKinds = [...]item.Kind{
	item.Steel, item.Components, item.HerbalMedicine,
	item.Leather, item.Cloth, item.Meal,
}

// requestRewards is the item pool of the reward options (ours).
var requestRewards = []qd.PoolItem{
	{Kind: item.Silver}, {Kind: item.Steel}, {Kind: item.Components},
	{Kind: item.HerbalMedicine}, {Kind: item.Cloth}, {Kind: item.Plasteel},
}

// Request is the trade request delivered by caravan (QST-070..074, 095, 118):
// a friendly faction's settlement asks for N items of one kind. The player's
// Fulfil command, from a caravan standing on the settlement's tile with the
// goods, hands them over; the choice reward goes into that caravan and the
// giver's goodwill rises by 12. Fails on the deadline and when the giver
// turns hostile (the root's Def rules), and when the settlement is lost.
type Request struct{}

func (Request) Def() qd.Def { return qd.DefOf(qd.Request) }

// requestGivers are the factions that may ask: not hostile, able to be
// friendly, standing, with a settlement. Ascending index, no draw.
func requestGivers(c quest.Ctx) []quest.FactionView {
	var out []quest.FactionView
	for _, f := range c.R.World.Factions() {
		if f.Relation != faction.Hostile && f.CanBeFriendly && !f.Defeated && len(f.Settlements) > 0 {
			out = append(out, f)
		}
	}
	return out
}

func (Request) CanOffer(c quest.Ctx) bool { return len(requestGivers(c)) > 0 }

// requestLines is the request for q: one item kind, N units so the market
// value is requestValue x stars x points / 500, clamped (ours), at least 1.
func requestLines(c quest.Ctx, q qd.Quest) []trade.Line {
	k := requestKinds[c.Intn(len(requestKinds))]
	target := requestValue * int(q.Stars) * q.Points / requestRef
	target = min(max(target, requestMin), requestMax)
	return []trade.Line{{Kind: k, Qty: max(1, target/max(1, k.Value()))}}
}

func (Request) Build(c quest.Ctx, q *qd.Quest) bool {
	givers := requestGivers(c)
	if len(givers) == 0 {
		return false
	}
	g := givers[c.Intn(len(givers))]
	tile, ok := c.R.Sites.Settlement(g.Index, c.Seed())
	if !ok {
		return false
	}
	q.Giver = g.Index + 1
	q.Parts = []qd.Part{{Kind: qd.PartDelivery, Tile: tile, Lines: requestLines(c, *q)}}
	q.Rewards = c.Rewards(*q, requestRewards, 0, false)
	return true
}

func (Request) Accepted(quest.Ctx, *qd.Quest) {}

// Signal: Fulfil hands the goods over (QST-071, 072); a lost settlement or a
// defeated giver ends the quest. Everything else, a mere arrival included,
// changes nothing: the UI enables the Fulfil button on arrival.
func (Request) Signal(c quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	if q.State != qd.Active || len(q.Parts) == 0 || q.Parts[0].Done {
		return quest.VerdictNone
	}
	p := &q.Parts[0]
	switch s.Kind {
	case quest.SigSiteLost, quest.SigFactionDefeated:
		return quest.VerdictFail(qd.SiteLost)
	case quest.SigFulfil:
		if at, ok := c.R.Items.CaravanAt(s.Caravan); !ok || at != p.Tile {
			return quest.VerdictNone
		}
		if !c.R.Items.Has(s.Caravan, p.Lines) || !c.R.Items.Take(s.Caravan, p.Lines) {
			return quest.VerdictNone
		}
		c.R.Goodwill.Adjust(q.Giver-1, requestGoodwill)
		p.Done = true
		return quest.VerdictSucceed
	}
	return quest.VerdictNone
}

func (Request) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict { return quest.VerdictNone }
func (Request) Ended(quest.Ctx, *qd.Quest)                      {}

// Detail: the item kind id, the count, the settlement tile and the giver.
func (Request) Detail(q qd.Quest) []quest.Line {
	if len(q.Parts) == 0 || len(q.Parts[0].Lines) == 0 {
		return nil
	}
	p, l := q.Parts[0], q.Parts[0].Lines[0]
	return []quest.Line{
		{Key: "quest.request.item", Args: []quest.Arg{quest.Item(l.Kind)}},
		{Key: "quest.request.count", Args: []quest.Arg{quest.Num(l.Qty)}},
		{Key: "quest.request.tile", Args: []quest.Arg{quest.Tile(p.Tile)}},
		{Key: "quest.request.giver", Args: []quest.Arg{quest.Faction(q.Giver)}},
	}
}
