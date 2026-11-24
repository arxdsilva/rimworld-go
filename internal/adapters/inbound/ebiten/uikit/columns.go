// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package uikit

import (
	"slices"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// Table columns that are drawn (QA-002): a table row's layout is one
// TableCell per declared column, in declared order, and a column test
// checks every declared column has a visible cell at both ColumnScreens.

// columnSlack is the float slack (logical px) a cell may pass its window by.
const columnSlack = 0.01

// TableCell is one placed cell of a table row: its column's declared id
// and the band its content occupies, in logical px (QA-002).
type TableCell struct {
	Col string
	R   gfx.Rect
}

// ColumnScreen is a logical screen a column test lays a window out on;
// Small is the smallest screen a UI step above 1 allows, where a window may
// drop the columns it declares droppable (QA-002).
type ColumnScreen struct {
	W, H  int
	Small bool
}

// ColumnScreens are the two logical screens every column test checks:
// the smallest step screen (MinStepW x MinStepH) and 3840 x 2160 (QA-002).
func ColumnScreens() [2]ColumnScreen {
	return [2]ColumnScreen{{W: MinStepW, H: MinStepH, Small: true}, {W: 3840, H: 2160}}
}

// ColumnGaps lists, in declared order, why a row's declared columns are
// not all drawn inside win (QA-002): a declared id with no cell (unless
// droppable), an id with more than one cell, a cell of width <= 0, a cell
// not fully inside win (0.01 px slack; a cell of height 0 is checked on X
// only), then each cell whose column is not declared. It is nil when every
// column is drawn.
func ColumnGaps(win gfx.Rect, declared, droppable []string, cells []TableCell) []string {
	var out []string
	for i, id := range declared {
		if slices.Index(declared[:i], id) >= 0 {
			continue // a repeated id is checked once
		}
		n := 0
		for _, c := range cells {
			if c.Col != id {
				continue
			}
			n++
			if n == 1 {
				out = cellGap(out, win, c)
			}
		}
		switch {
		case n == 0 && slices.Index(droppable, id) < 0:
			out = append(out, id+": no cell")
		case n > 1:
			out = append(out, id+": "+strconv.Itoa(n)+" cells")
		}
	}
	for _, c := range cells {
		if slices.Index(declared, c.Col) < 0 {
			out = append(out, c.Col+": not a declared column")
		}
	}
	return out
}

// cellGap appends c's width or placement problem in win, if any.
func cellGap(out []string, win gfx.Rect, c TableCell) []string {
	r := c.R
	switch {
	case r.W <= 0:
		return append(out, c.Col+": width "+pxNum(r.W))
	case r.X < win.X-columnSlack || r.Right() > win.Right()+columnSlack:
		return append(out, c.Col+": x "+pxSpan(r.X, r.Right())+" outside "+pxSpan(win.X, win.Right()))
	case r.H != 0 && (r.Y < win.Y-columnSlack || r.Bottom() > win.Bottom()+columnSlack):
		return append(out, c.Col+": y "+pxSpan(r.Y, r.Bottom())+" outside "+pxSpan(win.Y, win.Bottom()))
	}
	return out
}

// TableColumnGaps is ColumnGaps for a Kit.Table in r (rowH as Table's):
// its first body row's cells against its columns' ids, none droppable,
// after each column that has neither Cell nor Draw (QA-002).
func TableColumnGaps(win, r gfx.Rect, cols []TableCol, rowH float32) []string {
	var out []string
	for _, c := range cols {
		if c.Cell == nil && c.Draw == nil {
			out = append(out, c.ID+": no Cell or Draw")
		}
	}
	cells := TableCells(nil, TableBodyRow(r, rowH), cols)
	return append(out, ColumnGaps(win, TableColIDs(cols), nil, cells)...)
}

// TableColIDs are the columns' ids in order: the declared list of a
// window whose columns are a TableCol array (QA-002).
func TableColIDs(cols []TableCol) []string {
	if len(cols) == 0 {
		return nil
	}
	ids := make([]string, len(cols))
	for i, c := range cols {
		ids[i] = c.ID
	}
	return ids
}

func pxNum(v float32) string { return strconv.FormatFloat(float64(v), 'f', -1, 32) }

func pxSpan(a, b float32) string { return pxNum(a) + ".." + pxNum(b) }
