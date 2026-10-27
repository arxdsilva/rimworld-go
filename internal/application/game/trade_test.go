package game

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/randx"
)

// PLAY-024 trade simulation (agent S2): trader caravans, orbital traders,
// trade and comms sessions, deliveries and the save block.

// caravanGame is visitorGame (one neutral faction) with a trader caravan
// on the map.
func caravanGame(t *testing.T) (*Game, *traderCaravan) {
	t.Helper()
	g := visitorGame(t)
	now := uint64(g.clock.Now())
	if !g.caravanFits(now) {
		t.Fatal("a trader caravan should fit")
	}
	if !g.caravanArrive(now) {
		t.Fatal("the caravan did not come")
	}
	return g, g.trade.caravan
}

// optionAt is pawn's order labelled with prefix at (x, y) (nil: none).
func optionAt(g *Game, pawn entity.EntityID, x, y int, prefix string) *OrderOption {
	for _, o := range g.OrdersAt(pawn, x, y) {
		if strings.HasPrefix(o.Label, prefix) {
			return &o
		}
	}
	return nil
}

// openCaravanTrade puts a rested colonist beside the trader, orders
// "Trade with" and steps until the session opens; it returns the
// negotiator.
func openCaravanTrade(t *testing.T, g *Game, c *traderCaravan) entity.EntityID {
	t.Helper()
	neg := g.pawnIDs[0]
	g.SetNeeds(neg, 1000, 1000)
	tp, _ := g.positions.Get(c.Trader)
	x, y, ok := g.freeLandNear(tp.X, tp.Y, 2, [][2]int{{tp.X, tp.Y}})
	if !ok {
		t.Fatal("no tile beside the trader")
	}
	g.positions.Set(neg, Position{X: x, Y: y})
	g.invalidateCreatures()
	tp, _ = g.positions.Get(c.Trader)
	o := optionAt(g, neg, tp.X, tp.Y, "Trade with ")
	if o == nil || o.Disabled || o.Label != "Trade with "+g.name(c.Trader) {
		t.Fatalf("trade order %+v", o)
	}
	g.Enqueue(o.Cmd)
	psStepUntil(t, g, 2000, "trade session", func() bool { _, ok := g.TradeView(); return ok })
	return neg
}

// TestTraderCaravanArrivesStaysLeaves: a caravan of two to four comes from
// the neutral faction, one of them the trader with stock; they stay
// traderCaravanStay, stroll close to home and walk out with the goods.
func TestTraderCaravanArrivesStaysLeaves(t *testing.T) {
	t.Parallel()
	g, c := caravanGame(t)
	now := uint64(g.clock.Now())
	if c == nil || len(c.Members) < 2 || len(c.Members) > 4 || !slices.Contains(c.Members, c.Trader) {
		t.Fatalf("caravan %+v", c)
	}
	if c.LeaveAt != now+traderCaravanStay || c.Stock.Silver < 900 || len(c.Stock.Lines) == 0 {
		t.Fatalf("stay %d, stock %+v", c.LeaveAt-now, c.Stock)
	}
	if c.Kind != trade.BulkHauler && c.Kind != trade.Quartermaster {
		t.Fatalf("caravan kind %d", c.Kind)
	}
	for _, v := range g.worldSt.visitors {
		if v.LeaveAt != now+traderCaravanStay {
			t.Fatalf("visitor %d leaves at %d", v.ID, v.LeaveAt)
		}
	}
	if !reflect.DeepEqual(g.visitorIDs(), c.Members) {
		t.Fatalf("visitors %v, members %v", g.visitorIDs(), c.Members)
	}
	if g.caravanFits(now) || g.visitorsFit(now) {
		t.Fatal("another group fits while the caravan is here")
	}
	if st, ok := g.TradeStatus(); !ok || !st.CaravanVisiting || st.CaravanFaction != g.factionByID(c.Faction).Name || st.CaravanHoursLeft != 18 {
		t.Fatalf("trade status %+v %v", st, ok)
	}
	hx, hy := g.home()
	strolls := 0
	for i := 0; i < 20; i++ {
		g.Step(250)
		for _, id := range c.Members {
			if j, _ := g.jobs.Get(id); j.Kind == job.Visit && j.Active() {
				strolls++
				if max(abs(j.TX-hx), abs(j.TY-hy)) > traderCaravanRange+3 {
					t.Fatalf("a caravan member strolls to %d,%d, home %d,%d", j.TX, j.TY, hx, hy)
				}
			}
		}
	}
	if strolls == 0 {
		t.Fatal("the caravan never strolled")
	}
	gw := g.factionByID(c.Faction).Goodwill
	items := len(g.itemIDs)
	// the stay ends: they walk out and the caravan is over
	c.LeaveAt = uint64(g.clock.Now()) + 10
	for i := range g.worldSt.visitors {
		g.worldSt.visitors[i].LeaveAt = c.LeaveAt
	}
	g.Step(20)
	for i := 0; i < 600 && len(g.visitorIDs()) > 0; i++ {
		g.Step(10)
	}
	if len(g.visitorIDs()) != 0 || g.trade.caravan != nil {
		t.Fatalf("still here: visitors %v, caravan %v", g.visitorIDs(), g.trade.caravan)
	}
	if g.factionByID(c.Faction).Goodwill < gw-1 || len(g.itemIDs) > items {
		t.Fatal("a caravan that left in peace dropped goods or cost goodwill")
	}
	if _, ok := g.TradeStatus(); ok {
		t.Fatal("trade status after the caravan left")
	}
}

// TestKillingTraderDropsGoodsAndCostsGoodwill: a dead or captured trader
// drops its goods and silver forbidden where it fell, and its faction
// loses traderLostGoodwill.
func TestKillingTraderDropsGoodsAndCostsGoodwill(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"killed", "captured"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()
			g, c := caravanGame(t)
			f := g.factionByID(c.Faction)
			f.Goodwill = 20
			stock := cloneStock(c.Stock)
			tid := c.Trader
			if how == "killed" {
				g.die(tid, health.CauseInjury)
			} else {
				g.makePrisoner(tid, g.pawnIDs[0])
			}
			g.Step(1)
			if g.trade.caravan != nil {
				t.Fatal("the caravan outlived its trader")
			}
			if f.Goodwill != 20-traderLostGoodwill {
				t.Fatalf("goodwill %d, want %d", f.Goodwill, 20-traderLostGoodwill)
			}
			want := map[item.Kind]int{item.Silver: stock.Silver}
			for _, l := range stock.Lines {
				want[l.Kind] += l.Qty
			}
			got := map[item.Kind]int{}
			for _, iid := range g.itemIDs {
				if st := g.stacks.Ref(iid); st != nil && st.Forbidden && st.Carrier == 0 && want[st.Kind] > 0 {
					got[st.Kind] += st.Qty
				}
			}
			for k, n := range want {
				if got[k] < n {
					t.Fatalf("%s: %d forbidden on the ground, want %d", k, got[k], n)
				}
			}
			if _, ok := g.TradeStatus(); ok {
				t.Fatal("trade status shows a lost caravan")
			}
		})
	}
}

// TestTradeJobOpensSession: "Trade with <name>" walks the colonist up and
// opens the session after the wait; the view lists the trader, its stock
// and prices; CloseTrade ends the session and the job.
func TestTradeJobOpensSession(t *testing.T) {
	t.Parallel()
	g, c := caravanGame(t)
	neg := openCaravanTrade(t, g, c)
	v, _ := g.TradeView()
	f := g.factionByID(c.Faction)
	if v.Negotiator != neg || v.Trader != g.name(c.Trader) || v.Faction != f.Name || v.FactionID != f.ID || v.Orbital ||
		v.TraderSilver != c.Stock.Silver || v.HoursLeft <= 0 || v.TraderKind == "" || v.Goodwill != f.Goodwill {
		t.Fatalf("trade view %+v", v)
	}
	if len(v.Rows) < len(c.Stock.Lines) {
		t.Fatalf("%d rows for %d stock lines", len(v.Rows), len(c.Stock.Lines))
	}
	for _, r := range v.Rows {
		k, ok := item.ParseKind(r.Kind)
		if !ok || r.Kind == "silver" || r.TraderQty > 0 && (!r.WillBuy || r.BuyC <= 0) || r.SellC > r.BuyC {
			t.Fatalf("row %+v", r)
		}
		_ = k
	}
	if j, _ := g.jobs.Get(neg); j.Kind != job.Trade || j.State != job.Do || j.Count != 1 {
		t.Fatalf("negotiator job %+v", j)
	}
	// it holds while the session lasts
	g.Step(100)
	if _, ok := g.TradeView(); !ok {
		t.Fatal("the session closed by itself")
	}
	// a second colonist can't start another trade meanwhile
	if len(g.pawnIDs) > 1 {
		tp, _ := g.positions.Get(c.Trader)
		if o := optionAt(g, g.pawnIDs[1], tp.X, tp.Y, "Trade with "); o == nil || !o.Disabled {
			t.Fatalf("second trade order %+v", o)
		}
	}
	g.Enqueue(command.Command{Kind: command.CloseTrade})
	g.Step(2)
	if _, ok := g.TradeView(); ok {
		t.Fatal("CloseTrade left the session open")
	}
	if j, _ := g.jobs.Get(neg); j.Kind == job.Trade && j.Active() {
		t.Fatalf("the trade job outlived the session: %+v", j)
	}
}

// TestSessionClosesWhenTraderLeaves: the session closes when the
// negotiator goes down, and when the caravan's stay ends.
func TestSessionClosesWhenTraderLeaves(t *testing.T) {
	t.Parallel()
	g, c := caravanGame(t)
	neg := openCaravanTrade(t, g, c)
	v, _ := g.vital.Get(neg)
	down := v
	down.Status = health.Downed
	g.vital.Set(neg, down)
	g.Step(1)
	if _, ok := g.TradeView(); ok {
		t.Fatal("the session stayed open with the negotiator down")
	}
	g.vital.Set(neg, v)
	g.Step(5)
	neg = openCaravanTrade(t, g, c)
	c.LeaveAt = uint64(g.clock.Now()) + 5
	g.Step(10)
	if _, ok := g.TradeView(); ok {
		t.Fatal("the session stayed open after the trader's stay ended")
	}
	if j, _ := g.jobs.Get(neg); j.Kind == job.Trade && j.Active() {
		t.Fatalf("the trade job outlived the session: %+v", j)
	}
	if tp, _ := g.positions.Get(c.Trader); optionAt(g, neg, tp.X, tp.Y, "Trade with ") != nil {
		if o := optionAt(g, neg, tp.X, tp.Y, "Trade with "); !o.Disabled || o.Reason != ReasonNotTrading {
			t.Fatalf("order on a leaving trader %+v", o)
		}
	}
}

// totalOf counts every stack of kind k on the map (carried too).
func totalOf(g *Game, k item.Kind) int {
	n := 0
	for _, iid := range g.itemIDs {
		if st := g.stacks.Ref(iid); st != nil && st.Kind == k {
			n += st.Qty
		}
	}
	return n
}

// stockQty is st's quantity of kind k.
func stockQty(st trade.Stock, k item.Kind) int {
	for _, l := range st.Lines {
		if l.Kind == k {
			return l.Qty
		}
	}
	return 0
}

// tradeYard designates a stockpile beside the negotiator and puts silver
// and components on it.
func tradeYard(t *testing.T, g *Game, neg entity.EntityID, silver, comps int) {
	t.Helper()
	p, _ := g.positions.Get(neg)
	var cells [][2]int
	for d := 1; d <= 6 && len(cells) < 2; d++ {
		for _, o := range [][2]int{{d, 0}, {-d, 0}, {0, d}, {0, -d}} {
			x, y := p.X+o[0], p.Y+o[1]
			if g.inset(x, y) && g.dryWalkable(x, y) && g.itemAt(x, y) == 0 && !g.pawnOn(x, y) && len(cells) < 2 {
				cells = append(cells, [2]int{x, y})
			}
		}
	}
	if len(cells) < 2 {
		t.Fatal("no room for a stockpile")
	}
	for _, c := range cells {
		g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: c[0], Y: c[1], X2: c[0], Y2: c[1]})
	}
	g.ApplyPending()
	for _, c := range cells {
		if !g.stockpiled(c[0], c[1]) {
			t.Fatalf("%d,%d is no stockpile", c[0], c[1])
		}
	}
	spawn(t, g, item.Silver, silver, cells[0][0], cells[0][1])
	spawn(t, g, item.Components, comps, cells[1][0], cells[1][1])
}

// TestProposeTradeMovesGoods: an accepted deal takes the colony's sold
// goods and silver, drops the purchases beside the negotiator and moves
// the trader's stock and silver, exactly as Settle priced it.
func TestProposeTradeMovesGoods(t *testing.T) {
	t.Parallel()
	g, c := caravanGame(t)
	neg := openCaravanTrade(t, g, c)
	tradeYard(t, g, neg, 900, 10)
	buy := c.Stock.Lines[0]
	offer := []trade.Delta{{Kind: buy.Kind, Qty: 1}, {Kind: item.Components, Qty: -4}}
	if buy.Kind > item.Components {
		offer[0], offer[1] = offer[1], offer[0]
	}
	f := g.factionByID(c.Faction)
	gd := g.colonyGoods(false)
	if gd.counts[item.Components] != 10 || gd.counts[item.Silver] != 900 {
		t.Fatalf("colony goods %v", gd.counts)
	}
	res := trade.Settle(offer, gd.counts, c.Stock, g.termsFor(neg, f), false)
	if !res.OK() {
		t.Fatalf("settle: %s", res.Reject)
	}
	silver0, comps0, bought0 := totalOf(g, item.Silver), totalOf(g, item.Components), totalOf(g, buy.Kind)
	tsilver0, tstock0 := c.Stock.Silver, buy.Qty
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: EncodeTradeOffer(offer)})
	g.ApplyPending()
	v, ok := g.TradeView()
	if !ok || v.Reason != "" {
		t.Fatalf("view %+v, %v", v, ok)
	}
	if got := totalOf(g, item.Silver); got != silver0+res.Silver {
		t.Fatalf("silver %d, want %d", got, silver0+res.Silver)
	}
	if got := totalOf(g, item.Components); got != comps0-4 {
		t.Fatalf("components %d, want %d", got, comps0-4)
	}
	if got := totalOf(g, buy.Kind); got != bought0+1 {
		t.Fatalf("%s %d, want %d", buy.Kind, got, bought0+1)
	}
	if c.Stock.Silver != tsilver0-res.Silver || stockQty(c.Stock, buy.Kind) != tstock0-1 || stockQty(c.Stock, item.Components) < 4 {
		t.Fatalf("trader stock %+v", c.Stock)
	}
	// the purchase lies by the negotiator, not forbidden
	np, _ := g.positions.Get(neg)
	found := false
	for _, iid := range g.itemIDs {
		if st := g.stacks.Ref(iid); st != nil && st.Kind == buy.Kind && !st.Forbidden && max(abs(st.X-np.X), abs(st.Y-np.Y)) <= 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("no purchase beside the negotiator")
	}
	// a gift: goodwill for goods, no silver back
	f.Goodwill = 0
	silver1 := totalOf(g, item.Silver)
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: EncodeTradeOffer([]trade.Delta{{Kind: item.Silver, Qty: -200}}), Enabled: true})
	g.ApplyPending()
	if f.Goodwill != faction.GiftGoodwill(200) || totalOf(g, item.Silver) != silver1-200 {
		t.Fatalf("gift: goodwill %d, silver %d -> %d", f.Goodwill, silver1, totalOf(g, item.Silver))
	}
}

// TestProposeTradeRejectsStaleCounts: an offer made against counts that
// changed while the dialog was open is refused with the reason, and
// nothing moves.
func TestProposeTradeRejectsStaleCounts(t *testing.T) {
	t.Parallel()
	g, c := caravanGame(t)
	neg := openCaravanTrade(t, g, c)
	tradeYard(t, g, neg, 50, 10)
	offer := []trade.Delta{{Kind: item.Components, Qty: -10}}
	gd := g.colonyGoods(false)
	g.takeGoods(gd, item.Components, 5) // half is used up meanwhile
	stock := cloneStock(c.Stock)
	comps := totalOf(g, item.Components)
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: EncodeTradeOffer(offer)})
	g.ApplyPending()
	v, _ := g.TradeView()
	if v.Reason != trade.RejectOverStock.String() {
		t.Fatalf("reason %q", v.Reason)
	}
	if totalOf(g, item.Components) != comps || !reflect.DeepEqual(c.Stock, stock) {
		t.Fatal("a refused offer moved goods")
	}
	// a malformed offer is refused too
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: "x:y"})
	g.ApplyPending()
	if v, _ := g.TradeView(); v.Reason != ReasonOfferBroken {
		t.Fatalf("reason %q", v.Reason)
	}
}

// orbitalGame is a small map with a fuelled generator feeding a radio
// console (spot south of it) and a rested colonist.
func orbitalGame(t *testing.T) (g *Game, gen, console, pawn entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 1, Width: 40, Height: 24})
	gen = psFuelled(t, g, 2, 2)
	psConduits(t, g, 4, 2, 12)
	console = buildNow(t, g, construction.RadioConsole, item.Steel, construction.South, 12, 5)
	pawn = psWorker(t, g, 12, 9)
	g.Step(powerInterval)
	return g, gen, console, pawn
}

// openComms orders pawn to the console and steps until the comms session
// opens.
func openComms(t *testing.T, g *Game, pawn entity.EntityID) {
	t.Helper()
	o := optionAt(g, pawn, 12, 5, "Use radio console")
	if o == nil || o.Disabled {
		t.Fatalf("console order %+v", o)
	}
	g.Enqueue(o.Cmd)
	psStepUntil(t, g, 600, "comms session", func() bool { _, ok := g.CommsView(); return ok })
}

// TestOrbitalNeedsPoweredConsole: the orbital trader fits only while a
// radio console runs on power, and the console's order says why not.
func TestOrbitalNeedsPoweredConsole(t *testing.T) {
	t.Parallel()
	g, gen, console, pawn := orbitalGame(t)
	now := uint64(g.clock.Now())
	if !g.powered(console) || !g.orbitalFits(now) {
		t.Fatalf("powered %v, fits %v", g.powered(console), g.orbitalFits(now))
	}
	g.setFuel(gen, 0)
	g.Step(2 * powerInterval)
	now = uint64(g.clock.Now())
	if g.powered(console) || g.orbitalFits(now) {
		t.Fatal("fits without power")
	}
	if o := optionAt(g, pawn, 12, 5, "Use radio console"); o == nil || !o.Disabled || o.Reason != ReasonNoPower {
		t.Fatalf("unpowered console order %+v", o)
	}
	if g.FireIncident(incident.DefOf(incident.OrbitalTrader).Key) {
		t.Fatal("fired without power")
	}
	g.setFuel(gen, g.fuelCap(gen))
	g.Step(2 * powerInterval)
	if !g.FireIncident(incident.DefOf(incident.OrbitalTrader).Key) || g.trade.orbital == nil {
		t.Fatal("the ship did not come with power")
	}
	o := g.trade.orbital
	if o.Kind != trade.CurioMerchant || o.Until != uint64(g.clock.Now())+orbitalStay || o.Stock.Silver < 1600 {
		t.Fatalf("ship %+v", o)
	}
	if g.orbitalFits(uint64(g.clock.Now())) {
		t.Fatal("a second ship fits")
	}
	st, ok := g.TradeStatus()
	if !ok || !st.Present || st.Ship != o.Name || st.HoursLeft != 36 || st.CallOK || st.CallReason != ReasonNoBeacon || st.CaravanVisiting {
		t.Fatalf("trade status %+v", st)
	}
	o.Until = uint64(g.clock.Now()) + 3
	g.Step(5)
	if g.trade.orbital != nil {
		t.Fatal("the ship outstayed its time")
	}
}

// TestOrbitalGoodsOnlyNearBeacon: the ship can't be called without a
// trade beacon; with one, only unforbidden goods within its reach count.
func TestOrbitalGoodsOnlyNearBeacon(t *testing.T) {
	t.Parallel()
	g, _, _, pawn := orbitalGame(t)
	g.orbitalArrive(uint64(g.clock.Now()))
	spawn(t, g, item.Steel, 30, 24, 14)   // within reach
	spawn(t, g, item.Silver, 100, 23, 15) // within reach
	spawn(t, g, item.Steel, 40, 34, 14)   // too far
	fid, _ := g.SpawnStack(item.Steel, 5, 22, 16)
	st, _ := g.stacks.Get(fid)
	st.Forbidden = true
	g.stacks.Set(fid, st)
	openComms(t, g, pawn)
	cv, _ := g.CommsView()
	if !cv.Orbital.Present || cv.Orbital.CallOK || cv.Orbital.CallReason != ReasonNoBeacon || cv.BeaconSilver != 0 {
		t.Fatalf("comms without a beacon %+v", cv)
	}
	g.Enqueue(command.Command{Kind: command.CommsCallTrader})
	g.ApplyPending()
	if _, ok := g.TradeView(); ok {
		t.Fatal("called the ship without a beacon")
	}
	buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, 22, 14)
	cv, _ = g.CommsView()
	if !cv.Orbital.CallOK || cv.BeaconSilver != 100 {
		t.Fatalf("comms with a beacon %+v", cv)
	}
	g.Enqueue(command.Command{Kind: command.CommsCallTrader})
	g.ApplyPending()
	v, ok := g.TradeView()
	if !ok || !v.Orbital || v.ColonySilver != 100 || v.Trader != g.trade.orbital.Name || v.FactionID != -1 {
		t.Fatalf("orbital trade view %+v", v)
	}
	if _, ok := g.CommsView(); ok {
		t.Fatal("comms stays open beside the trade")
	}
	steel := -1
	for _, r := range v.Rows {
		if r.Kind == item.Steel.String() {
			steel = r.ColonyQty
		}
	}
	if steel != 30 {
		t.Fatalf("steel within reach %d, want 30", steel)
	}
	g.Step(50)
	if _, ok := g.TradeView(); !ok {
		t.Fatal("the orbital session closed by itself")
	}
	if j, _ := g.jobs.Get(pawn); j.Kind != job.UseComms || j.Count != 1 {
		t.Fatalf("operator job %+v", j)
	}
}

// TestOrbitalPurchaseDropsPod: an orbital purchase comes down in a drop
// pod near the beacon tradePodFall ticks later: a shell and the goods.
func TestOrbitalPurchaseDropsPod(t *testing.T) {
	t.Parallel()
	g, _, _, pawn := orbitalGame(t)
	g.orbitalArrive(uint64(g.clock.Now()))
	buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, 22, 14)
	spawn(t, g, item.Silver, 3000, 23, 15)
	openComms(t, g, pawn)
	g.Enqueue(command.Command{Kind: command.CommsCallTrader})
	g.ApplyPending()
	o := g.trade.orbital
	buy := o.Stock.Lines[0]
	offer := []trade.Delta{{Kind: buy.Kind, Qty: 1}}
	res := trade.Settle(offer, g.colonyGoods(true).counts, o.Stock, g.termsFor(pawn, nil), false)
	if !res.OK() {
		t.Fatalf("settle: %s", res.Reject)
	}
	before := totalOf(g, buy.Kind)
	g.Enqueue(command.Command{Kind: command.ProposeTrade, Name: EncodeTradeOffer(offer)})
	g.ApplyPending()
	if v, _ := g.TradeView(); v.Reason != "" {
		t.Fatalf("refused: %s", v.Reason)
	}
	if totalOf(g, item.Silver) != 3000+res.Silver || totalOf(g, buy.Kind) != before {
		t.Fatal("the purchase arrived at once, or the silver did not move")
	}
	if len(g.trade.pods) != 1 || g.trade.pods[0].Landed {
		t.Fatalf("pods %+v", g.trade.pods)
	}
	p := g.trade.pods[0]
	if dx, dy := p.X-22, p.Y-14; dx*dx+dy*dy > podRadius*podRadius*2 {
		t.Fatalf("pod at %d,%d, far from the beacon", p.X, p.Y)
	}
	if vs := g.arrivalViews(uint64(g.clock.Now())); len(vs) != 1 || vs[0].X != p.X || vs[0].Landed {
		t.Fatalf("arrival views %+v", vs)
	}
	g.Step(tradePodFall + 2)
	if !g.trade.pods[0].Landed || totalOf(g, buy.Kind) != before+1 {
		t.Fatalf("pod %+v, %s %d", g.trade.pods[0], buy.Kind, totalOf(g, buy.Kind))
	}
	if b := g.bldgs.Ref(g.layerBldgAt(p.X, p.Y, construction.LayerObject)); b == nil || b.Kind != construction.PodShell {
		t.Fatal("no pod shell where it landed")
	}
	g.Step(arrivalKeep)
	if len(g.trade.pods) != 0 {
		t.Fatal("a landed pod is never forgotten")
	}
}

// commsFactions gives an orbital game the default factions, the first
// human one neutral at goodwill 0.
func commsFactions(g *Game) *faction.Faction {
	g.ng.factions = faction.Generate(faction.DefaultSlots(), randx.New(7))
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		if f.Type.Human() && !faction.DefOf(f.Type).Fixed {
			f.Goodwill, f.Relation = 0, faction.Neutral
			return f
		}
	}
	return nil
}

// TestCommsGiftRaisesGoodwill: silver sent over the radio comes from
// beacon reach and buys goodwill (one per 40, capped); more than there is
// changes nothing.
func TestCommsGiftRaisesGoodwill(t *testing.T) {
	t.Parallel()
	g, _, _, pawn := orbitalGame(t)
	f := commsFactions(g)
	buildNow(t, g, construction.TradeBeacon, item.Steel, construction.South, 22, 14)
	spawn(t, g, item.Silver, 300, 23, 15)
	spawn(t, g, item.Silver, 500, 35, 20) // out of reach
	openComms(t, g, pawn)
	cv, _ := g.CommsView()
	var row *CommsFactionView
	for i := range cv.Factions {
		if cv.Factions[i].ID == f.ID {
			row = &cv.Factions[i]
		}
	}
	if row == nil || !row.Gift.Allowed || row.Gift.Cost != faction.GiftSilverPerGoodwill || row.Aid.Allowed || row.Aid.Reason != faction.ReasonNotFriend {
		t.Fatalf("faction row %+v", row)
	}
	g.Enqueue(command.Command{Kind: command.CommsGift, Index: f.ID, Count: 400}) // more than in reach
	g.ApplyPending()
	if f.Goodwill != 0 || totalOf(g, item.Silver) != 800 {
		t.Fatal("a gift beyond the silver in reach went through")
	}
	g.Enqueue(command.Command{Kind: command.CommsGift, Index: f.ID, Count: 200})
	g.ApplyPending()
	if f.Goodwill != 5 || totalOf(g, item.Silver) != 600 {
		t.Fatalf("goodwill %d, silver %d", f.Goodwill, totalOf(g, item.Silver))
	}
	if cv, _ := g.CommsView(); cv.BeaconSilver != 100 {
		t.Fatalf("beacon silver %d", cv.BeaconSilver)
	}
}

// TestCommsRequestCaravan: a caravan request costs goodwill, starts the
// cooldown, schedules the caravan 6-12 hours on and it comes.
func TestCommsRequestCaravan(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	var f *faction.Faction
	for i := range g.ng.factions {
		if g.ng.factions[i].Relation == faction.Neutral && g.ng.factions[i].Type.Human() {
			f = &g.ng.factions[i]
		}
	}
	f.Goodwill = 30
	g.Enqueue(command.Command{Kind: command.CommsRequestCaravan, Index: f.ID})
	g.ApplyPending()
	if f.Goodwill != 30 || len(g.trade.requests) != 0 {
		t.Fatal("a request without a comms session went through")
	}
	g.trade.comms = &commsSession{operator: g.pawnIDs[0]} // the console itself is TestOrbitalGoodsOnlyNearBeacon's
	now := uint64(g.clock.Now())
	g.Enqueue(command.Command{Kind: command.CommsRequestCaravan, Index: f.ID})
	g.Enqueue(command.Command{Kind: command.CommsRequestCaravan, Index: f.ID}) // a second ask is refused
	g.ApplyPending()
	if f.Goodwill != 30-faction.CaravanCost || len(g.trade.requests) != 1 {
		t.Fatalf("goodwill %d, requests %+v", f.Goodwill, g.trade.requests)
	}
	r := g.trade.requests[0]
	if r.Faction != f.ID || r.At < now+6*clock.TicksPerHour || r.At > now+12*clock.TicksPerHour {
		t.Fatalf("request %+v (now %d)", r, now)
	}
	if cd := g.cooldownOf(f.ID); cd.CaravanUntil != now+faction.CaravanCooldownDays*clock.TicksPerDay {
		t.Fatalf("cooldown %+v", cd)
	}
	if ok, why := g.caravanRequestOK(*f, now); ok || why != faction.ReasonCoolingOff {
		t.Fatalf("request again: %v %q", ok, why)
	}
	g.trade.requests[0].At = uint64(g.clock.Now()) + 5
	g.Step(10)
	c := g.trade.caravan
	if c == nil || c.Faction != f.ID || len(g.trade.requests) != 0 {
		t.Fatalf("caravan %+v, requests %+v", c, g.trade.requests)
	}
	if ok, why := g.caravanRequestOK(*f, uint64(g.clock.Now())); ok || why != faction.ReasonCoolingOff {
		t.Fatalf("request while cooling: %v %q", ok, why)
	}
}

// tradeRich gives a caravan game every piece of saved trade state.
func tradeRich(t *testing.T) *Game {
	t.Helper()
	g, c := caravanGame(t)
	now := uint64(g.clock.Now())
	g.orbitalArrive(now)
	g.setCooldown(c.Faction, faction.Cooldowns{CaravanUntil: now + 1000, AidUntil: now + 2000})
	g.addRequest(caravanRequest{Faction: c.Faction, At: now + 5000})
	g.sendTradePod([]trade.Line{{Kind: item.Steel, Qty: 7}}, now)
	return g
}

// TestTradeSaveRoundTrip: the caravan, ship, pods, request and cooldowns
// survive a save; an open session does not; the loaded game runs on
// identically.
func TestTradeSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := tradeRich(t)
	g.trade.session = &tradeSession{negotiator: g.pawnIDs[0], trader: g.trade.caravan.Trader}
	want := g.exportTrade()
	if want == nil || want.Caravan == nil || want.Orbital == nil || len(want.Pods) != 1 || len(want.Requests) != 1 || len(want.Cooldowns) != 1 {
		t.Fatalf("export %+v", want)
	}
	before := g.StateHash()
	h := saveLoad(t, g)
	if got := h.exportTrade(); !reflect.DeepEqual(got, want) {
		t.Fatalf("trade block\n got %+v\nwant %+v", got, want)
	}
	if h.trade.session != nil || h.trade.comms != nil {
		t.Fatal("a session survived the load")
	}
	if h.StateHash() != before {
		t.Fatal("the hash moved over a round trip")
	}
	g.trade.session = nil
	g.Step(300)
	h.Step(300)
	if g.StateHash() != h.StateHash() || !reflect.DeepEqual(g.exportTrade(), h.exportTrade()) {
		t.Fatal("the loaded game drifted")
	}
}

// TestTradeSameSeed: two colonies from the same seed run the same trade.
func TestTradeSameSeed(t *testing.T) {
	t.Parallel()
	a, b := tradeRich(t), tradeRich(t)
	for i := 0; i < 6; i++ {
		a.Step(500)
		b.Step(500)
		if a.StateHash() != b.StateHash() {
			t.Fatalf("hashes diverged after %d ticks", (i+1)*500)
		}
	}
	if !reflect.DeepEqual(a.exportTrade(), b.exportTrade()) {
		t.Fatal("trade state diverged")
	}
}

// TestTradeStatusReportsShipAndCaravan: TradeStatus reports the ship and
// the caravan without any session open.
func TestTradeStatusReportsShipAndCaravan(t *testing.T) {
	t.Parallel()
	g := tradeRich(t)
	st, ok := g.TradeStatus()
	if !ok || !st.Present || !st.CaravanVisiting || st.Ship == "" || st.CaravanFaction == "" || st.HoursLeft != 36 || st.CaravanHoursLeft != 18 {
		t.Fatalf("status %+v %v", st, ok)
	}
	if _, open := g.TradeView(); open {
		t.Fatal("a session is open")
	}
}
