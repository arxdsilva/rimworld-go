// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// assignEnglish is the texts an Assign column test declares its titles with.
type assignEnglish struct{}

func (assignEnglish) T(s string) string { return s }

// assignDeclared is the Assign window's declared column ids, in order.
func assignDeclared() []string { c := assignColumns(assignEnglish{}); return uikit.TableColIDs(c[:]) }

// assignRows is the Assign window's header row and its first colonist row in
// body, as body lays them out (assign.go): a header of assignHeadH, then rows
// of AssignRowH-4 height.
func assignRows(body gfx.Rect) (head, row gfx.Rect) {
	head = gfx.Rect{X: body.X, Y: body.Y, W: body.W, H: assignHeadH}
	row = gfx.Rect{X: body.X, Y: body.Y + assignHeadH, W: body.W, H: AssignRowH - 4}
	return head, row
}

// TestAssignCellsKeepTheirAnchors pins the columns where they were before
// assignCells: numeric literals worked out from the old column offsets.
func TestAssignCellsKeepTheirAnchors(t *testing.T) {
	t.Parallel()
	type xw struct {
		col  string
		x, w float32
	}
	cases := []struct {
		name string
		row  gfx.Rect
		want []xw
	}{
		{"full width 840", gfx.Rect{X: 100, Y: 50, W: 840, H: 26}, []xw{
			{"colonist", 100, 140}, {"drugs", 240, 130}, {"medical", 380, 120},
			{"food", 510, 150}, {"apparel", 670, 130}, {"clear", 810, 90}}},
		{"shrunk to half 505", gfx.Rect{X: 100, Y: 50, W: 505, H: 26}, []xw{
			{"colonist", 100, 70}, {"drugs", 170, 65}, {"medical", 245, 60},
			{"food", 315, 150}, {"apparel", 475, 65}, {"clear", 550, 45}}},
	}
	for _, c := range cases {
		var buf [6]uikit.TableCell
		got := assignCells(buf[:0], c.row)
		if len(got) != len(c.want) {
			t.Fatalf("%s: %d cells, want %d: %+v", c.name, len(got), len(c.want), got)
		}
		for i, w := range c.want {
			g := got[i]
			if g.Col != w.col || g.R.X != w.x || g.R.W != w.w || g.R.Y != c.row.Y || g.R.H != c.row.H {
				t.Errorf("%s cell %d: %+v, want %s x=%v w=%v on row %+v", c.name, i, g, w.col, w.x, w.w, c.row)
			}
		}
	}
}

// TestAssignColumnsDrawn: every declared column has a cell inside the window
// in the header and in a colonist row, at both column screens, through the
// production window rect and body.
func TestAssignColumnsDrawn(t *testing.T) {
	t.Parallel()
	declared := assignDeclared()
	if len(declared) != 6 || declared[5] != "clear" {
		t.Fatalf("declared %v", declared)
	}
	for _, s := range uikit.ColumnScreens() {
		win, body := windowsKitBody(t, func(w *Set) { w.Assign().Launch() }, s.W, s.H)
		head, row := assignRows(body)
		for name, r := range map[string]gfx.Rect{"header": head, "row": row} {
			var buf [6]uikit.TableCell
			if gaps := uikit.ColumnGaps(win, declared, nil, assignCells(buf[:0], r)); gaps != nil {
				t.Errorf("%dx%d %s (window %+v, body %+v): %s", s.W, s.H, name, win, body, strings.Join(gaps, "; "))
			}
		}
	}
}

// TestAssignColumnTitles: the header reads the declared titles, and Clear
// forced has none.
func TestAssignColumnTitles(t *testing.T) {
	t.Parallel()
	c := assignColumns(assignEnglish{})
	want := []string{"Colonist", "Drugs", "Medical", "Food", "Apparel", ""}
	for i, w := range want {
		if c[i].Title != w {
			t.Errorf("column %d (%s) title %q, want %q", i, c[i].ID, c[i].Title, w)
		}
	}
}

// TestAssignWidgetsUseCells: the row buttons and the header tips and button
// sit on the rects assignCells gives.
func TestAssignWidgetsUseCells(t *testing.T) {
	t.Parallel()
	sh, w := openAssignWindow(t, 1)
	body := gfx.Rect{X: 20, Y: 40, W: 840, H: 300}
	sh.kit.Begin()
	w.Assign().body(body)
	head, row := assignRows(body)
	var hb, rb [6]uikit.TableCell
	hc, rc := assignCells(hb[:0], head), assignCells(rb[:0], row)
	byID := map[string]gfx.Rect{}
	for _, wd := range sh.kit.Cur {
		byID[wd.ID] = wd.R
	}
	rowCol := func(id, col string) {
		t.Helper()
		r, ok := byID[id]
		if !ok {
			t.Fatalf("no widget %s", id)
		}
		c, _ := assignCellRect(rc, col)
		if r.X != c.X || r.W != c.W || r.H != c.H {
			t.Errorf("%s at %+v, cell %q %+v", id, r, col, c)
		}
	}
	rowCol(AssignDrugID(1), "drugs")
	rowCol(AssignCareID(1), "medical")
	rowCol(AssignFoodID(1), "food")
	rowCol(AssignApparelID(1), "apparel")
	rowCol(AssignClearID(1), "clear")
	for id, col := range map[string]string{"assign.head.drugs": "drugs", "assign.head.care": "medical", "assign.head.food": "food"} {
		r, ok := byID[id]
		c, _ := assignCellRect(hc, col)
		if !ok || r.X != c.X || r.W != c.W {
			t.Errorf("%s at %+v (found %v), header cell %q %+v", id, r, ok, col, c)
		}
	}
	if r, c := byID[AssignApparelEditID], func() gfx.Rect { c, _ := assignCellRect(hc, "apparel"); return c }(); r.X != c.X || r.W != c.W {
		t.Errorf("apparel header at %+v, cell %+v", r, c)
	}
}

// TestAssignCellRectMissing: a column assignCells did not place has no rect.
func TestAssignCellRectMissing(t *testing.T) {
	t.Parallel()
	var buf [6]uikit.TableCell
	cells := assignCells(buf[:0], gfx.Rect{X: 0, Y: 0, W: 100, H: 26})
	if _, ok := assignCellRect(cells, "clear"); ok {
		t.Errorf("clear placed in a 100 px row: %+v", cells)
	}
	if _, ok := assignCellRect(cells, "colonist"); !ok {
		t.Errorf("colonist missing: %+v", cells)
	}
}
