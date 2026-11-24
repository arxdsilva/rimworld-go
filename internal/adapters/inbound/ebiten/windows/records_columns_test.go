// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

type recordsTexts struct{}

func (recordsTexts) T(s string) string { return s }

// TestRecordCellsKeepTheirAnchors pins today's placement as literals: x=100,
// w=700 puts Colonist at x+6 (w-400 wide) and the others at x+w-380/-260/-130.
func TestRecordCellsKeepTheirAnchors(t *testing.T) {
	t.Parallel()
	var buf [4]uikit.TableCell
	cells := recordCells(buf[:0], gfx.Rect{X: 100, Y: 50, W: 700, H: 22})
	wantX := [4]float32{106, 420, 540, 670}
	for i, c := range cells {
		if c.R.X != wantX[i] {
			t.Errorf("cell %d (%s) X = %v, want %v", i, c.Col, c.R.X, wantX[i])
		}
		if c.R.Y != 50 || c.R.H != 22 {
			t.Errorf("cell %d Y/H = %v/%v, want 50/22", i, c.R.Y, c.R.H)
		}
	}
	if cells[0].R.W != 300 {
		t.Errorf("Colonist width = %v, want 300 (w-400)", cells[0].R.W)
	}
	if cells[3].R.Right() != 800 {
		t.Errorf("Damage taken ends at %v, want 800 (list right)", cells[3].R.Right())
	}
}

// TestRecordColumnsDrawn: the four declared columns each get one cell inside
// the History window at both column screens.
func TestRecordColumnsDrawn(t *testing.T) {
	t.Parallel()
	cols := recordColumns(recordsTexts{})
	declared := uikit.TableColIDs(cols[:])
	for _, s := range uikit.ColumnScreens() {
		win, body := windowsKitBody(t, func(w *Set) { w.History().Launch() }, s.W, s.H)
		var buf [4]uikit.TableCell
		cells := recordCells(buf[:0], recordsView(body))
		if gaps := uikit.ColumnGaps(win, declared, nil, cells); len(gaps) > 0 {
			t.Errorf("%dx%d: %v (cells %+v, window %+v)", s.W, s.H, gaps, cells, win)
		}
		for _, c := range cols {
			if c.Title == "" {
				t.Errorf("column %q has no header title", c.ID)
			}
		}
	}
}

// TestHistoryListRectKeepsTheCut: the list sits below the 31 px tab strip and
// a 6 px gap, as every History tab did before.
func TestHistoryListRectKeepsTheCut(t *testing.T) {
	t.Parallel()
	got := historyListRect(gfx.Rect{X: 10, Y: 20, W: 500, H: 400})
	if want := (gfx.Rect{X: 10, Y: 57, W: 500, H: 363}); got != want {
		t.Fatalf("historyListRect = %+v, want %+v", got, want)
	}
}

// recordsView is the rect the Records draw lays its cells in: the list less
// the scroll bar's width (ScrollArea's View).
func recordsView(body gfx.Rect) gfx.Rect {
	l := historyListRect(body)
	l.W -= uikit.ScrollBarW
	return l
}
