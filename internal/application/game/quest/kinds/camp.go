package kinds

import (
	"strconv"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/combat/kit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/pkg/randx"
)

// Hostile camp tuning (ours; the wiki publishes no numbers, QST-060..062).
const (
	// campMinTiles..campMaxTiles is the ring the camp lies in, as for the
	// other site kinds.
	campMinTiles = 6
	campMaxTiles = 25
	// campDefenders is the most defenders a camp fields: fewer than a
	// settlement's 16.
	campDefenders = 10
	// campGoodwill is the giver's goodwill for the cleared camp (QST-062,
	// QST-095), on top of the chosen reward.
	campGoodwill = 8
	// campSalt keeps the defenders' arms apart from the layout's stream.
	campSalt uint64 = 0xa7e5
)

// The camp's two parts: the site, and its defenders.
const (
	campSite  = 0
	campGuard = 1
)

// campPool is the reward options' item pool (ours).
var campPool = []qd.PoolItem{
	{Kind: item.Silver}, {Kind: item.Steel}, {Kind: item.Components},
	{Kind: item.IndustrialMedicine}, {Kind: item.Plasteel}, {Kind: item.Gold},
}

// Camp is the hostile camp to clear (QST-060..064): a faction that is not
// hostile asks the colony to break an outpost of a hostile faction 6-25
// tiles away. It is an offer (6 days to accept, then the def's 17-day
// window, QST-060); the reward options come from the giver, paid by pod at
// home, the goodwill option with the camp's loot (QST-082). A caravan
// reaching the tile opens the site map; the camp is laid then, with its
// defenders. Once more than half of them are down, dead or fled the quest
// succeeds and the giver gains 8 goodwill (QST-061, 062). It fails on the
// deadline, on the giver turning hostile or the target not (the root's Def
// rules, QST-063), and when the site is lost or its map closes first.
//
// Parts: campSite is the PartSite (Tile, Seed; Map once open, Count the
// defenders spawned); campGuard the PartThreat (Points the defenders'
// points, rolled at build with the stars; Count those standing at the last
// census). Neither has a timer.
type Camp struct{}

func (Camp) Def() qd.Def { return qd.DefOf(qd.Camp) }

// campGivers may ask: not hostile, able to be friends, standing, with a
// settlement (QST-038). Ascending index.
func campGivers(c quest.Ctx) []int {
	var out []int
	for _, f := range c.R.World.Factions() {
		if f.Relation != faction.Hostile && f.CanBeFriendly && !f.Defeated && len(f.Settlements) > 0 {
			out = append(out, f.Index)
		}
	}
	return out
}

// campTargets may hold the camp: hostile, standing, with a settlement (so
// with people). Ascending index.
func campTargets(c quest.Ctx) []int {
	var out []int
	for _, f := range c.R.World.Factions() {
		if f.Relation == faction.Hostile && !f.Defeated && len(f.Settlements) > 0 {
			out = append(out, f.Index)
		}
	}
	return out
}

// CanOffer: quest threats are on (QST-033), and a giver and a target exist.
func (Camp) CanOffer(c quest.Ctx) bool {
	return c.R.World.QuestThreats() && len(campGivers(c)) > 0 && len(campTargets(c)) > 0
}

// Build picks the giver, the target and the tile, rolls the defenders'
// points (stars and spread) and the giver's reward options.
func (Camp) Build(c quest.Ctx, q *qd.Quest) bool {
	gs, ts := campGivers(c), campTargets(c)
	if len(gs) == 0 || len(ts) == 0 {
		return false
	}
	seed := c.Seed()
	tile, ok := c.R.World.NearTile(seed, campMinTiles, campMaxTiles)
	if !ok {
		return false
	}
	q.Giver, q.Target = gs[c.Intn(len(gs))]+1, ts[c.Intn(len(ts))]+1
	q.Parts = []qd.Part{
		{Kind: qd.PartSite, Tile: tile, Seed: seed},
		{Kind: qd.PartThreat, Tile: -1, Points: c.ThreatPoints(*q)},
	}
	q.Rewards = c.Rewards(*q, campPool, 0, true)
	return len(q.Rewards) > 0
}

func (Camp) Accepted(quest.Ctx, *qd.Quest) {}

// campParts are q's site and guard parts (ok false: not a camp's).
func campParts(q *qd.Quest) (site, guard *qd.Part, ok bool) {
	if len(q.Parts) != 2 || q.Parts[campSite].Kind != qd.PartSite || q.Parts[campGuard].Kind != qd.PartThreat {
		return nil, nil, false
	}
	return &q.Parts[campSite], &q.Parts[campGuard], true
}

// Signal: the site map opening lays the camp; each census may succeed it;
// the map closing, or the site being lost, while it is open fails it.
func (Camp) Signal(c quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	site, guard, ok := campParts(q)
	if !ok || q.State != qd.Active {
		return quest.VerdictNone
	}
	switch s.Kind {
	case quest.SigSiteOpened:
		if site.Map == 0 && s.Tile == site.Tile && s.Map != 0 {
			layCamp(c, q, site, guard, s.Map)
		}
	case quest.SigSiteCensus:
		if site.Map != 0 && s.Map == site.Map {
			return census(c, q, site, guard, s.Count)
		}
	case quest.SigMapClosed:
		if site.Map != 0 && s.Map == site.Map {
			return quest.VerdictFail(qd.SiteLost)
		}
	case quest.SigSiteLost:
		if s.Tile == site.Tile {
			return quest.VerdictFail(qd.SiteLost)
		}
	}
	return quest.VerdictNone
}

// layCamp lays the camp on the open site map m and stands its defenders,
// armed by the target's tech from the guard's points.
func layCamp(c quest.Ctx, q *qd.Quest, site, guard *qd.Part, m uint32) {
	b := c.R.Builder
	w, h := b.Size()
	tech := campTech(c, q.Target)
	comp := mapgen.Camp(mapgen.CompoundConfig{W: w, H: h, Tech: tech, Seed: site.Seed})
	arms := kit.Pick(guard.Points, campDefenders, tech, randx.New(site.Seed^campSalt).IntN)
	site.Map = m
	site.Count = b.Camp(comp, q.Target, arms, site.Seed)
	guard.Count = site.Count
}

// campTech is faction ref's tech. The faction view carries none yet, so
// every camp is armed and built as an industrial faction's.
func campTech(quest.Ctx, int) faction.Tech { return faction.Industrial }

// census takes the hour's count of standing hostiles: the camp is cleared
// once fewer than half of those spawned stand (more than 50 % down, dead or
// fled, QST-061), or none stands. A success is terminal, so it cannot flap.
func census(c quest.Ctx, q *qd.Quest, site, guard *qd.Part, standing int) quest.Verdict {
	guard.Count = standing
	if standing*2 >= site.Count && standing > 0 {
		return quest.VerdictNone
	}
	site.Done, guard.Done = true, true
	c.R.Goodwill.Adjust(q.Giver-1, campGoodwill)
	return quest.VerdictSucceed
}

func (Camp) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict { return quest.VerdictNone }
func (Camp) Ended(quest.Ctx, *qd.Quest)                      {}

// Detail: the giver and the target (faction refs), the site tile, the
// deadline once accepted and, once the camp is laid, how many of its
// defenders stand.
func (Camp) Detail(q qd.Quest) []quest.Line {
	site, guard, ok := campParts(&q)
	if !ok {
		return nil
	}
	out := []quest.Line{
		{Key: "quest.camp.giver", Args: []string{strconv.Itoa(q.Giver)}},
		{Key: "quest.camp.target", Args: []string{strconv.Itoa(q.Target)}},
		{Key: "quest.camp.site", Args: []string{strconv.Itoa(site.Tile)}},
	}
	if q.Deadline != 0 {
		out = append(out, quest.Line{Key: "quest.camp.deadline", Args: []string{strconv.FormatUint(q.Deadline, 10)}})
	}
	if site.Map != 0 {
		out = append(out, quest.Line{Key: "quest.camp.standing", Args: []string{strconv.Itoa(guard.Count), strconv.Itoa(site.Count)}})
	}
	return out
}
