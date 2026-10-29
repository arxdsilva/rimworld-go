package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
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
	f.Cur.Mx, f.Cur.My = -50, -50
	f.Frame(t, a)
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
	w, ok := a.uiSt().Find(id)
	if !ok {
		t.Fatalf("no widget %q", id)
	}
	cx, cy := w.R.Center()
	f.clickAt(t, a, int(cx), int(cy))
}

func TestTradeDialogEmitsProposeTrade(t *testing.T) {
	t.Parallel()
	s := &dealSim{tv: testTradeView(), tok: true}
	a, f := dealApp(t, s)
	f.Frame(t, a)
	if !a.uiSt().ModalOpen() || a.renderer.speed != speedPaused {
		t.Fatalf("open=%v speed=%v", a.uiSt().ModalOpen(), a.renderer.speed)
	}
	clickID(t, a, f, "trade."+item.Wood.String()+".take.1")
	clickID(t, a, f, "trade."+item.Wood.String()+".give.1")
	clickID(t, a, f, "trade."+item.Wood.String()+".give.1")
	clickID(t, a, f, "modal."+dialogs.TradeModalID+".accept")
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

func TestTradeDialogCancelEmitsClose(t *testing.T) {
	t.Parallel()
	s := &dealSim{tv: testTradeView(), tok: true}
	a, f := dealApp(t, s)
	f.Frame(t, a)
	clickID(t, a, f, "modal."+dialogs.TradeModalID+".cancel")
	f.Frame(t, a)
	n := 0
	for _, c := range s.cmds {
		if c.Kind == command.CloseTrade {
			n++
		}
	}
	if n != 1 || a.uiSt().ModalOpen() {
		t.Fatalf("CloseTrade x%d, modal open=%v (the live session must not reopen it)", n, a.uiSt().ModalOpen())
	}
	s.tok = false
	f.Frame(t, a)
	if a.renderer.speed == speedPaused && a.deal.Paused() {
		t.Fatal("pause not released")
	}
}
