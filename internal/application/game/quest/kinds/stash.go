package kinds

import (
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/combat/kit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/world/mapgen"
)

// Item stash tuning (ours; the wiki publishes no numbers, QST-050, 051).
const (
	// stashMinTiles..stashMaxTiles is the ring the stash lies in, the same
	// as the scanner's and the talks' (QST-050).
	stashMinTiles = 6
	stashMaxTiles = 25
	// stashGuardPct is the share of the quest's threat points the guards
	// are armed with; stashGuardMax the most guards; stashRing the posts
	// around the pile and stashRadius their distance in cells (ours).
	stashGuardPct = 50
	stashGuardMax = 8
	stashRing     = 6
	stashRadius   = 6
	// stashClear is the cleared margin around the pile (cells).
	stashClear = stashRadius + 2
)

// stashPool is the goods a stash may hold (ours).
var stashPool = []qd.PoolItem{
	{Kind: item.Silver}, {Kind: item.Steel}, {Kind: item.Components},
	{Kind: item.Plasteel}, {Kind: item.Gold}, {Kind: item.Cloth}, {Kind: item.HerbalMedicine},
}

// Stash is the item stash site (QST-050..054): goods lie on a land tile
// 6-25 tiles away behind a hostile faction's guards. It accepts itself with
// the def's 16-day window; the goods are its one reward option and stay at
// the site (the map's pack-up carries them home). A caravan reaching the
// tile opens the site map; the kind lays the pile and the guards then and
// succeeds at once (decision 7: reaching the stash is the success). It
// fails when the window runs out (the root's deadline) or the site is lost.
type Stash struct{}

func (Stash) Def() qd.Def { return qd.DefOf(qd.Stash) }

// stashGuards are the factions that may guard: hostile, standing, with a
// settlement (so with people). Ascending index.
func stashGuards(c quest.Ctx) []int {
	var out []int
	for _, f := range c.R.World.Factions() {
		if f.Relation == faction.Hostile && f.Human && !f.Defeated && len(f.Settlements) > 0 {
			out = append(out, f.Index)
		}
	}
	return out
}

// CanOffer: quest threats are on (a guarded site) and a hostile faction
// can guard it.
func (Stash) CanOffer(c quest.Ctx) bool {
	return c.R.World.QuestThreats() && len(stashGuards(c)) > 0
}

// Build picks the guard faction (Target), the tile, the goods (the single
// reward option, items only) and the guards' points. The giver is cosmetic:
// a non-hostile faction when there is one, rolled after the rewards so it
// cannot turn them into goodwill.
func (Stash) Build(c quest.Ctx, q *qd.Quest) bool {
	gs := stashGuards(c)
	if len(gs) == 0 {
		return false
	}
	seed := c.Seed()
	tile, ok := c.R.World.NearTile(seed, stashMinTiles, stashMaxTiles)
	if !ok {
		return false
	}
	q.Giver = 0
	q.Target = gs[c.Intn(len(gs))] + 1
	rs := c.Rewards(*q, stashPool, 0, false)
	if len(rs) == 0 || len(rs[0].Lines) == 0 {
		return false
	}
	q.Rewards = rs[:1]
	var friends []int
	for _, f := range c.R.World.Factions() {
		if f.Relation != faction.Hostile && !f.Defeated {
			friends = append(friends, f.Index)
		}
	}
	if len(friends) > 0 {
		q.Giver = friends[c.Intn(len(friends))] + 1
	}
	q.Parts = []qd.Part{{Kind: qd.PartSite, Tile: tile, Seed: c.Seed(), Points: c.ThreatPoints(*q)}}
	return true
}

func (Stash) Accepted(quest.Ctx, *qd.Quest) {}

// Signal: the site map opening lays the pile and the guards and succeeds;
// a lost site, or a site map that closed before it was laid, fails.
func (Stash) Signal(c quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	if q.State != qd.Active || len(q.Parts) == 0 || q.Parts[0].Done || (s.Tile != q.Parts[0].Tile && s.Kind != quest.SigMapClosed) {
		return quest.VerdictNone
	}
	switch s.Kind {
	case quest.SigSiteLost, quest.SigMapClosed:
		return quest.VerdictFail(qd.SiteLost)
	case quest.SigSiteOpened:
		return layStash(c, q)
	}
	return quest.VerdictNone
}

// layStash posts the guards on a ring around the map's middle, then drops
// the goods there. A site without goods to drop is no stash: the quest
// fails (SiteLost). Guards that cannot stand (the builder refuses) do not
// stop the success: the goods are down.
func layStash(c quest.Ctx, q *qd.Quest) quest.Verdict {
	if len(q.Rewards) == 0 || len(q.Rewards[0].Lines) == 0 {
		return quest.VerdictFail(qd.SiteLost)
	}
	p := &q.Parts[0]
	b := c.R.Builder
	w, h := b.Size()
	cx, cy := w/2, h/2
	posts := make([]mapgen.Point, 0, stashRing)
	for i := range stashRing {
		posts = append(posts, ringPoint(cx, cy, i, w, h))
	}
	comp := mapgen.Compound{
		X0: max(cx-stashClear, 0), Y0: max(cy-stashClear, 0),
		X1: min(cx+stashClear, w-1), Y1: min(cy+stashClear, h-1),
		Centre: mapgen.Point{X: cx, Y: cy}, Posts: posts,
	}
	arms := kit.Pick(p.Points*stashGuardPct/100, stashGuardMax, quest.TechOf(c.R.World.Factions(), q.Target), c.Intn)
	p.Count = b.Camp(comp, q.Target, arms, p.Seed)
	// After the camp: laying it clears every item inside its bounds.
	b.Goods(q.Rewards[0].Lines, cx, cy, true)
	p.Done = true
	return quest.VerdictSucceed
}

// ringPoint is post i of stashRing around (cx, cy), clamped to the map.
func ringPoint(cx, cy, i, w, h int) mapgen.Point {
	// Six evenly spaced points on a hexagon of "radius" stashRadius.
	dx := [stashRing]int{1, 1, 0, -1, -1, 0}
	dy := [stashRing]int{0, 1, 1, 0, -1, -1}
	return mapgen.Point{
		X: min(max(cx+dx[i]*stashRadius, 0), w-1),
		Y: min(max(cy+dy[i]*stashRadius, 0), h-1),
	}
}

func (Stash) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict { return quest.VerdictNone }
func (Stash) Ended(quest.Ctx, *qd.Quest)                      {}

// Detail: the site tile, the deadline and each stack of the goods.
func (Stash) Detail(q qd.Quest) []quest.Line {
	if len(q.Parts) == 0 || len(q.Rewards) == 0 {
		return nil
	}
	out := []quest.Line{
		{Key: "quest.stash.site", Args: []quest.Arg{quest.Tile(q.Parts[0].Tile)}},
		{Key: "quest.stash.deadline", Args: []quest.Arg{quest.Tick(q.Deadline)}},
	}
	for _, l := range q.Rewards[0].Lines {
		out = append(out, quest.Line{Key: "quest.stash.item", Args: []quest.Arg{quest.Item(l.Kind), quest.Num(l.Qty)}})
	}
	return out
}
