// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package uikit

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"slices"
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Table (NG-1 §1.4): a header row with sortable columns above a list.
// Rows are data rows (0..n-1); sorting only changes the display order.

const (
	TableHeaderH = 28
	TableCellPad = 8
)

// sortState is a table's sort: column index and direction.
type sortState struct {
	col  int
	desc bool
}

// TableCol is one column.
type TableCol struct {
	ID, Title string
	W         float32 // px; 0 shares the remaining width equally
	Align     gfx.Align
	// Cell is the cell text; Draw replaces it (icons, bars). dst is clipped
	// to the table body and cell is in screen coordinates.
	Cell func(row int) string
	Draw func(dst *ebiten.Image, row int, cell gfx.Rect)
	// Less orders two data rows ascending; nil makes the column unsortable.
	Less func(a, b int) bool
	// Tip is the cell's hover card.
	Tip func(row int) string
}

// TableCells appends row's cells, one per column in order (QA-002): fixed
// widths first, the rest of row.W shared by the W 0 columns (never below 0),
// from row.X, with row's Y and H.
func TableCells(dst []TableCell, row gfx.Rect, cols []TableCol) []TableCell {
	fixed, flex := float32(0), 0
	for _, c := range cols {
		if c.W > 0 {
			fixed += c.W
		} else {
			flex++
		}
	}
	share := float32(0)
	if flex > 0 {
		share = max(0, row.W-fixed) / float32(flex)
	}
	dst = slices.Grow(dst, len(cols))
	x := row.X
	for _, c := range cols {
		cw := c.W
		if cw <= 0 {
			cw = share
		}
		dst = append(dst, TableCell{Col: c.ID, R: gfx.Rect{X: x, Y: row.Y, W: cw, H: row.H}})
		x += cw
	}
	return dst
}

// TableBodyRow is the first body row of a Table in r (QA-002): under the
// header, left of the scroll bar, rowH tall (ListRowH when rowH <= 0).
func TableBodyRow(r gfx.Rect, rowH float32) gfx.Rect {
	if rowH <= 0 {
		rowH = ListRowH
	}
	_, body := r.SplitV(TableHeaderH)
	return gfx.Rect{X: body.X, Y: body.Y, W: body.W - ScrollBarW, H: rowH}
}

// tableHeadCell is a column's header cell: its body cell's X and W in head.
func tableHeadCell(head gfx.Rect, c TableCell) gfx.Rect {
	return gfx.Rect{X: c.R.X, Y: head.Y, W: c.R.W, H: TableHeaderH}
}

// TableSetSort sets a table's sort (col < 0 clears it).
func (k *Kit) TableSetSort(id string, col int, desc bool) {
	if k.sorts == nil {
		k.sorts = map[string]sortState{}
	}
	if col < 0 {
		delete(k.sorts, id)
		return
	}
	k.sorts[id] = sortState{col, desc}
}

// TableSort is a table's current sort column (-1: none) and direction.
func (k *Kit) TableSort(id string) (col int, desc bool) {
	if s, ok := k.sorts[id]; ok {
		return s.col, s.desc
	}
	return -1, false
}

// TableOrder is the data rows in display order for the current sort.
func (k *Kit) TableOrder(id string, cols []TableCol, n int) []int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	col, desc := k.TableSort(id)
	if col < 0 || col >= len(cols) || cols[col].Less == nil {
		return order
	}
	less := cols[col].Less
	sort.SliceStable(order, func(i, j int) bool {
		if desc {
			return less(order[j], order[i])
		}
		return less(order[i], order[j])
	})
	return order
}

// Table is a sortable table of n data rows in r, bound to the selected
// data row *sel (-1: none). Clicking a sortable header sorts by it, again
// reverses it. The body is a list (id+".body", row ids by display
// position) with hover and selection; cells with a Tip get tooltips. It
// reports the selection changed.
func (k *Kit) Table(id string, r gfx.Rect, cols []TableCol, n int, sel *int, rowH float32) bool {
	if rowH <= 0 {
		rowH = ListRowH
	}
	head, body := r.SplitV(TableHeaderH)
	cells := TableCells(nil, TableBodyRow(r, rowH), cols)
	for i, c := range cols {
		if c.Less == nil {
			continue
		}
		i := i
		cycle := func() {
			col, desc := k.TableSort(id)
			k.TableSetSort(id, i, col == i && !desc)
		}
		if hr := tableHeadCell(head, cells[i]); k.Add(Widget{ID: id + ".col." + c.ID, R: hr, Click: cycle}) && k.Press(hr) {
			cycle()
			k.OK()
		}
	}
	order := k.TableOrder(id, cols, n)
	disp := -1
	for di, row := range order {
		if row == *sel {
			disp = di
		}
	}
	bodyID := id + ".body"
	changed := k.List(bodyID, body, n, &disp, ListSpec{
		RowH: rowH,
		Draw: func(dst *ebiten.Image, di int, row gfx.Rect, hover, selected bool) {
			hover = hover || row.Hit(k.mx, k.my) && body.Hit(k.mx, k.my)
			switch {
			case selected:
				FillRect(dst, row, BtnSelected)
			case hover:
				FillRect(dst, row, BtnHover)
			case di%2 == 1:
				// a faint stripe: non-premultiplied, so the 16 % alpha scales the colour
				FillRect(dst, row, color.NRGBA{R: PanelBorder.R, G: PanelBorder.G, B: PanelBorder.B, A: 0x28})
			}
			for ci, c := range cols {
				cell := gfx.Rect{X: cells[ci].R.X, Y: row.Y, W: cells[ci].R.W, H: row.H}
				if c.Draw != nil {
					c.Draw(dst, order[di], cell)
					continue
				}
				if c.Cell != nil {
					drawCell(dst, cell, c.Cell(order[di]), c.Align)
				}
			}
		},
	})
	if changed && disp >= 0 && disp < len(order) {
		*sel = order[disp]
	}
	first, last := ListVisible(k.Scroll[bodyID], body.H, rowH, n)
	for ci, c := range cols {
		if c.Tip == nil {
			continue
		}
		for di := first; di <= last; di++ {
			cell := gfx.Rect{X: cells[ci].R.X, Y: body.Y + float32(di)*rowH - k.Scroll[bodyID], W: cells[ci].R.W, H: rowH}
			k.Add(Widget{ID: id + ".cell." + Num(di) + "." + c.ID, R: cell.Intersect(body), Tip: c.Tip(order[di])})
		}
	}
	col, desc := k.TableSort(id)
	k.Queue(func(dst *ebiten.Image) {
		FillRect(dst, head, Btn)
		StrokeRect(dst, head, BtnBorder)
		for i, c := range cols {
			st := gfx.Label
			st.Color = Heading
			cell := tableHeadCell(head, cells[i])
			if c.Less != nil && k.Hot == id+".col."+c.ID {
				FillRect(dst, cell, BtnHover)
			}
			drawCell(dst, cell, c.Title, c.Align, st)
			if i == col {
				cx, cy := cell.Right()-12, cell.Y+cell.H/2
				if desc {
					FillTri(dst, cx-4, cy-2, cx+4, cy-2, cx, cy+3)
				} else {
					FillTri(dst, cx-4, cy+2, cx+4, cy+2, cx, cy-3)
				}
			}
		}
	})
	return changed
}

// drawCell draws text in a cell with padding and alignment.
func drawCell(dst *ebiten.Image, cell gfx.Rect, s string, al gfx.Align, sts ...gfx.TextStyle) {
	st := gfx.Body
	if len(sts) > 0 {
		st = sts[0]
	}
	s = FitText(s, st, cell.W-2*TableCellPad)
	st.Align = al
	x := float64(cell.X + TableCellPad)
	switch al {
	case gfx.AlignCenter:
		x = float64(cell.X + cell.W/2)
	case gfx.AlignRight:
		x = float64(cell.Right() - TableCellPad)
	}
	gfx.Text(dst, s, x, gfx.CenterTop(float64(cell.Y), float64(cell.H), st), st)
}

func FillTri(dst *ebiten.Image, x0, y0, x1, y1, x2, y2 float32) {
	gfx.FillPoly(dst, [][2]float32{{x0, y0}, {x1, y1}, {x2, y2}}, Heading, true)
}
