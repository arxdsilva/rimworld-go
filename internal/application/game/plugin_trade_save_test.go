// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Trade migration regressions, with funded inventory and atomicity controls.
package game

import (
	"bytes"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
)

func TestMarketStockFollowsPluginKeys(t *testing.T) {
	g := worldGame(t)
	oldBook := pluginItems("a.cup", "b.bell")
	newBook := pluginItems("b.bell", "a.cup")
	g.sk.content.Items = oldBook
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	// This regression sells four items at the sell rate but attempts to
	// buy two at the buy rate, which can exhaust the proceeds. Fund both
	// purchases independently so a genuine silver refusal cannot mask
	// whether the restored market is usable.
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 1000})
	cup, bell := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	for _, k := range []item.Kind{cup, bell} {
		travelOf(g).Carry(cid, item.Stack{Kind: k, Qty: 2})
		res, why := fleetOf(g).Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: k, Qty: -2}}), false)
		if !res.OK() || why != "" {
			t.Fatalf("sale of %v refused: %v %q", oldBook.Name(k), res, why)
		}
	}
	if len(g.WorldMap().Markets) != 1 {
		t.Fatal("fixture: market absent before save")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "plugin-market"); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadContent(&buf, Content{Items: newBook})
	if err != nil {
		t.Fatal(err)
	}
	loaded.worldS().planet, loaded.worldS().planetTried = testPlanet().Fork(), true
	m, ok := fleetOf(loaded).Market().At(tile)
	if !ok {
		t.Fatal("market lost")
	}
	t.Logf("remapped stock tail: %+v, preserved: %+v", m.Stock.Lines[max(0, len(m.Stock.Lines)-4):], m.Preserved)
	if len(loaded.WorldMap().Markets) != 1 {
		t.Fatal("market disappeared when two existing plugin item IDs were reordered")
	}
	if !slices.IsSortedFunc(m.Stock.Lines, func(a, b trade.Line) int { return a.Goods().Compare(b.Goods()) }) || stockQty(m.Stock, cup) != 2 || stockQty(m.Stock, bell) != 2 {
		t.Fatalf("wrong restored stock: %+v", m.Stock)
	}
	for _, k := range []item.Kind{cup, bell} {
		res, why := fleetOf(loaded).Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: k, Qty: 1}}), false)
		if !res.OK() || why != "" {
			t.Fatalf("buying %v refused: %v %q", newBook.Name(k), res, why)
		}
		if got := travelOf(loaded).TradeCounts(cid)[k]; got != 1 {
			t.Fatalf("wrong received %s count %d", newBook.Name(k), got)
		}
	}
}

func TestPendingPluginGiftFollowsKey(t *testing.T) {
	g := worldGame(t)
	oldBook := pluginItems("a.cup", "b.bell")
	newBook := pluginItems("b.bell", "a.cup")
	g.sk.content.Items = oldBook
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	cup, bell := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	travelOf(g).Carry(cid, item.Stack{Kind: cup, Qty: 2})
	travelOf(g).Carry(cid, item.Stack{Kind: bell, Qty: 3})
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer([]trade.Delta{{Kind: cup, Qty: -1}}), Enabled: true})
	var buf bytes.Buffer
	if err := g.Save(&buf, "pending-plugin-gift"); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadContent(&buf, Content{Items: newBook})
	if err != nil {
		t.Fatal(err)
	}
	loaded.worldS().planet, loaded.worldS().planetTried = testPlanet().Fork(), true
	t.Logf("pending after remap %+v", loaded.cmds.Items())
	if got := travelOf(loaded).TradeCounts(cid); got[cup] != 3 || got[bell] != 2 {
		t.Fatalf("fixture remap failed: %v", got)
	}
	loaded.ApplyPending()
	got := travelOf(loaded).TradeCounts(cid)
	reply := loaded.WorldMap().MarketReply
	t.Logf("after gift: a.cup=%d b.bell=%d reply=%+v", got[bell], got[cup], reply)
	if got[bell] != 1 || got[cup] != 3 {
		t.Fatal("queued gift no longer names the saved item's stable key")
	}
}

func TestPendingQuotedPurchaseRejectsRepricedBook(t *testing.T) {
	g := worldGame(t)
	oldBook := pluginItems("a.cup")
	k := item.Kind(item.CoreKinds)
	defs := make([]item.Def, len(oldBook.Keys()))
	for i := range defs {
		defs[i] = oldBook.Def(item.Kind(i))
	}
	defs[int(k)].Value *= 10
	newBook := item.NewBook(defs, oldBook.Keys())
	g.sk.content.Items = oldBook
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: k, Qty: 2})
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 1000})
	res, why := fleetOf(g).Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: k, Qty: -2}}), false)
	if !res.OK() || why != "" {
		t.Fatalf("fixture sale refused %v %q", res, why)
	}
	quote := g.WorldMap().Markets[0].Quote
	price := func(g *Game) int {
		for _, row := range g.WorldMap().Markets[0].Rows {
			if row.ItemKind == k && row.StockQty > 0 {
				return row.BuyC
			}
		}
		return 0
	}
	oldPrice := price(g)
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer([]trade.Delta{{Kind: k, Qty: 1}}), Quote: quote})
	var buf bytes.Buffer
	if err := g.Save(&buf, "pending-quoted-purchase"); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadContent(&buf, Content{Items: newBook})
	if err != nil {
		t.Fatal(err)
	}
	loaded.worldS().planet, loaded.worldS().planetTried = testPlanet().Fork(), true
	before := loaded.WorldMap().Markets[0]
	newPrice := price(loaded)
	if oldPrice == 0 || newPrice <= oldPrice {
		t.Fatalf("invalid price fixture %d -> %d", oldPrice, newPrice)
	}
	cargoBefore := travelOf(loaded).TradeCounts(cid)
	stockBefore, _ := fleetOf(loaded).Market().At(tile)
	loaded.ApplyPending()
	after := travelOf(loaded).TradeCounts(cid)
	reply := loaded.WorldMap().MarketReply
	t.Logf("price centi %d -> %d; quote %d -> %d; silver %d -> %d; item %d -> %d; reply %+v", oldPrice, newPrice, quote, before.Quote, cargoBefore[item.Silver], after[item.Silver], cargoBefore[k], after[k], reply)
	stockAfter, _ := fleetOf(loaded).Market().At(tile)
	if before.Quote == quote || reply.Reason != "The trade quote changed. Review the current offer." || !reflect.DeepEqual(after, cargoBefore) || !reflect.DeepEqual(stockBefore, stockAfter) {
		t.Fatal("accepted a quoted purchase after changing its price")
	}
}

// New/removed keys must neither change a valid remaining stack's payload
// nor turn a mixed legacy gift into a partial transfer.
func TestPluginMarketRemovedAndAddedKeys(t *testing.T) {
	g := worldGame(t)
	oldBook := pluginItems("a.cup", "b.bell")
	g.sk.content.Items = oldBook
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	cup, bell := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 1000})
	for _, k := range []item.Kind{cup, bell} {
		travelOf(g).Carry(cid, item.Stack{Kind: k, Qty: 4, Wear: 3})
		res, why := fleetOf(g).Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: k, Qty: -2}}), false)
		if !res.OK() || why != "" {
			t.Fatalf("fixture sell: %v %q", res, why)
		}
	}
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Enabled: true, Name: trade.EncodeOffer([]trade.Delta{{Kind: cup, Qty: -1}, {Kind: bell, Qty: -1}})})
	var buf bytes.Buffer
	if err := g.Save(&buf, "removed-key"); err != nil {
		t.Fatal(err)
	}
	for _, keys := range [][]string{{"a.cup", "b.bell", "c.pot"}, {"b.bell", "c.pot"}} {
		book := pluginItems(keys...)
		h, _, err := LoadContent(bytes.NewReader(buf.Bytes()), Content{Items: book})
		if err != nil {
			t.Fatal(err)
		}
		h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
		bellNow, _ := book.KindOf("b.bell")
		m, ok := fleetOf(h).Market().At(tile)
		if !ok || len(h.WorldMap().Markets) != 1 || stockQty(m.Stock, bellNow) != 2 {
			t.Fatal("remaining inventory not tradeable")
		}
		for _, st := range m.Preserved {
			if int(st.Kind) >= book.Len() || book.Key(st.Kind) == "c.pot" || st.Wear != 3 {
				t.Fatalf("wrong physical stock: %+v", st)
			}
		}
		before := travelOf(h).TradeCounts(cid)
		f := h.factionByID(m.FactionID)
		goodwill := f.Goodwill
		h.ApplyPending()
		after := travelOf(h).TradeCounts(cid)
		if _, hasCup := book.KindOf("a.cup"); !hasCup {
			if !reflect.DeepEqual(before, after) || f.Goodwill != goodwill {
				t.Fatal("removed-key mixed gift was partially applied")
			}
		} else if after[bellNow] != before[bellNow]-1 {
			t.Fatal("append-only book broke valid pending gift")
		}
		m, ok = fleetOf(h).Market().At(tile)
		if !ok {
			t.Fatal("stock lost")
		}
		rows := h.WorldMap().Markets[0].Rows
		i := slices.IndexFunc(rows, func(r tabview.MarketRow) bool { return r.ItemKind == bellNow && r.StockQty > 0 && r.StackID != 0 })
		if i < 0 {
			t.Fatalf("physical bell stock not quoted: %+v", rows)
		}
		row := rows[i]
		res, why := fleetOf(h).Market().Trade(cid, pawn, trade.EncodeOffer([]trade.Delta{{Kind: bellNow, Qty: 1, StackID: row.StackID}}), false, h.WorldMap().Markets[0].Quote)
		if !res.OK() || why != "" {
			t.Fatalf("remaining physical lot not buyable: %v %q", res, why)
		}
	}
}

func TestLocalStockAndCarrierRemap(t *testing.T) {
	oldBook := pluginItems("a.cup", "b.bell")
	g, c := ptGame(t, oldBook)
	c.Kind, c.Stock.Kind = trade.CurioMerchant, trade.CurioMerchant
	cup, bell := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	c.Stock.Lines = trade.AddLine(c.Stock.Lines, trade.Line{Kind: cup, Qty: 2, Lots: []trade.Lot{{ID: 1, Stack: item.Stack{Kind: cup, Qty: 1, Wear: 2}}}})
	c.Stock.Lines = trade.AddLine(c.Stock.Lines, trade.Line{Kind: bell, Qty: 3, Lots: []trade.Lot{{ID: 2, Stack: item.Stack{Kind: bell, Qty: 1, Wear: 4}}}})
	c.Stock.NextLotID = 2
	c.Cargo = visit.AssignCargo(c.Stock, c.Members)
	var buf bytes.Buffer
	if err := g.Save(&buf, "local-owner-remap"); err != nil {
		t.Fatal(err)
	}
	for _, keys := range [][]string{{"b.bell", "a.cup"}, {"b.bell", "c.pot"}} {
		book := pluginItems(keys...)
		h, _, err := LoadContent(bytes.NewReader(buf.Bytes()), Content{Items: book})
		if err != nil {
			t.Fatal(err)
		}
		c := h.trade.caravan
		checkCaravanManifest(t, c)
		for _, lines := range append([][]trade.Line{c.Stock.Lines}, func() [][]trade.Line {
			var out [][]trade.Line
			for _, owner := range c.Cargo {
				out = append(out, owner.Lines)
			}
			return out
		}()...) {
			if !slices.IsSortedFunc(lines, func(a, b trade.Line) int { return a.Goods().Compare(b.Goods()) }) {
				t.Fatal("unsorted carrier/stock")
			}
			for _, l := range lines {
				if int(l.Kind) >= book.Len() || book.Key(l.Kind) == "c.pot" {
					t.Fatal("removed kind survived or became new kind")
				}
			}
		}
		bellNow, _ := book.KindOf("b.bell")
		res := trade.Result{Buy: []trade.Line{{Kind: bellNow, Qty: 1, Lots: []trade.Lot{{ID: 2, Stack: item.Stack{Kind: bellNow, Qty: 1, Wear: 4}}}}}}
		if !visit.SettleCargo(c.Cargo, c.Trader, res) || !c.Stock.ApplyTrade(res) {
			t.Fatal("restored local physical lot not transferable")
		}
		checkCaravanManifest(t, c)
	}
}
