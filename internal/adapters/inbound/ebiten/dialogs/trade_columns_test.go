// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
)

// TestTradeCellsKeepTheirAnchors pins the eight cells of a fixed body to the
// numbers the dialog drew before the layout became a function (no visual
// change). Body {100, 50, 800, 500}: Goods runs from 658 to 900 less the bar.
func TestTradeCellsKeepTheirAnchors(t *testing.T) {
	t.Parallel()
	var buf [8]uikit.TableCell
	got := tradeCells(buf[:0], gfx.Rect{X: 100, Y: 50, W: 800, H: 500})
	want := []uikit.TableCell{
		{Col: "sell", R: gfx.Rect{X: 100, W: 62}},
		{Col: "colony", R: gfx.Rect{X: 162, W: 46}},
		{Col: "give", R: gfx.Rect{X: 212, W: 126}},
		{Col: "amount", R: gfx.Rect{X: 342, W: 64}},
		{Col: "take", R: gfx.Rect{X: 410, W: 126}},
		{Col: "trader", R: gfx.Rect{X: 540, W: 46}},
		{Col: "buy", R: gfx.Rect{X: 586, W: 62}},
		{Col: "goods", R: gfx.Rect{X: 658, W: 230}},
	}
	if len(got) != len(want) {
		t.Fatalf("%d cells, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cell %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestTradeColumnsDrawn: every declared trade column has one cell inside the
// production modal body at both screens.
func TestTradeColumnsDrawn(t *testing.T) {
	t.Parallel()
	for _, sc := range uikit.ColumnScreens() {
		s, dl := newFake()
		v := testTradeView()
		s.tv = &v
		s.frame(dl)
		if len(s.kit.Modals) != 1 {
			t.Fatalf("%dx%d: %d modals open, want the trade dialog", sc.W, sc.H, len(s.kit.Modals))
		}
		_, _, body, _ := uikit.ModalLayout(sc.W, sc.H, s.kit.Modals[0])
		cols := tradeColumns(s, false)
		var buf [8]uikit.TableCell
		cells := tradeCells(buf[:0], body)
		win := gfx.Rect{X: body.X, Y: body.Y, W: body.W, H: body.H}
		if gaps := uikit.ColumnGaps(win, uikit.TableColIDs(cols[:]), nil, cells); gaps != nil {
			t.Errorf("%dx%d: %v", sc.W, sc.H, gaps)
		}
		gift := tradeColumns(s, true)
		if gaps := uikit.ColumnGaps(win, uikit.TableColIDs(gift[:]), nil, cells); gaps != nil {
			t.Errorf("%dx%d gift: %v", sc.W, sc.H, gaps)
		}
	}
}

// TestTradeColumnsTitlesInOrder: the header reads the declared list.
func TestTradeColumnsTitlesInOrder(t *testing.T) {
	t.Parallel()
	s, _ := newFake()
	cols := tradeColumns(s, false)
	want := []string{"Sell for", "Colony", "Give to trader", "Amount", "Take from trader", "Trader", "Buy for", "Goods"}
	for i, w := range want {
		if cols[i].Title != w {
			t.Errorf("col %d title %q, want %q", i, cols[i].Title, w)
		}
	}
}

func TestGiftColumnsTitlesInOrder(t *testing.T) {
	t.Parallel()
	s, _ := newFake()
	paid, gift := tradeColumns(s, false), tradeColumns(s, true)
	want := []string{"Gift value", "Colony", "Give to trader", "Amount", "", "", "", "Goods"}
	for i, title := range want {
		if gift[i].ID != paid[i].ID || gift[i].Title != title {
			t.Errorf("gift column %d = %+v, want id %q title %q", i, gift[i], paid[i].ID, title)
		}
	}
}

// Exercise real labels, controls and the physical row selection through the
// shared geometry in both paid and gift modes, at both supported screens.
func TestTradeDrawsPhysicalRowsOnCells(t *testing.T) { //nolint:paralleltest // canvas-global draw trace
	for _, sc := range uikit.ColumnScreens() {
		for _, gift := range []bool{false, true} {
			s, dl := newFake()
			v := testTradeView()
			v.Rows = []game.TradeRowView{{Kind: item.Rifle.String() + "@c111", Label: "Paid rifle", Category: "Weapons",
				ColonyQty: 2, TraderQty: 3, SellC: 700, BuyC: 1100, StackID: 111, WillSell: true, WillBuy: true}}
			v.GiftRows = []game.TradeRowView{{Kind: item.Rifle.String() + "@222", Label: "Gift rifle", Category: "Weapons",
				ColonyQty: 1, SellC: 700, GiftC: 3100, StackID: 222, WillSell: true}}
			dl.openTrade(v)
			d := dl.trade
			d.setGift(gift)
			_, _, body, _ := uikit.ModalLayout(sc.W, sc.H, s.kit.Modals[0])
			s.kit.Begin()
			s.kit.Layer = len(s.kit.Modals) // the modal body is the active input layer
			dl.tradeBody(d, body)
			row := d.rows()[0]
			cells := tradeCells(nil, body)
			cols := tradeColumns(s, gift)
			// Each give/take button stays within the declared slot and
			// uses the exact physical row's identity, not the item kind.
			for i, step := range tradeSteps {
				for _, side := range []string{"give", "take"} {
					col, index := tcGive, i
					if side == "take" {
						col, index = tcTake, len(tradeSteps)-1-i
					}
					b, ok := s.kit.Find("trade." + row.Kind + "." + side + "." + step.id)
					if !ok || b.R.X != cells[col].R.X+float32(index)*42 ||
						b.R.Right() > cells[col].R.Right() || b.Disabled != (gift && side == "take") {
						t.Fatalf("%dx%d gift=%t %s control: %+v, cell %+v", sc.W, sc.H, gift, side, b, cells[col])
					}
				}
			}
			var got []gfx.TraceEvent
			func() {
				gfx.SetDrawTrace(func(e gfx.TraceEvent) {
					if e.Kind == "text" {
						got = append(got, e)
					}
				})
				defer gfx.SetDrawTrace(nil)
				s.kit.Paint(ebiten.NewImage(sc.W, sc.H))
			}()
			scale := float32(gfx.UIScale())
			has := func(label string, x, y float32) bool {
				for _, e := range got {
					if e.Text == label && e.X == x*scale && e.Y == y*scale {
						return true
					}
				}
				return false
			}
			for i, c := range cols {
				if c.Title != "" && !has(c.Title, cells[i].R.X+tradeHeadDX[i], body.Y+54) {
					t.Errorf("%dx%d gift=%t missing header %s", sc.W, sc.H, gift, c.Title)
				}
			}
			y := body.Y + 50 + tradeHeadH + tradeRowH + 6
			price := row.SellC
			if gift {
				price = row.GiftC
			}
			for _, check := range []struct {
				text string
				x    float32
			}{
				{row.Label, cells[tcGoods].R.X},
				{SilverText(price), cells[tcSell].R.X},
				{uikit.Num(row.ColonyQty), cells[tcColony].R.X + 4},
			} {
				if !has(check.text, check.x, y) {
					t.Errorf("%dx%d gift=%t physical row text %q not drawn at %v,%v", sc.W, sc.H, gift, check.text, check.x, y)
				}
			}
			if has(SilverText(row.BuyC), cells[tcBuy].R.X+4, y) == gift ||
				has(uikit.Num(row.TraderQty), cells[tcTrader].R.X+2, y) == gift {
				t.Errorf("%dx%d gift=%t trader cells visibility wrong", sc.W, sc.H, gift)
			}
		}
	}
}
