package dialogs

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
)

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
	if SilverText(-250) != "-2.50" {
		t.Fatalf("SilverText = %q", SilverText(-250))
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

func TestTradeLinesGroupByCategory(t *testing.T) {
	t.Parallel()
	ls := TradeLines(testTradeView().Rows)
	if len(ls) != 3 || ls[0].Head != "Materials" || !strings.HasPrefix(ls[1].Row.Label, "S") {
		t.Fatalf("lines = %+v", ls)
	}
}

// TestSyncDealingsPausesAndResumes: an open session pauses the display, a
// session that ends gives the interrupted speed back, and a dismissed
// dialog does not reopen while the session lives (no flapping).
func TestSyncDealingsPausesAndResumes(t *testing.T) {
	t.Parallel()
	s, dl := newFake()
	v := testTradeView()
	s.tv = &v
	dl.SyncDealings()
	if !dl.Open() || s.tier != 0 || !dl.Paused() {
		t.Fatalf("open=%v tier=%d paused=%v", dl.Open(), s.tier, dl.Paused())
	}
	dl.dismissTrade()
	dl.SyncDealings()
	if dl.Open() || s.tier != 2 || dl.Paused() {
		t.Fatalf("dismissed: open=%v tier=%d paused=%v", dl.Open(), s.tier, dl.Paused())
	}
	s.tv = nil
	dl.SyncDealings()
	s.tv = &v
	dl.SyncDealings()
	if !dl.Open() {
		t.Fatal("a new session reopens the dialog")
	}
}

func opt[T any](p *T) (v T, ok bool) {
	if p == nil {
		return v, false
	}
	return *p, true
}

// frame declares one frame: dialogs sync with the sessions, modals run.
func (s *fakeShell) frame(dl *Dealings) {
	s.kit.Begin()
	dl.SyncDealings()
	s.kit.RunModals(s.kit.CloseModal)
}
