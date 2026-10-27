package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Radio comms (PLAY-024, agent S2): the UseComms job walks a colonist to a
// powered radio console's spot, waits tradeWait ticks and opens the comms
// session; the job holds in Do while the comms session (or the orbital
// trade it opened) lasts, and checkSessions closes it when the console
// loses power or the operator goes down. The dialog reads CommsView and
// sends the Comms* commands, each a no-op without an open session.
// Factions are named by faction.Faction.ID (FactionView.ID).

// Comms order and action refusals.
const (
	ReasonNoPower      = "no power"
	ReasonNoBeacon     = "needs a trade beacon"
	ReasonNoSilver     = "no silver near a trade beacon"
	ReasonOnTheWay     = "a caravan is on its way"
	ReasonCaravanHere  = "a caravan is already here"
	ReasonNotTraders   = "they don't trade"
	ReasonGiftTooSmall = "too little silver to matter"
)

// CommsView is the open comms session (ok false while none is open).
func (g *Game) CommsView() (CommsView, bool) {
	s := g.trade.comms
	if s == nil {
		return CommsView{}, false
	}
	now := uint64(g.clock.Now())
	v := CommsView{Operator: s.operator, OperatorName: g.name(s.operator), Console: s.console, Orbital: g.orbitalView(now)}
	beacons := len(g.beaconIDs()) > 0
	if beacons {
		v.BeaconSilver = g.colonyGoods(true).counts[item.Silver]
	}
	for _, f := range g.ng.factions {
		if faction.DefOf(f.Type).Hidden {
			continue
		}
		v.Factions = append(v.Factions, g.commsFaction(f, beacons, v.BeaconSilver, now))
	}
	return v, true
}

// commsFaction is faction f's comms row.
func (g *Game) commsFaction(f faction.Faction, beacons bool, silver int, now uint64) CommsFactionView {
	fv := factionView(f) // factions.go: the relation label
	row := CommsFactionView{ID: f.ID, Name: f.Name, Type: f.Type.String(), Goodwill: f.Goodwill, Relation: fv.Relation}
	row.Gift = CommsActionView{Cost: faction.GiftSilverPerGoodwill}
	switch {
	case f.Defeated:
		row.Gift.Reason = faction.ReasonGone
	case !beacons:
		row.Gift.Reason = ReasonNoBeacon
	case silver < faction.GiftSilverPerGoodwill:
		row.Gift.Reason = ReasonNoSilver
	default:
		row.Gift.Allowed = true
	}
	row.Caravan = CommsActionView{Cost: faction.CaravanCost}
	row.Caravan.Allowed, row.Caravan.Reason = g.caravanRequestOK(f, now)
	row.Aid = CommsActionView{Cost: faction.AidCost}
	row.Aid.Allowed, row.Aid.Reason = g.aidRequestOK(f, now) // allies.go (Phase 4)
	return row
}

// caravanRequestOK reports whether faction f sends a caravan if asked
// now, or why not.
func (g *Game) caravanRequestOK(f faction.Faction, now uint64) (bool, string) {
	if !f.Type.Human() {
		return false, ReasonNotTraders
	}
	if ok, why := faction.CanRequestCaravan(f, g.cooldownOf(f.ID), now); !ok {
		return false, why
	}
	if g.requestPending(f.ID) {
		return false, ReasonOnTheWay
	}
	if c := g.trade.caravan; c != nil && c.Faction == f.ID {
		return false, ReasonCaravanHere
	}
	return true, ""
}

// cmdCommsCallTrader opens trade with the orbital ship in range: the
// comms session becomes the orbital trade session (same operator).
func (g *Game) cmdCommsCallTrader() {
	c := g.trade.comms
	if c == nil || !g.orbitalView(uint64(g.clock.Now())).CallOK {
		return
	}
	g.trade.comms = nil
	g.trade.session = &tradeSession{negotiator: c.operator}
}

// cmdCommsGift gives faction silver from beacon reach (ascending stack
// ids), for goodwill.
func (g *Game) cmdCommsGift(id, silver int) {
	c := g.trade.comms
	f := g.factionByID(id)
	if c == nil || f == nil || f.Defeated || faction.DefOf(f.Type).Hidden || silver <= 0 {
		return
	}
	gw := faction.GiftGoodwill(silver)
	gd := g.colonyGoods(true)
	if gw <= 0 || gd.counts[item.Silver] < silver {
		return
	}
	g.takeGoods(gd, item.Silver, silver)
	now := uint64(g.clock.Now())
	faction.Adjust(f, gw, fmt.Sprintf("Day %d: %d silver sent by radio (%+d goodwill)", g.relDay(now)+1, silver, gw))
	g.worldNote("%s received %d silver (%+d goodwill).", f.Name, silver, gw)
}

// cmdCommsRequestCaravan asks faction id for a trade caravan: it costs
// goodwill, starts the cooldown and the caravan comes in 6-12 hours.
func (g *Game) cmdCommsRequestCaravan(id int) {
	f := g.factionByID(id)
	now := uint64(g.clock.Now())
	if g.trade.comms == nil || f == nil {
		return
	}
	if ok, _ := g.caravanRequestOK(*f, now); !ok {
		return
	}
	faction.Adjust(f, -faction.CaravanCost, fmt.Sprintf("Day %d: trade caravan requested (%d goodwill)", g.relDay(now)+1, -faction.CaravanCost))
	cd := g.cooldownOf(id)
	cd.CaravanUntil = now + faction.CaravanCooldownDays*clock.TicksPerDay
	g.setCooldown(id, cd)
	hours := faction.CaravanArriveMinHours + g.stream(streamTrade).IntN(faction.CaravanArriveMaxHours-faction.CaravanArriveMinHours+1)
	g.addRequest(caravanRequest{Faction: id, At: now + uint64(hours)*clock.TicksPerHour}) // trade_caravan.go
	g.worldNote("%s will send a trade caravan; it should arrive in about %d hours.", f.Name, hours)
}

// cmdCommsRequestAid asks faction id for military aid (allies.go): it
// costs goodwill, starts the cooldown and the fighters come in 2-4 hours.
func (g *Game) cmdCommsRequestAid(id int) {
	f := g.factionByID(id)
	now := uint64(g.clock.Now())
	if g.trade.comms == nil || f == nil {
		return
	}
	if ok, _ := g.aidRequestOK(*f, now); !ok {
		return
	}
	hours := g.requestAid(f, now)
	g.worldNote("%s will send fighters to help; they should arrive in about %d hours.", f.Name, hours)
}

// cmdCloseComms ends the comms session.
func (g *Game) cmdCloseComms() { g.trade.comms = nil }

// consoleUsable reports building id is a radio console running on power.
func (g *Game) consoleUsable(id entity.EntityID) bool {
	b := g.bldgs.Ref(id)
	return b != nil && b.Kind == construction.RadioConsole && g.powered(id)
}

// operating reports id holds the comms session or the orbital trade it
// opened.
func (g *Game) operating(id entity.EntityID) bool {
	if c := g.trade.comms; c != nil && c.operator == id {
		return true
	}
	s := g.trade.session
	return s != nil && s.trader == 0 && s.negotiator == id
}

// doUseComms runs a UseComms job's Do phase: wait at the console, open
// the comms session, then hold it (Count 1) until it closes.
func (g *Game) doUseComms(id entity.EntityID, j job.Record) {
	if !g.consoleUsable(j.Target) {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	if j.Count == 1 {
		if g.operating(id) {
			return // on the radio
		}
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if g.sessionBusy(id) != "" {
		j.State, j.Fail = job.Fail, job.Reserved
		g.jobs.Set(id, j)
		return
	}
	if j.WorkLeft--; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.trade.comms = &commsSession{operator: id, console: j.Target}
	j.Count = 1
	g.jobs.Set(id, j)
}

// commsJob builds the forced UseComms job at the console at x, y, or the
// reason it can't start.
func (g *Game) commsJob(id entity.EntityID, x, y int) (job.Record, string) {
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	b := g.bldgs.Ref(bid)
	if b == nil || b.Kind != construction.RadioConsole {
		return job.Record{}, ReasonGone
	}
	if g.capacity(id, body.Talking) <= 0 {
		return job.Record{}, ReasonCantTalk
	}
	if !g.powered(bid) {
		return job.Record{}, ReasonNoPower
	}
	if why := g.sessionBusy(id); why != "" {
		return job.Record{}, why
	}
	if r := g.busyBy(id, bid); r != "" {
		return job.Record{}, r
	}
	_, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	if !g.reachable(id, spot[0], spot[1]) {
		return job.Record{}, ReasonUnreachable
	}
	return job.Record{Kind: job.UseComms, Target: bid, TX: spot[0], TY: spot[1], WorkLeft: tradeWait}, ""
}

// commsOrders lists "Use radio console" for a console at x, y (OrdersAt's
// add asks commsJob whether it is allowed).
func (g *Game) commsOrders(add func(string, command.Order, construction.Layer), x, y int) {
	if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
		if b := g.bldgs.Ref(bid); b != nil && b.Kind == construction.RadioConsole {
			add("Use radio console", command.OrderUseComms, construction.LayerObject)
		}
	}
}
