// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package uikit

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// colSplitCase is a table width split pinned to the values the old
// tableColRects gave (QA-002: the rename keeps every X and W).
type colSplitCase struct {
	name  string
	w     float32
	cols  []float32 // column widths, 0: flex
	wantX []float32
	wantW []float32
}

func colSplitCases() []colSplitCase {
	return []colSplitCase{
		{"fixed and two flex", 300, []float32{100, 0, 50, 0}, []float32{10, 110, 185, 235}, []float32{100, 75, 50, 75}},
		{"flex share clamped at 0", 100, []float32{100, 0, 50, 0}, []float32{10, 110, 110, 160}, []float32{100, 0, 50, 0}},
		{"all flex", 300, []float32{0, 0, 0}, []float32{10, 110, 210}, []float32{100, 100, 100}},
	}
}

func colsOfWidths(ws []float32) []TableCol {
	cols := make([]TableCol, len(ws))
	for i, w := range ws {
		cols[i] = TableCol{ID: string(rune('a' + i)), W: w}
	}
	return cols
}

func TestTableCellsKeepTableColRectsSplit(t *testing.T) {
	t.Parallel()
	for _, c := range colSplitCases() {
		cols := colsOfWidths(c.cols)
		got := TableCells(nil, gfx.Rect{X: 10, Y: 20, W: c.w, H: 24}, cols)
		if len(got) != len(c.cols) {
			t.Fatalf("%s: %d cells, want %d", c.name, len(got), len(c.cols))
		}
		for i, cell := range got {
			want := gfx.Rect{X: c.wantX[i], Y: 20, W: c.wantW[i], H: 24}
			if cell.R != want || cell.Col != cols[i].ID {
				t.Errorf("%s: cell %d = %+v, want %s %+v", c.name, i, cell, cols[i].ID, want)
			}
		}
	}
}

// TableCells appends to dst: a caller's stack buffer keeps its cells and
// allocates nothing. Not parallel: AllocsPerRun.
func TestTableCellsAppendToDst(t *testing.T) {
	var buf [4]TableCell
	head := append(buf[:0], TableCell{Col: "pre"})
	got := TableCells(head, gfx.Rect{X: 0, Y: 5, W: 90, H: 10}, colsOfWidths([]float32{0, 0, 0}))
	if len(got) != 4 || got[0].Col != "pre" || &got[0] != &buf[0] {
		t.Fatalf("TableCells did not append into dst: %+v", got)
	}
	if got[3] != (TableCell{Col: "c", R: gfx.Rect{X: 60, Y: 5, W: 30, H: 10}}) {
		t.Errorf("last cell = %+v", got[3])
	}
	cols := colsOfWidths3()
	if a := testing.AllocsPerRun(20, func() { _ = TableCells(buf[:0], gfx.Rect{W: 90}, cols) }); a != 0 {
		t.Errorf("TableCells into a stack buffer allocates %v times", a)
	}
}

func colsOfWidths3() []TableCol { return []TableCol{{ID: "a"}, {ID: "b", W: 20}, {ID: "c"}} }

// TableBodyRow is the first body row: below the header, left of the
// scroll bar, ListRowH tall by default.
func TestTableBodyRow(t *testing.T) {
	t.Parallel()
	r := gfx.Rect{X: 10, Y: 100, W: 400, H: 200}
	if got, want := TableBodyRow(r, 0), (gfx.Rect{X: 10, Y: 100 + TableHeaderH, W: 400 - ScrollBarW, H: ListRowH}); got != want {
		t.Errorf("TableBodyRow default = %+v, want %+v", got, want)
	}
	if got := TableBodyRow(r, 40); got.H != 40 {
		t.Errorf("TableBodyRow rowH 40 = %+v", got)
	}
	if got := TableBodyRow(gfx.Rect{X: 0, Y: 50, W: 100, H: 10}, 20); got.Y != 60 {
		t.Errorf("a table shorter than its header puts the body at %v, want 60 (SplitV clamps)", got.Y)
	}
}

func TestTableColIDs(t *testing.T) {
	t.Parallel()
	got := TableColIDs(colsOfWidths3())
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("TableColIDs = %v", got)
	}
	if TableColIDs(nil) != nil {
		t.Errorf("TableColIDs(nil) is not nil")
	}
}

func TestColumnScreens(t *testing.T) {
	t.Parallel()
	s := ColumnScreens()
	if s[0] != (ColumnScreen{W: MinStepW, H: MinStepH, Small: true}) || s[1] != (ColumnScreen{W: 3840, H: 2160}) {
		t.Errorf("ColumnScreens = %+v", s)
	}
}

func TestColumnGapsNilForAGoodRow(t *testing.T) {
	t.Parallel()
	win := gfx.Rect{X: 8, Y: 0, W: 1264, H: 700}
	cells := []TableCell{
		{Col: "name", R: gfx.Rect{X: 8, Y: 10, W: 200, H: 24}},
		{Col: "kills", R: gfx.Rect{X: 1200, W: 72}}, // H 0: only X is checked
	}
	if got := ColumnGaps(win, []string{"name", "kills"}, nil, cells); got != nil {
		t.Errorf("a good row has problems: %v", got)
	}
	// 0.01 px of float slack is not a problem
	cells[1].R.W = 72.005
	if got := ColumnGaps(win, []string{"name", "kills"}, nil, cells); got != nil {
		t.Errorf("float slack reported: %v", got)
	}
}

func TestColumnGapsReportsEachProblem(t *testing.T) {
	t.Parallel()
	win := gfx.Rect{X: 8, Y: 0, W: 1264, H: 700}
	declared := []string{"Damage taken", "Kills", "Goods", "Twice", "Tall", "Dropped"}
	cells := []TableCell{
		{Col: "Kills", R: gfx.Rect{X: 100, W: 0}},
		{Col: "Goods", R: gfx.Rect{X: 1270, W: 30}},
		{Col: "Twice", R: gfx.Rect{X: 10, W: 10}},
		{Col: "Twice", R: gfx.Rect{X: 30, W: 10}},
		{Col: "Tall", R: gfx.Rect{X: 10, Y: 690, W: 10, H: 24}},
		{Col: "Stray", R: gfx.Rect{X: 10, W: 10}},
	}
	got := ColumnGaps(win, declared, []string{"Dropped"}, cells)
	want := []string{
		"Damage taken: no cell",
		"Kills: width 0",
		"Goods: x 1270..1300 outside 8..1272",
		"Twice: 2 cells",
		"Tall: y 690..714 outside 0..700",
		"Stray: not a declared column",
	}
	if len(got) != len(want) {
		t.Fatalf("ColumnGaps = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("problem %d = %q, want %q", i, got[i], want[i])
		}
	}
	// without the droppable list, a dropped column is a gap
	if got := ColumnGaps(win, []string{"Dropped"}, nil, nil); len(got) != 1 || got[0] != "Dropped: no cell" {
		t.Errorf("an undeclared drop = %q", got)
	}
}

func TestTableColumnGapsFlagsUndrawnColumns(t *testing.T) {
	t.Parallel()
	win := gfx.Rect{X: 0, Y: 0, W: 400, H: 300}
	cell := func(int) string { return "x" }
	cols := []TableCol{
		{ID: "name", W: 300, Cell: cell},
		{ID: "blank", W: 50},            // neither Cell nor Draw
		{ID: "flex", Cell: cell},        // the fixed columns leave no share
		{ID: "squeezed", Cell: cell},    // ...for either flex column
		{ID: "wide", W: 60, Cell: cell}, // past the right edge
	}
	got := TableColumnGaps(win, win, cols, 0)
	want := []string{
		"blank: no Cell or Draw",
		"flex: width 0",
		"squeezed: width 0",
		"wide: x 350..410 outside 0..400",
	}
	if len(got) != len(want) {
		t.Fatalf("TableColumnGaps = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("problem %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestTableCellsDrawEveryColumn (QA-002): a Table in a dialog's content
// rect places every column at both column screens.
func TestTableCellsDrawEveryColumn(t *testing.T) {
	t.Parallel()
	cell := func(int) string { return "x" }
	cols := []TableCol{
		{ID: "name", Title: "Name", Cell: cell},
		{ID: "age", Title: "Age", W: 80, Align: gfx.AlignRight, Cell: cell},
		{ID: "bar", Title: "Bar", W: 120, Draw: func(_ *ebiten.Image, _ int, _ gfx.Rect) {}},
		{ID: "note", Title: "Note", Cell: cell},
	}
	for _, s := range ColumnScreens() {
		_, _, content, _ := DialogFrame(s.W, s.H)
		if p := TableColumnGaps(content, content, cols, 0); p != nil {
			t.Errorf("%dx%d: %q", s.W, s.H, p)
		}
		cells := TableCells(nil, TableBodyRow(content, 0), cols)
		if p := ColumnGaps(content, TableColIDs(cols), nil, cells); p != nil {
			t.Errorf("%dx%d: %q", s.W, s.H, p)
		}
	}
}
