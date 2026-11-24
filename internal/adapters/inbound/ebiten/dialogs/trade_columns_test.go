// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
		cols := tradeColumns(s)
		var buf [8]uikit.TableCell
		cells := tradeCells(buf[:0], body)
		win := gfx.Rect{X: body.X, Y: body.Y, W: body.W, H: body.H}
		if gaps := uikit.ColumnGaps(win, uikit.TableColIDs(cols[:]), nil, cells); gaps != nil {
			t.Errorf("%dx%d: %v", sc.W, sc.H, gaps)
		}
	}
}

// TestTradeColumnsTitlesInOrder: the header reads the declared list.
func TestTradeColumnsTitlesInOrder(t *testing.T) {
	t.Parallel()
	s, _ := newFake()
	cols := tradeColumns(s)
	want := []string{"Sell for", "Colony", "Give to trader", "Amount", "Take from trader", "Trader", "Buy for", "Goods"}
	for i, w := range want {
		if cols[i].Title != w {
			t.Errorf("col %d title %q, want %q", i, cols[i].Title, w)
		}
	}
}
