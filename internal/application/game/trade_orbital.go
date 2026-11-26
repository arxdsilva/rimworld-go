// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/beacon"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/store"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Orbital traders (PLAY-024, agent S2): incident.OrbitalTrader puts a
// curio merchant's ship in radio range for orbitalStay. A constructed
// radio console admits the event even during an outage; only a powered
// console can discover and call the ship. A colonist at the console calls it
// from the comms dialog (comms.go); only goods a trade beacon reaches
// (beaconCells) can be traded, and purchases come down in a drop pod near
// the first beacon tradePodFall ticks later (streamTrade picks the spot).

// orbitalStay is how long an orbital ship stays in radio range.
const orbitalStay = 40000 // wiki: Trade (orbital traders remain 40,000 ticks)

// beaconGoods is what the orbital ship can trade: the loose stacks on the
// cells a trade beacon reaches (the ground index; nothing without one).
func beaconGoods(g *Game) store.TradeGoods {
	return store.CollectBeacon(beaconCells(g), g.grid.Width(),
		func(x, y int) []entity.EntityID { return g.idx.ground.Loose(haulHost{g}, x, y) },
		g.stacks.Get, g.fogged)
}

// beaconCells is the powered beacons' reach (beacon.Reach; rock and built walls cut it),
// cached under bit 254 keyed structVer+pwc.vers (build; any power change, flicks too; both grow).
func beaconCells(g *Game) tileSet {
	return g.cellSet([4]uint64{3: 1 << 62}).get(g, g.structVer+g.pwc.vers, func() tileSet {
		w, h, walls := g.grid.Width(), g.grid.Height(), g.buildingCells(construction.Wall)
		set := newTileSet(w * h)
		for _, bid := range g.beaconIDs() {
			b := g.bldgs.Ref(bid)
			beacon.Reach(w, h, b.X, b.Y, func(x, y int) bool { return walls.has(y*w+x) || terrain.IsRockWall(g.terr.At(x, y)) }, set.add)
		}
		return set
	})
}

// shipNames are the orbital traders' ship names (original).
var shipNames = [...]string{
	"Brass Heron", "Quiet Lantern", "Salt Comet", "Far Meridian", "Copper Gull",
	"Patient Ember", "Long Wake", "Tin Albatross", "Second Dawn", "Velvet Anchor",
}

// orbitalFits admits a passing ship when a radio exists, even during
// an outage. The arrival is silent until console power is restored.
func (g *Game) orbitalFits(now uint64) bool {
	if g.trade.orbital != nil || len(g.pawnIDs) == 0 {
		return false
	}
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.RadioConsole {
			return true
		}
	}
	return false
}

// orbitalArrive puts an orbital trader in range; false when it could not.
func (g *Game) orbitalArrive(now uint64) bool {
	if g.trade.orbital != nil {
		return false
	}
	r := g.stream(streamTrade)
	name := shipNames[r.IntN(len(shipNames))]
	stock := trade.Generate(g.sk.content.Items, trade.CurioMerchant, faction.Spacer, r)
	g.trade.orbital = &orbitalShip{Name: name, Kind: trade.CurioMerchant, Stock: stock, Until: now + orbitalStay}
	if g.poweredConsole() != 0 {
		letterf(g, letters.Info, "Trade ship", 0, "The %s, a %s, is in radio range for about 16 hours. Call it from a powered radio console; only goods within reach of a powered trade beacon can be traded.",
			name, tradeview.LowerLabel(trade.DefOf(trade.CurioMerchant).Label))
	}
	return true
}

// orbitalLeaves: the ship's time is up.
func (g *Game) orbitalLeaves() {
	g.worldNote("The trade ship %s has left radio range.", g.trade.orbital.Name)
	g.trade.orbital = nil
}

// poweredConsole is the first radio console running on power (0: none).
func (g *Game) poweredConsole() entity.EntityID {
	for id := range beacon.Powered(g.bldgIDs, construction.RadioConsole, g.bldgs.Ref, g.powered) {
		return id
	}
	return 0
}

// orbitalView is the ship in radio range for the comms dialog and the
// trade alerts.
func (g *Game) orbitalView(now uint64) OrbitalView {
	o := g.trade.orbital
	if o == nil {
		return tradeview.OrbitalStatus("", 0, 0, now, false, false, false)
	}
	return tradeview.OrbitalStatus(o.Name, o.Kind, o.Until, now, true, g.poweredConsole() != 0, len(g.beaconIDs()) != 0)
}

// sendTradePod sends lines down in a drop pod near the first trade beacon
// (home() without one), landing tradePodFall ticks from now.
func (g *Game) sendTradePod(lines []trade.Line, now uint64) {
	if len(lines) == 0 {
		return
	}
	cx, cy := g.home()
	if bs := g.beaconIDs(); len(bs) > 0 {
		b := g.bldgs.Ref(bs[0])
		cx, cy = b.X, b.Y
	}
	var taken [][2]int
	for _, p := range g.trade.pods {
		if !p.Landed {
			taken = append(taken, [2]int{p.X, p.Y})
		}
	}
	x, y, ok := g.podSpotFrom(g.stream(streamTrade), cx, cy, taken) // arrival.go
	if !ok {
		x, y = cx, cy
	}
	g.trade.pods = append(g.trade.pods, tradePod{X: x, Y: y, Tick: now + tradePodFall, Items: lines})
}

// tradePodsTick lands the pods whose time has come and forgets them
// arrivalKeep after landing.
func (g *Game) tradePodsTick(now uint64) {
	kept := g.trade.pods[:0]
	for _, p := range g.trade.pods {
		if !p.Landed && now >= p.Tick {
			p.Landed = true
			g.landTradePod(p)
		}
		if p.Landed && now >= p.Tick+arrivalKeep {
			continue
		}
		kept = append(kept, p)
	}
	clear(g.trade.pods[len(kept):])
	g.trade.pods = kept
	if len(g.trade.pods) == 0 {
		g.trade.pods = nil
	}
}

// landTradePod: the shell, and the goods beside it.
func (g *Game) landTradePod(p tradePod) {
	g.BuildNow(construction.DefOf(construction.PodShell).Name, item.Steel.String(), "south", p.X, p.Y)
	bx, by := p.X, p.Y
	for _, d := range [...][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		if g.dryWalkable(p.X+d[0], p.Y+d[1]) {
			bx, by = p.X+d[0], p.Y+d[1]
			break
		}
	}
	for _, l := range p.Items {
		g.dropGoods(l, bx, by, false)
	}
	if p.Reward {
		g.message(bx, by, "A drop pod with your quest reward has landed.")
	} else {
		g.message(bx, by, "A drop pod with your purchases has landed.")
	}
}

// tradePodViews are the trade drop pods for the renderer (ArrivalView).
func (g *Game) tradePodViews(now uint64) []ArrivalView {
	var out []ArrivalView
	for _, p := range g.trade.pods {
		if !p.Landed || now < p.Tick+arrivalKeep {
			out = append(out, ArrivalView{X: p.X, Y: p.Y, Tick: p.Tick, Landed: p.Landed})
		}
	}
	return out
}
