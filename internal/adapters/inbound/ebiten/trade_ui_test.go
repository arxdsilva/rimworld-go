package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
)

// dealSim is a stub game with a trade and a comms session.
type dealSim struct {
	*stubSim
	tv  game.TradeView
	tok bool
	cv  game.CommsView
	cok bool
}

func (s *dealSim) TradeView() (game.TradeView, bool) { return s.tv, s.tok }
func (s *dealSim) CommsView() (game.CommsView, bool) { return s.cv, s.cok }

func dealApp(t *testing.T, s *dealSim) (*App, *fakeInput) {
	t.Helper()
	s.stubSim = &stubSim{w: 24, h: 24}
	a := testApp(s)
	a.finishBoot()
	a.startGame()
	f := fakeOn(a)
	f.cur.mx, f.cur.my = -50, -50
	f.frame(t, a)
	return a, f
}

func testTradeView() game.TradeView {
	return game.TradeView{
		Trader: "Mara", TraderKind: "bulk hauler", Faction: "Ridge", Goodwill: 5,
		NegotiatorName: "Ada", Social: 4, PriceModPct: 8, HoursLeft: 20,
		ColonySilver: 10, TraderSilver: 100,
		Rows: []game.TradeRowView{
			{Kind: item.Wood.String(), Label: "Wood", Category: "Materials", ColonyQty: 30, TraderQty: 5, BuyC: 250, SellC: 100, WillBuy: true, WillSell: true},
			{Kind: item.Steel.String(), Label: "Steel", Category: "Materials", ColonyQty: 0, TraderQty: 50, BuyC: 400, SellC: 200, WillBuy: true},
		},
	}
}

func clickID(t *testing.T, a *App, f *fakeInput, id string) {
	t.Helper()
	w, ok := a.uiSt().find(id)
	if !ok {
		t.Fatalf("no widget %q", id)
	}
	cx, cy := w.r.Center()
	f.clickAt(t, a, int(cx), int(cy))
}

func TestTradeDialogTotals(t *testing.T) {
	t.Parallel()
	d := newTradeDraft(testTradeView())
	wood, steel := d.view.Rows[0], d.view.Rows[1]
	d.step(steel, 2) // buy 2 steel: 8.00
	d.step(wood, -3) // sell 3 wood: 3.00
	if got := d.totalC(); got != 500 {
		t.Fatalf("total = %d, want 500", got)
	}
	if d.colonyAfterC() != 500 || d.traderAfterC() != 10500 {
		t.Fatalf("after = %d / %d", d.colonyAfterC(), d.traderAfterC())
	}
	d.step(steel, 10) // 12 steel = 48.00 -> colony short
	if d.colonyAfterC() >= 0 || d.reason() == "" {
		t.Fatalf("overspend must disable Accept: after %d reason %q", d.colonyAfterC(), d.reason())
	}
	if silverText(-250) != "-2.50" {
		t.Fatalf("silverText = %q", silverText(-250))
	}
}

func TestTradeDialogStepperClamps(t *testing.T) {
	t.Parallel()
	d := newTradeDraft(testTradeView())
	wood, steel := d.view.Rows[0], d.view.Rows[1]
	d.step(wood, 99)
	if d.net[wood.Kind] != 5 {
		t.Fatalf("buy clamps to trader stock: %d", d.net[wood.Kind])
	}
	d.step(wood, -99)
	if d.net[wood.Kind] != -30 {
		t.Fatalf("sell clamps to colony stock: %d", d.net[wood.Kind])
	}
	d.step(steel, -5) // the trader does not buy steel from an empty colony
	if _, ok := d.net[steel.Kind]; ok {
		t.Fatalf("no selling what the colony lacks: %v", d.net)
	}
}

func TestTradeDialogEmitsProposeTrade(t *testing.T) {
	t.Parallel()
	s := &dealSim{tv: testTradeView(), tok: true}
	a, f := dealApp(t, s)
	f.frame(t, a)
	if !a.uiModalOpen() || a.renderer.speed != speedPaused {
		t.Fatalf("open=%v speed=%v", a.uiModalOpen(), a.renderer.speed)
	}
	clickID(t, a, f, "trade."+item.Wood.String()+".take.1")
	clickID(t, a, f, "trade."+item.Wood.String()+".give.1")
	clickID(t, a, f, "trade."+item.Wood.String()+".give.1")
	clickID(t, a, f, "modal."+tradeModalID+".accept")
	want := game.EncodeTradeOffer([]trade.Delta{{Kind: item.Wood, Qty: -1}})
	var got *command.Command
	for i := range s.cmds {
		if s.cmds[i].Kind == command.ProposeTrade {
			got = &s.cmds[i]
		}
	}
	if got == nil || got.Name != want || got.Enabled {
		t.Fatalf("cmds = %+v, want ProposeTrade %q", s.cmds, want)
	}
}

func TestTradeDialogGiftMode(t *testing.T) {
	t.Parallel()
	d := newTradeDraft(testTradeView())
	wood := d.view.Rows[0]
	d.step(wood, 3)
	d.setGift(true)
	if d.net[wood.Kind] != 0 {
		t.Fatalf("gift drops buying: %v", d.net)
	}
	d.step(wood, 4)
	if d.net[wood.Kind] != 0 {
		t.Fatalf("gift cannot buy: %v", d.net)
	}
	d.step(wood, -4)
	if d.giftSilver() != 4 || d.reason() != "" {
		t.Fatalf("gift value %d reason %q", d.giftSilver(), d.reason())
	}
	d.view.ColonySilver = 0 // a gift needs no silver
	if d.reason() != "" {
		t.Fatalf("gift reason %q", d.reason())
	}
}

func TestTradeDialogCancelEmitsClose(t *testing.T) {
	t.Parallel()
	s := &dealSim{tv: testTradeView(), tok: true}
	a, f := dealApp(t, s)
	f.frame(t, a)
	clickID(t, a, f, "modal."+tradeModalID+".cancel")
	f.frame(t, a)
	n := 0
	for _, c := range s.cmds {
		if c.Kind == command.CloseTrade {
			n++
		}
	}
	if n != 1 || a.uiModalOpen() {
		t.Fatalf("CloseTrade x%d, modal open=%v (the live session must not reopen it)", n, a.uiModalOpen())
	}
	s.tok = false
	f.frame(t, a)
	if a.renderer.speed == speedPaused && a.dealings.paused {
		t.Fatal("pause not released")
	}
}

func TestTradeLinesGroupByCategory(t *testing.T) {
	t.Parallel()
	ls := tradeLines(testTradeView().Rows)
	if len(ls) != 3 || ls[0].head != "Materials" || !strings.HasPrefix(ls[1].row.Label, "S") {
		t.Fatalf("lines = %+v", ls)
	}
}
