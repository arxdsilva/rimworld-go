package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Trader caravans (PLAY-024, agent S2): incident.TraderCaravan (or a
// caravan asked for over the radio) brings a visitor group from a
// non-hostile faction: a trader with the goods and one to three guards.
// They are ordinary visitors (visitors.go) that stroll within
// traderCaravanRange of home() for traderCaravanStay, then walk out.
// Killing or capturing the trader drops its goods (silver too) forbidden
// where it fell and costs the faction traderLostGoodwill. Faction, kind
// and stock draws come from streamTrade; the edge and the strolls reuse
// the visitors' world stream.

// caravanFits is the map's check for a trader caravan: a colony with
// people, a non-hostile human faction (a legacy colony has none), and no
// caravan or visitors already here.
func (g *Game) caravanFits(now uint64) bool {
	if g.trade.caravan != nil || len(g.pawnIDs) == 0 || len(g.visitingFactions()) == 0 {
		return false
	}
	return g.worldSt == nil || len(g.worldSt.visitors) == 0
}

// caravanArrive sends a trader caravan in; false when it could not come.
func (g *Game) caravanArrive(now uint64) bool {
	facs := g.visitingFactions()
	if len(facs) == 0 {
		return false
	}
	return g.traderCaravanFrom(facs[g.stream(streamTrade).IntN(len(facs))], now, false)
}

// traderCaravanFrom brings faction ref's (1 + index) trader caravan in
// from a map edge; requested: it was asked for over the radio.
func (g *Game) traderCaravanFrom(ref int, now uint64, requested bool) bool {
	f := g.factionOf(ref)
	if f == nil || g.trade.caravan != nil {
		return false
	}
	ex, ey, ok := g.worldEdge()
	if !ok {
		return false
	}
	r := g.stream(streamTrade)
	n := 2 + r.IntN(3)
	kind := trade.BulkHauler
	if r.IntN(2) == 1 {
		kind = trade.Quartermaster
	}
	var used [][2]int
	var ids []entity.EntityID
	for k := 0; k < n; k++ {
		x, y, ok := g.insetLandNear(ex, ey, used)
		if !ok {
			break
		}
		used = append(used, [2]int{x, y})
		id := g.spawnVisitor(ref, f, x, y, now)
		g.setVisitorStay(id, now+traderCaravanStay) // visitors.go
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return false
	}
	stock := trade.Generate(kind, faction.DefOf(f.Type).Tech, r)
	members := slices.Clone(ids)
	slices.Sort(members)
	g.trade.caravan = &traderCaravan{Trader: ids[0], Members: members, Faction: f.ID, Kind: kind, Stock: stock,
		LeaveAt: now + traderCaravanStay}
	why := "has come"
	if requested {
		why = "has come as you asked"
	}
	g.letter(letterInfo, "Trader caravan", fmt.Sprintf("%s, a %s of %s, %s to trade, with %d guards. They will camp near the colony for most of a day. Right-click the trader with a colonist selected to trade.",
		g.name(ids[0]), lower(trade.DefOf(kind).Label), f.Name, why, len(ids)-1), ids[0])
	return true
}

// lower is s with its first letter in lower case (labels inside a
// sentence).
func lower(s string) string {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return s
	}
	return string(s[0]+'a'-'A') + s[1:]
}

// traderOpen reports trader id is ready to trade now: the caravan's
// trader, conscious, on the map and not yet leaving.
func (g *Game) traderOpen(id entity.EntityID) bool {
	c := g.trade.caravan
	if c == nil || c.Trader != id || uint64(g.clock.Now()) >= c.LeaveAt || !g.visitor(id) {
		return false
	}
	v, _ := g.vital.Get(id)
	return v.Status == health.Alive
}

// traderAt is the trader standing at x, y (0: none).
func (g *Game) traderAt(x, y int) entity.EntityID {
	c := g.trade.caravan
	if c == nil || !g.visitor(c.Trader) {
		return 0
	}
	if p, ok := g.positions.Get(c.Trader); ok && p.X == x && p.Y == y {
		return c.Trader
	}
	return 0
}

// traderHeld reports visitor id is the trader while a colonist walks up
// to it or trades with it: it stays put (no new stroll).
func (g *Game) traderHeld(id entity.EntityID) bool {
	c := g.trade.caravan
	if c == nil || c.Trader != id {
		return false
	}
	if s := g.trade.session; s != nil && s.trader == id {
		return true
	}
	_, taken := g.reserve.EntityOwner(id)
	return taken
}

// caravanStrollRange is how far from home() visitor id strolls.
func (g *Game) caravanStrollRange(id entity.EntityID) int {
	if c := g.trade.caravan; c != nil {
		if _, ok := slices.BinarySearch(c.Members, id); ok {
			return traderCaravanRange
		}
	}
	return visitorRange
}

// traderVisitorGone: visitor id is no longer a visitor on the map
// (visitorsTick). A trader that walked off takes the caravan with it; one
// killed or taken prisoner drops its goods and angers its faction.
func (g *Game) traderVisitorGone(id entity.EntityID) {
	c := g.trade.caravan
	if c == nil {
		return
	}
	if i, ok := slices.BinarySearch(c.Members, id); ok {
		c.Members = slices.Delete(c.Members, i, i+1)
	}
	if id != c.Trader {
		return
	}
	g.trade.caravan = nil
	if g.trade.session != nil && g.trade.session.trader == id {
		g.trade.session = nil
	}
	if p := g.personOf(id); p != nil && p.Where == whereOffMap {
		return // left with its goods
	}
	g.traderLost(c)
}

// traderLost drops caravan c's goods forbidden where its trader fell and
// costs the faction traderLostGoodwill.
func (g *Game) traderLost(c *traderCaravan) {
	x, y := g.home()
	if p, ok := g.positions.Get(c.Trader); ok {
		x, y = p.X, p.Y
	}
	for _, l := range c.Stock.Lines {
		g.dropGoods(l.Kind, l.Qty, x, y, true)
	}
	if c.Stock.Silver > 0 {
		g.dropGoods(item.Silver, c.Stock.Silver, x, y, true)
	}
	f := g.factionByID(c.Faction)
	if f == nil {
		return
	}
	now := uint64(g.clock.Now())
	faction.Adjust(f, -traderLostGoodwill, fmt.Sprintf("Day %d: trader killed or taken (%d goodwill)", g.relDay(now)+1, -traderLostGoodwill))
	g.letter(letterCaution, "Trader lost", fmt.Sprintf("%s, the trader of %s, is lost to them. Its goods lie where it fell. %s will not forget it (%d goodwill).",
		g.name(c.Trader), f.Name, f.Name, -traderLostGoodwill), c.Trader)
}

// requestedCaravanDue sends the first requested caravan whose time has
// come; one that can't come yet (visitors or another caravan here) tries
// again later, one whose faction turned hostile or fell never comes.
func (g *Game) requestedCaravanDue(now uint64) {
	t := &g.trade
	req := t.requests[0]
	t.requests = slices.Delete(t.requests, 0, 1)
	f := g.factionByID(req.Faction)
	if f == nil || f.Defeated || f.Relation == faction.Hostile {
		return
	}
	busy := t.caravan != nil || g.worldSt != nil && len(g.worldSt.visitors) > 0
	if !busy && g.traderCaravanFrom(g.factionRef(req.Faction), now, true) {
		return
	}
	g.addRequest(caravanRequest{Faction: req.Faction, At: now + requestRetry})
}

// addRequest queues r in arrival order (ties: lower faction id first).
func (g *Game) addRequest(r caravanRequest) {
	rs := g.trade.requests
	i, _ := slices.BinarySearchFunc(rs, r, func(a, b caravanRequest) int {
		if a.At != b.At {
			if a.At < b.At {
				return -1
			}
			return 1
		}
		return a.Faction - b.Faction
	})
	g.trade.requests = slices.Insert(rs, i, r)
}

// requestPending reports a caravan from faction id is on its way.
func (g *Game) requestPending(id int) bool {
	for _, r := range g.trade.requests {
		if r.Faction == id {
			return true
		}
	}
	return false
}
