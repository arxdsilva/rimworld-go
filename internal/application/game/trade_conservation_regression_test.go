// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestSplitDebit(t *testing.T) {
	g, c := caravanGame(t)
	neg := openCaravanTrade(t, g, c)
	tradeYard(t, g, neg, 900, 10)
	_ = neg
	gd := g.colonyGoods(false)
	ids := gd.IDs(item.Components)
	if len(ids) != 1 {
		t.Fatalf("fixture: %v", ids)
	}
	id := ids[0]
	before, _ := g.stacks.Get(id)
	g.cmdProposeTrade(trade.EncodeOffer([]trade.Delta{{Kind: item.Components, Qty: -2, StackID: uint64(id)}, {Kind: item.Components, Qty: -2}}), false)
	after, ok := g.stacks.Get(id)
	if g.trade.session.reason != "" {
		t.Fatalf("offer refused: %s", g.trade.session.reason)
	}
	if !ok || after.Qty != before.Qty-4 {
		t.Fatalf("accepted sale of 4: original %d, remaining %d (want %d)", before.Qty, after.Qty, before.Qty-4)
	}
}
func TestSecondGroupLeave(t *testing.T) {
	g, c := caravanGame(t)
	if !g.caravanArrive(uint64(g.clock.Now())) {
		t.Fatal("second caravan arrival failed")
	}
	second := g.trade.others[0]
	visitsOf(g).LeaveGroup(second.GroupID, visit.LeaveDismissed, uint64(g.clock.Now()))
	a, _ := visitsOf(g).GroupOf(c.Trader)
	b, _ := visitsOf(g).GroupOf(second.Trader)
	if a.Phase == visit.PhaseLeaving || b.Phase != visit.PhaseLeaving {
		t.Fatalf("first phase %v, requested second phase %v (want leaving)", a.Phase, b.Phase)
	}
}
func TestPackLedgerSurvivesHumanDeparture(t *testing.T) {
	g, c := caravanGame(t)
	pack := caravanPack(g, c)
	if pack == 0 {
		t.Fatal("fixture: no pack")
	}
	for _, id := range slices.Clone(c.Members) {
		visitBodies{g: g}.Depart(id)
		g.traderVisitorGone(id)
	}
	if !g.animals.Has(pack) {
		t.Fatal("fixture: pack absent")
	}
	if caravanByGroup(&g.trade, c.GroupID) == nil {
		t.Fatalf("live pack %d lost its group's ledger after all humans departed", pack)
	}
}
func TestLeavingTraderClosed(t *testing.T) {
	g, c := caravanGame(t)
	visitsOf(g).LeaveGroup(c.GroupID, visit.LeaveDismissed, uint64(g.clock.Now()))
	if g.traderOpen(c.Trader) {
		t.Fatal("dismissed trader still open")
	}
}
func TestSettlementPhysicalPrice(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	st := item.Stack{Kind: item.Parka, Qty: 1, Quality: quality.Stored(quality.Awful), Stuff: item.Cloth}
	st.Wear = st.MaxHP() / 2
	travelOf(g).Carry(cid, st)
	stock, _ := fleetOf(g).Market().At(tile)
	terms := g.termsFor(pawn, g.factionOf(g.settlementFaction(tile)))
	terms.ImproveCenti = 200
	want := trade.PriceMarket(trade.Goods{Kind: st.Kind}, item.StackValue(st), stock.Stock.Kind, trade.Sell, terms)
	res, why := fleetOf(g).Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: st.Kind, Qty: -1}}), false)
	if why != "" {
		t.Fatalf("trade refused: %s", why)
	}
	if res.SilverCenti != want {
		t.Fatalf("worn awful parka priced at %d cents, actual physical value requires %d", res.SilverCenti, want)
	}
}
func TestRestockDuringLiveVisit(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	f := fleetOf(g)
	stock, _ := f.Market().At(tile)
	// Recreate a visit beginning one tick before the scheduled restock.
	g.clock.Advance(int(stock.RestockAt - 1 - uint64(g.clock.Now())))
	maps.DevArrive(f, entity.EntityID(cid), tile, uint64(g.clock.Now()))
	g.clock.Advance(1)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 100})
	res, why := f.Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: item.Silver, Qty: -1}}), true)
	if !res.OK() || why != "" {
		t.Fatalf("live visit trade refused: %s", why)
	}
	after, _ := f.Market().At(tile)
	if after.Epoch != stock.Epoch+1 {
		t.Fatalf("accepted trade at deadline without restock: epoch %d -> %d, deadline %d", stock.Epoch, after.Epoch, stock.RestockAt)
	}
}
func TestBulkSelectionUsesOrdinaryStack(t *testing.T) {
	stale := item.Stack{Kind: item.Potatoes, Qty: 3, Rot: 1}
	fresh := item.Stack{Kind: item.Potatoes, Qty: 3}
	if !trade.Detailed(stale) || trade.GiftIndividual(stale) {
		t.Fatal("fixture classification unexpected")
	}
	stock := trade.Stock{Kind: trade.BulkHauler, Silver: 1000}
	res, p, valid := trade.SettlePhysical(item.Book{}, []trade.Delta{{Kind: item.Potatoes, Qty: -1}}, map[item.Kind]int{item.Potatoes: 6}, stock, trade.Terms{}, false, []trade.GiftStack{{ID: 1, Stack: stale}, {ID: 2, Stack: fresh}})
	if !valid || !res.OK() {
		t.Fatalf("offer rejected: %v %v", valid, res.Reject)
	}
	if len(p) != 1 || p[0].ID != 2 {
		t.Fatalf("bulk ordinary row selected separate detailed stack: %+v", p)
	}
}

func TestMarketExactLotAndStaleQuote(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 2000})
	worn := item.Stack{Kind: item.Parka, Qty: 1, Quality: quality.Stored(quality.Awful), Stuff: item.Cloth, Wear: 20}
	fine := item.Stack{Kind: item.Parka, Qty: 1, Quality: quality.Stored(quality.Excellent), Stuff: item.Cloth}
	travelOf(g).Carry(cid, fine)
	travelOf(g).Carry(cid, worn)
	market := g.WorldMap().Markets[0]
	var rowID uint64
	want := 0
	for _, r := range market.Rows {
		if r.ItemKind == item.Parka && r.CargoQty == 1 && (want == 0 || r.SellC < want) {
			want = r.SellC
			rowID = r.StackID
		}
	}
	if rowID == 0 {
		t.Fatal("no exact physical sale row")
	}
	offer := trade.EncodeOffer([]trade.Delta{{Kind: item.Parka, StackID: rowID, Qty: -1}})
	stock0, _ := fleetOf(g).Market().At(tile)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Wood, Qty: 1})
	cargo0 := travelOf(g).Goods(cid)
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: offer, Quote: market.Quote})
	g.ApplyPending()
	reply := g.WorldMap().MarketReply
	stock1, _ := fleetOf(g).Market().At(tile)
	if reply.Reason == "" || !reflect.DeepEqual(stock0, stock1) || !reflect.DeepEqual(cargo0, travelOf(g).Goods(cid)) {
		t.Fatal("stale quote was accepted or mutated inventories")
	}
	market = g.WorldMap().Markets[0]
	res, why := fleetOf(g).Market().Trade(cid, pawn, offer, false, market.Quote)
	if why != "" || res.SilverCenti != want {
		t.Fatalf("exact price rejected: %v %s want %d", res, why, want)
	}
	m, _ := fleetOf(g).Market().At(tile)
	if len(m.Preserved) != 1 || m.Preserved[0] != worn {
		t.Fatalf("wrong sale lot: %+v", m.Preserved)
	}
	projected, ok := maps.PhysicalStock(m)
	if !ok {
		t.Fatal("projection invalid")
	}
	var lotID uint64
	for _, l := range projected.Lines {
		for _, lot := range l.Lots {
			if lot.Stack == worn {
				lotID = lot.ID
			}
		}
	}
	if lotID == 0 {
		t.Fatal("sale lot missing")
	}
	loaded := saveLoad(t, g)
	loaded.worldS().planet, loaded.worldS().planetTried = testPlanet().Fork(), true
	market = loaded.WorldMap().Markets[0]
	res, why = fleetOf(loaded).Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: item.Parka, StackID: lotID, Qty: 1}}), false, market.Quote)
	if why != "" || !res.OK() {
		t.Fatalf("exact lot buy after load failed: %v %s", res, why)
	}
	if !slices.Contains(travelOf(loaded).Goods(cid), worn) {
		t.Fatal("purchased lot was not preserved")
	}
}

func TestQuotedOrdinaryDoesNotSubstitutePhysicalLot(t *testing.T) {
	stale := item.Stack{Kind: item.Potatoes, Qty: 3, Rot: 1}
	stock := trade.Stock{Kind: trade.BulkHauler, Silver: 1000}
	d := []trade.Delta{{Kind: item.Potatoes, Qty: -1}}
	goods := []trade.GiftStack{{ID: 1, Stack: stale}}
	_, _, valid := trade.SettlePhysical(item.Book{}, d, map[item.Kind]int{item.Potatoes: 3}, stock, trade.Terms{}, false, goods, trade.ExactSelection)
	if valid {
		t.Fatal("stale ordinary quote silently substituted a physical lot")
	}
	res, plan, valid := trade.SettlePhysical(item.Book{}, d, map[item.Kind]int{item.Potatoes: 3}, stock, trade.Terms{}, false, goods)
	if !valid || !res.OK() || len(plan) != 1 || plan[0].ID != 1 {
		t.Fatal("legacy kind-only physical sale broken")
	}
}
func TestRadioGiftAtGoodwillCap(t *testing.T) {
	g, c := caravanGame(t)
	live := g.factionByID(c.Faction)
	if live == nil {
		t.Fatal("fixture faction is missing")
	}
	f := *live
	f.Goodwill = 100
	row := g.commsFaction(f, true, 10000, uint64(g.clock.Now()))
	if row.Gift.Allowed || row.Gift.Reason != ReasonNoGiftGain {
		t.Fatalf("gift at goodwill cap: %+v", row.Gift)
	}
}
func TestSessionRefusesSameBatchDismissal(t *testing.T) {
	g, c := caravanGame(t)
	neg := openCaravanTrade(t, g, c)
	tradeYard(t, g, neg, 900, 10)
	before := cloneStock(c.Stock)
	goods := g.colonyGoods(false).Counts()
	visitsOf(g).LeaveGroup(c.GroupID, visit.LeaveDismissed, uint64(g.clock.Now()))
	g.cmdProposeTrade(trade.EncodeOffer([]trade.Delta{{Kind: item.Components, Qty: -1}}), false)
	if !reflect.DeepEqual(before, c.Stock) || !reflect.DeepEqual(goods, g.colonyGoods(false).Counts()) || g.trade.session != nil {
		t.Fatal("dismissed session committed or remained open")
	}
}

func TestPackExitConservesLedger(t *testing.T) {
	for _, humanFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(humanFirst), func(t *testing.T) {
			g, c := caravanGame(t)
			pack := caravanPack(g, c)
			if pack == 0 {
				t.Fatal("missing pack animal")
			}
			groupID := c.GroupID
			now := uint64(g.clock.Now())
			visitsOf(g).LeaveGroup(groupID, visit.LeaveDismissed, now)
			if humanFirst {
				for _, id := range slices.Clone(c.Members) {
					(visitBodies{g: g}).Depart(id)
					g.traderVisitorGone(id)
				}
			}
			// Save at an actual departure intermediate state.
			g = saveLoad(t, g)
			c = caravanByGroup(&g.trade, groupID)
			if c == nil {
				t.Fatal("active carrier ledger missing on restore")
			}
			checkCaravanManifest(t, c)
			owned := slices.IndexFunc(c.Cargo, func(h visit.Cargo) bool { return h.Owner == pack })
			if owned < 0 {
				t.Fatal("pack manifest missing")
			}
			before := cloneStock(c.Stock)
			manifest := c.Cargo[owned]
			drops := len(g.itemIDs)
			gr, ok := visitsOf(g).GroupOf(pack)
			if !ok {
				t.Fatal("pack group missing")
			}
			g.positions.Set(pack, Position{X: gr.ExitX, Y: gr.ExitY})
			(visitGround{g: g}).PackStep(pack, gr, uint64(g.clock.Now()))
			if g.animals.Has(pack) || len(g.itemIDs) != drops {
				t.Fatal("normal exit did not remove pack or created a drop")
			}
			checkCaravanManifest(t, c)
			if c.Stock.Silver != before.Silver-manifest.Silver {
				t.Fatal("normal exit silver debit missing")
			}
			for _, line := range manifest.Lines {
				if stockQty(c.Stock, line.Kind) != stockQty(before, line.Kind)-line.Qty {
					t.Fatalf("normal exit did not debit %v", line)
				}
			}
			packCargoLost(g, pack)
			if len(g.itemIDs) != drops {
				t.Fatal("departure cargo dropped twice")
			}
		})
	}
}

func TestRestockBoundaryWithoutTrade(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	a, b := g.pawnIDs[0], g.pawnIDs[1]
	cid := visitingMarket(t, g, tile, a)
	other := visitingMarket(t, g, tile, b)
	before, _ := fleetOf(g).Market().At(tile)
	now := uint64(g.clock.Now())
	g.clock.Advance(int(before.RestockAt - 1 - now))
	maps.DevArrive(fleetOf(g), entity.EntityID(cid), tile, uint64(g.clock.Now()))
	maps.DevArrive(fleetOf(g), entity.EntityID(other), tile, uint64(g.clock.Now()))
	h := saveLoad(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	for _, x := range []*Game{g, h} {
		x.clock.Advance(1)
		x.ApplyPending()
		after, _ := fleetOf(x).Market().At(tile)
		if after.Epoch != before.Epoch+1 {
			t.Fatalf("idle restock epoch %d want %d", after.Epoch, before.Epoch+1)
		}
		if len(x.WorldMap().Markets) != 2 {
			t.Fatal("two live visitors not quoted after scheduled restock")
		}
	}
	m1, _ := fleetOf(g).Market().At(tile)
	m2, _ := fleetOf(h).Market().At(tile)
	if !reflect.DeepEqual(m1, m2) {
		t.Fatal("restock changed after checkpoint")
	}
}
