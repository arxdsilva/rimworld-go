package game

import (
	"fmt"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

// Orbital traders (PLAY-024, agent S2): incident.OrbitalTrader puts a
// curio merchant's ship in radio range for orbitalStay; it fits only while
// a radio console is powered. A colonist at a powered console calls it
// from the comms dialog (comms.go); only goods within beaconRadius of a
// trade beacon can be traded, and purchases come down in a drop pod near
// the first beacon tradePodFall ticks later (streamTrade picks the spot).

// shipNames are the orbital traders' ship names (original).
var shipNames = [...]string{
	"Brass Heron", "Quiet Lantern", "Salt Comet", "Far Meridian", "Copper Gull",
	"Patient Ember", "Long Wake", "Tin Albatross", "Second Dawn", "Velvet Anchor",
}

// orbitalFits is the map's check for an orbital trader: people, a
// powered radio console, and no ship already in range.
func (g *Game) orbitalFits(now uint64) bool {
	return g.trade.orbital == nil && len(g.pawnIDs) > 0 && g.poweredConsole() != 0
}

// orbitalArrive puts an orbital trader in range; false when it could not.
func (g *Game) orbitalArrive(now uint64) bool {
	if g.trade.orbital != nil {
		return false
	}
	r := g.stream(streamTrade)
	name := shipNames[r.IntN(len(shipNames))]
	stock := trade.Generate(trade.CurioMerchant, faction.Spacer, r)
	g.trade.orbital = &orbitalShip{Name: name, Kind: trade.CurioMerchant, Stock: stock, Until: now + orbitalStay}
	g.letter(letterInfo, "Trade ship", fmt.Sprintf("The %s, a %s, is in radio range for about a day and a half. Call it from a powered radio console; only goods within reach of a trade beacon can be traded.",
		name, lower(trade.DefOf(trade.CurioMerchant).Label)), 0)
	return true
}

// orbitalLeaves: the ship's time is up.
func (g *Game) orbitalLeaves() {
	g.worldNote("The trade ship %s has left radio range.", g.trade.orbital.Name)
	g.trade.orbital = nil
}

// poweredConsole is the first radio console running on power (0: none).
func (g *Game) poweredConsole() entity.EntityID {
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.RadioConsole && g.powered(id) {
			return id
		}
	}
	return 0
}

// orbitalView is the ship in radio range for the comms dialog and the
// trade alerts.
func (g *Game) orbitalView(now uint64) OrbitalView {
	o := g.trade.orbital
	if o == nil {
		return OrbitalView{CallReason: "no ship in range"}
	}
	v := OrbitalView{Present: true, Ship: o.Name, Kind: lower(trade.DefOf(o.Kind).Label), HoursLeft: hoursLeft(o.Until, now), CallOK: true}
	if len(g.beaconIDs()) == 0 {
		v.CallOK, v.CallReason = false, "needs a trade beacon"
	}
	return v
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
		g.dropGoods(l.Kind, l.Qty, bx, by, false)
	}
	g.message(bx, by, "A drop pod with your purchases has landed.")
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
