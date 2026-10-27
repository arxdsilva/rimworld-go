package game

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Trade state (PLAY-024, agent S2): the trader caravan on the map, the
// orbital ship in radio range, drop pods bringing orbital purchases, the
// caravans asked for over the radio and each faction's request
// cooldowns, plus the open trade or comms session. Only one session is
// open at a time; sessions are never saved (a load closes them, and the
// held Trade/UseComms job ends on its next Do tick). Everything here is
// sorted slices (no maps), so the save and the hash are deterministic.
// tradeTick runs once a tick from phaseIncidents and returns at once
// while there is nothing to do.

const (
	tradeSaveVersion = 1
	// tradeWait is how long a negotiator or radio operator stands at the
	// trader or console before the session opens.
	tradeWait = 60
	// traderCaravanStay is how long a trader caravan camps near home().
	traderCaravanStay = clock.TicksPerDay * 3 / 4
	// traderCaravanRange is how far from home() its people stroll.
	traderCaravanRange = 6
	// orbitalStay is how long an orbital ship stays in radio range.
	orbitalStay = clock.TicksPerDay * 3 / 2
	// beaconRadius is a trade beacon's reach (Euclidean tiles).
	beaconRadius = 6
	// tradePodFall is how long an orbital purchase takes to land.
	tradePodFall = 60
	// traderLostGoodwill is what killing or capturing a trader costs.
	traderLostGoodwill = 40
	// requestRetry: a requested caravan that can't come yet tries again.
	requestRetry = clock.TicksPerHour
)

// tradeState is the colony's trade.
type tradeState struct {
	caravan   *traderCaravan
	orbital   *orbitalShip
	pods      []tradePod        // by landing tick, then position
	cooldowns []factionCooldown // ascending faction id
	requests  []caravanRequest  // by arrival tick, then faction
	aid       []aidCall         // military aid, by request (allies.go)
	session   *tradeSession
	comms     *commsSession
}

// traderCaravan is a trader caravan on the map (saved). Trader is the
// visitor carrying the goods, Members the whole group (ascending),
// Faction the faction id (faction.Faction.ID).
type traderCaravan struct {
	Trader  entity.EntityID
	Members []entity.EntityID
	Faction int
	Kind    trade.TraderKind
	Stock   trade.Stock
	LeaveAt uint64
}

// orbitalShip is an orbital trader in radio range until Until (saved).
type orbitalShip struct {
	Name  string
	Kind  trade.TraderKind
	Stock trade.Stock
	Until uint64
}

// tradePod is a drop pod with orbital purchases (saved): it falls until
// Tick, then lands a shell at X, Y with Items beside it; it stays listed
// for the renderer for arrivalKeep after landing.
type tradePod struct {
	X, Y   int
	Tick   uint64
	Landed bool
	Items  []trade.Line
}

// factionCooldown is faction id Faction's request cooldowns (saved).
type factionCooldown struct {
	Faction int
	CD      faction.Cooldowns
}

// caravanRequest is a caravan asked for over the radio, due at At.
type caravanRequest struct {
	Faction int
	At      uint64
}

// tradeSession is an open trade: the negotiating colonist and the trader
// (a visitor pawn, or 0 for the orbital ship), and why the last offer was
// refused.
type tradeSession struct {
	negotiator entity.EntityID
	trader     entity.EntityID
	reason     string
}

// commsSession is an open comms session: the operator and the console.
type commsSession struct {
	operator entity.EntityID
	console  entity.EntityID
}

// tradeSave is the saved trade state (versioned), nil while there is
// nothing to keep, so older saves and colonies that never traded hash as
// before.
type tradeSave struct {
	Version   int
	Caravan   *traderCaravan
	Orbital   *orbitalShip
	Pods      []tradePod
	Cooldowns []factionCooldown
	Requests  []caravanRequest
	Aid       []aidCall // allies.go (Phase 4): nil without aid
}

// exportTrade is the Trade save field (nil: nothing to keep).
func (g *Game) exportTrade() *tradeSave {
	t := &g.trade
	if t.caravan == nil && t.orbital == nil && len(t.pods) == 0 && len(t.cooldowns) == 0 && len(t.requests) == 0 && len(t.aid) == 0 {
		return nil
	}
	return &tradeSave{Version: tradeSaveVersion, Caravan: cloneCaravan(t.caravan), Orbital: cloneShip(t.orbital),
		Pods: clonePodsT(t.pods), Cooldowns: slices.Clone(t.cooldowns), Requests: slices.Clone(t.requests), Aid: slices.Clone(t.aid)}
}

// importTrade restores exportTrade's state (nil: none); open sessions
// close on load.
func (g *Game) importTrade(s *tradeSave) {
	g.trade = tradeState{}
	if s == nil {
		return
	}
	g.trade = tradeState{caravan: cloneCaravan(s.Caravan), orbital: cloneShip(s.Orbital), pods: clonePodsT(s.Pods),
		cooldowns: slices.Clone(s.Cooldowns), requests: slices.Clone(s.Requests), aid: slices.Clone(s.Aid)}
}

func cloneStock(s trade.Stock) trade.Stock {
	s.Lines = slices.Clone(s.Lines)
	return s
}

func cloneCaravan(c *traderCaravan) *traderCaravan {
	if c == nil {
		return nil
	}
	out := *c
	out.Members = slices.Clone(c.Members)
	out.Stock = cloneStock(c.Stock)
	return &out
}

func cloneShip(s *orbitalShip) *orbitalShip {
	if s == nil {
		return nil
	}
	out := *s
	out.Stock = cloneStock(s.Stock)
	return &out
}

func clonePodsT(ps []tradePod) []tradePod {
	out := slices.Clone(ps)
	for i := range out {
		out[i].Items = slices.Clone(out[i].Items)
	}
	return out
}

// tradeTick runs the trade once a tick (phaseIncidents): requested
// caravans come, the ship leaves, pods land and sessions whose people or
// trader are gone close.
func (g *Game) tradeTick(now uint64) {
	t := &g.trade
	if t.caravan == nil && t.orbital == nil && len(t.pods) == 0 && len(t.requests) == 0 && len(t.aid) == 0 && t.session == nil && t.comms == nil {
		return
	}
	if len(t.aid) > 0 {
		g.aidTick(now) // allies.go (Phase 4): military aid
	}
	if len(t.requests) > 0 && now >= t.requests[0].At {
		g.requestedCaravanDue(now) // trade_caravan.go
	}
	if t.orbital != nil && now >= t.orbital.Until {
		g.orbitalLeaves() // trade_orbital.go
	}
	if len(t.pods) > 0 {
		g.tradePodsTick(now) // trade_orbital.go
	}
	g.checkSessions()
}

// checkSessions closes a session whose people are no longer at it: the
// negotiator or operator down, off the job, or the trader gone.
func (g *Game) checkSessions() {
	t := &g.trade
	if s := t.session; s != nil {
		ok := g.holdingJob(s.negotiator)
		if s.trader != 0 {
			ok = ok && g.traderOpen(s.trader)
		} else {
			ok = ok && t.orbital != nil
		}
		if !ok {
			t.session = nil
		}
	}
	if c := t.comms; c != nil && (!g.holdingJob(c.operator) || !g.consoleUsable(c.console)) {
		t.comms = nil
	}
}

// TradeStatus is the trade alerts' read model: the orbital ship in radio
// range (Present) and the trader caravan on the map (CaravanVisiting,
// CaravanFaction, CaravanHoursLeft). ok: either is here. It reads no
// session, so it works whether or not a dialog is open.
func (g *Game) TradeStatus() (OrbitalView, bool) {
	v := g.orbitalView(uint64(g.clock.Now()))
	if c := g.trade.caravan; c != nil {
		v.CaravanVisiting = true
		if f := g.factionByID(c.Faction); f != nil {
			v.CaravanFaction = f.Name
		}
		v.CaravanHoursLeft = hoursLeft(c.LeaveAt, uint64(g.clock.Now()))
	}
	return v, v.Present || v.CaravanVisiting
}

// hoursLeft is the whole hours (rounded up) from now until end.
func hoursLeft(end, now uint64) int {
	if end <= now {
		return 0
	}
	return int((end - now + clock.TicksPerHour - 1) / clock.TicksPerHour)
}

// factionByID is the faction whose ID is id (nil: none).
func (g *Game) factionByID(id int) *faction.Faction {
	for i := range g.ng.factions {
		if g.ng.factions[i].ID == id {
			return &g.ng.factions[i]
		}
	}
	return nil
}

// factionRef is faction id's visitor/raid reference (1 + index; 0: none).
func (g *Game) factionRef(id int) int {
	for i := range g.ng.factions {
		if g.ng.factions[i].ID == id {
			return i + 1
		}
	}
	return 0
}

// cooldownOf is faction id's request cooldowns.
func (g *Game) cooldownOf(id int) faction.Cooldowns {
	cs := g.trade.cooldowns
	if i, ok := slices.BinarySearchFunc(cs, id, func(c factionCooldown, id int) int { return c.Faction - id }); ok {
		return cs[i].CD
	}
	return faction.Cooldowns{}
}

// setCooldown records faction id's request cooldowns.
func (g *Game) setCooldown(id int, cd faction.Cooldowns) {
	cs := g.trade.cooldowns
	i, ok := slices.BinarySearchFunc(cs, id, func(c factionCooldown, id int) int { return c.Faction - id })
	if ok {
		cs[i].CD = cd
		return
	}
	g.trade.cooldowns = slices.Insert(cs, i, factionCooldown{Faction: id, CD: cd})
}

// beaconIDs lists the trade beacons, ascending.
func (g *Game) beaconIDs() []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.TradeBeacon {
			out = append(out, id)
		}
	}
	return out
}

// tradeGoods is what the colony can trade: for a caravan the unforbidden
// loose stacks on stockpile cells, for the orbital ship those within
// beaconRadius of a trade beacon (one pass over the items, so it stays
// bounded by the item count). ids are ascending per kind.
type tradeGoods struct {
	ids    map[item.Kind][]entity.EntityID
	counts map[item.Kind]int
}

func (g *Game) colonyGoods(orbital bool) tradeGoods {
	gd := tradeGoods{ids: map[item.Kind][]entity.EntityID{}, counts: map[item.Kind]int{}}
	var beacons [][2]int
	if orbital {
		for _, bid := range g.beaconIDs() {
			b := g.bldgs.Ref(bid)
			beacons = append(beacons, [2]int{b.X, b.Y})
		}
		if len(beacons) == 0 {
			return gd
		}
	}
	for _, iid := range g.itemIDs {
		st := g.stacks.Ref(iid)
		if st == nil || st.Carrier != 0 || st.Forbidden || st.Qty <= 0 || g.fogged(st.X, st.Y) {
			continue
		}
		if orbital {
			if !nearAny(st.X, st.Y, beacons, beaconRadius) {
				continue
			}
		} else if !g.stockpiled(st.X, st.Y) {
			continue
		}
		gd.ids[st.Kind] = append(gd.ids[st.Kind], iid)
		gd.counts[st.Kind] += st.Qty
	}
	for k := range gd.ids {
		slices.Sort(gd.ids[k])
	}
	return gd
}

// nearAny reports (x, y) within r (Euclidean) of any of pts.
func nearAny(x, y int, pts [][2]int, r int) bool {
	for _, p := range pts {
		dx, dy := x-p[0], y-p[1]
		if dx*dx+dy*dy <= r*r {
			return true
		}
	}
	return false
}

// takeGoods removes n of kind k from gd's stacks, ascending id first.
func (g *Game) takeGoods(gd tradeGoods, k item.Kind, n int) {
	for _, iid := range gd.ids[k] {
		if n <= 0 {
			return
		}
		st, ok := g.stacks.Get(iid)
		if !ok {
			continue
		}
		take := min(n, st.Qty)
		g.takeItems(iid, st, take) // animal_needs.go: deletes an emptied stack
		n -= take
	}
}

// dropGoods puts qty of k down at (x, y) in stacks of at most
// item.StackLimit (spilling to free tiles), forbidden when forbid.
func (g *Game) dropGoods(k item.Kind, qty, x, y int, forbid bool) {
	for qty > 0 {
		n := min(qty, item.StackLimit)
		qty -= n
		sid, err := g.SpawnStack(k, n, x, y)
		if err != nil {
			return
		}
		if s, ok := g.stacks.Get(sid); ok {
			s.Forbidden = forbid
			g.mergeOrSet(sid, s)
		}
	}
}

// stockAdd adds qty (negative: removes) of kind k to st, keeping its lines
// sorted by kind and dropping empty ones.
func stockAdd(st *trade.Stock, k item.Kind, qty int) {
	i, ok := slices.BinarySearchFunc(st.Lines, k, func(l trade.Line, k item.Kind) int { return int(l.Kind) - int(k) })
	if ok {
		st.Lines[i].Qty += qty
		if st.Lines[i].Qty <= 0 {
			st.Lines = slices.Delete(st.Lines, i, i+1)
		}
		return
	}
	if qty > 0 {
		st.Lines = slices.Insert(st.Lines, i, trade.Line{Kind: k, Qty: qty})
	}
}
