// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/platform/input"
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
	value := trade.GiftValue(item.Book{}, d.offer(item.Book{}), trade.Terms{Social: d.view.Social})
	if value != 2 || d.reason() != "" {
		t.Fatalf("gift selling value %d reason %q", value, d.reason())
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

// CTL-018: Ctrl multiplies a trade step by 10, Shift by 100, both by 1000;
// the amount never passes what the colony or the trader holds.
func TestTradeStepsFollowCtrlAndShift(t *testing.T) {
	t.Parallel()
	s, dl := newFake()
	v := testTradeView()
	v.Rows[0].TraderQty, v.Rows[0].ColonyQty = 2000, 30
	s.tv = &v
	s.frame(dl)
	s.frame(dl)
	wood := v.Rows[0].Kind
	press := func(id string, mods input.Mod) {
		s.f.Raw.Mods = mods
		if !s.kit.Click(id) {
			t.Fatalf("no widget %s", id)
		}
		s.frame(dl)
	}
	press("trade."+wood+".take.1", 0)
	if got := dl.trade.net[wood]; got != 1 {
		t.Fatalf("plain take 1 = %d", got)
	}
	press("trade."+wood+".take.1", input.ModCtrl)
	if got := dl.trade.net[wood]; got != 11 {
		t.Fatalf("ctrl take: %d, want 11", got)
	}
	press("trade."+wood+".take.1", input.ModShift)
	if got := dl.trade.net[wood]; got != 111 {
		t.Fatalf("shift take: %d, want 111", got)
	}
	press("trade."+wood+".take.1", input.ModCtrl|input.ModShift)
	if got := dl.trade.net[wood]; got != 1111 {
		t.Fatalf("both: %d, want 1111", got)
	}
	press("trade."+wood+".take.1", input.ModCtrl|input.ModShift)
	if got := dl.trade.net[wood]; got != 2000 {
		t.Fatalf("take clamps to the trader's stock: %d", got)
	}
	press("trade."+wood+".give.1", input.ModCtrl|input.ModShift)
	press("trade."+wood+".give.1", input.ModCtrl|input.ModShift)
	press("trade."+wood+".give.1", input.ModCtrl|input.ModShift)
	if got := dl.trade.net[wood]; got != -30 {
		t.Fatalf("give clamps to the colony's stock: %d", got)
	}
}

func TestTradeDialogWholeOfferRoundingMatchesSettlement(t *testing.T) {
	d := newTradeDraft(game.TradeView{ColonySilver: 3, TraderSilver: 0,
		Rows: []game.TradeRowView{{Kind: item.Steel.String(), TraderQty: 1,
			BuyC: 349, WillBuy: true}}})
	d.step(d.view.Rows[0], 1)
	if d.totalC() != 349 || d.colonyAfterC() != 0 || d.reason() != "" {
		t.Fatalf("3.49 centi-silver quote settles as 3: net %d, balance %d, reason %q",
			d.totalC(), d.colonyAfterC(), d.reason())
	}
	d.view.ColonySilver = 2
	if d.reason() == "" {
		t.Fatal("two silver cannot buy a rounded three-silver purchase")
	}
}

// TestTradeDialogGiftPhysicalSelection verifies named items and their
// individual values survive the gift UI and command boundary.
func TestTradeDialogGiftPhysicalSelection(t *testing.T) {
	t.Parallel()
	v := testTradeView()
	v.GiftRows = []game.TradeRowView{
		{Kind: item.Rifle.String() + "@111", Label: "Rifle (poor)", ColonyQty: 1, WillSell: true, StackID: 111, GiftC: 700},
		{Kind: item.Rifle.String() + "@222", Label: "Rifle (excellent)", ColonyQty: 1, WillSell: true, StackID: 222, GiftC: 3100},
	}
	d := newTradeDraft(v)
	d.setGift(true)
	d.step(v.GiftRows[1], -1)
	if got := d.giftSilver(item.Book{}); got != 31 {
		t.Fatalf("physical gift preview %d, want 31", got)
	}
	offer := d.offer(item.Book{})
	if len(offer) != 1 || offer[0] != (trade.Delta{Kind: item.Rifle, Qty: -1, StackID: 222}) {
		t.Fatalf("physical gift offer: %+v", offer)
	}
	d.setGift(false)
	if len(d.net) != 0 || len(d.rows()) != len(v.Rows) {
		t.Fatal("changing modes reused selected physical gift")
	}
}

// TestPaidDialogSelectsPhysicalBuyAndSellInOneOffer uses separate item IDs
// of the same kind in both directions without collapsing the codec rows.
func TestPaidDialogSelectsPhysicalBuyAndSellInOneOffer(t *testing.T) {
	t.Parallel()
	v := testTradeView()
	v.ColonySilver, v.TraderSilver = 1000, 5000
	v.Rows = []game.TradeRowView{
		{Kind: item.Longsword.String() + "@c345", Label: "Worn wood longsword",
			ColonyQty: 1, SellC: 1019, StackID: 345, WillSell: true},
		{Kind: item.Longsword.String() + "@t6", Label: "Excellent gold longsword",
			TraderQty: 1, BuyC: 18351, StackID: 6, WillBuy: true},
	}
	d := newTradeDraft(v)
	d.step(v.Rows[0], -1)
	d.step(v.Rows[1], 1)
	if d.totalC() != 18351-1019 || d.reason() != "" {
		t.Fatalf("physical buy/sell mismatch: %d, reason %q", d.totalC(), d.reason())
	}
	offer := d.offer(item.Book{})
	if len(offer) != 2 || offer[0].Qty != -1 || offer[0].StackID != 345 ||
		offer[1].Qty != 1 || offer[1].StackID != 6 {
		t.Fatalf("mixed physical offer collapsed by kind: %+v", offer)
	}
	decoded, err := trade.DecodeOffer(item.Book{}, trade.EncodeOffer(offer))
	if err != nil || !reflect.DeepEqual(decoded, offer) {
		t.Fatalf("command boundary lost exact IDs: %+v, %v", decoded, err)
	}
	d.setGift(true)
	if len(d.net) != 0 {
		t.Fatal("physical paid selections remained in gift mode")
	}
}

func TestTradeDialogRefreshesChangedPhysicalPrice(t *testing.T) {
	t.Parallel()
	v := testTradeView()
	d := newTradeDraft(v)
	d.step(v.Rows[0], -1)
	v.Rows[0].SellC += 100
	d.refresh(v)
	if len(d.net) != 0 {
		t.Fatal("stale quote remained after physical market repriced")
	}
}
