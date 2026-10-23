package ebiten

import (
	"image/color"
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Table (NG-1 §1.4): a header row with sortable columns above a list.
// Rows are data rows (0..n-1); sorting only changes the display order.

const (
	tableHeaderH = 28
	tableCellPad = 8
)

// sortState is a table's sort: column index and direction.
type sortState struct {
	col  int
	desc bool
}

// tableCol is one column.
type tableCol struct {
	ID, Title string
	W         float32 // px; 0 shares the remaining width equally
	Align     align
	// Cell is the cell text; Draw replaces it (icons, bars). dst is clipped
	// to the table body and cell is in screen coordinates.
	Cell func(row int) string
	Draw func(dst *ebiten.Image, row int, cell rect)
	// Less orders two data rows ascending; nil makes the column unsortable.
	Less func(a, b int) bool
	// Tip is the cell's hover card.
	Tip func(row int) string
}

// tableColRects splits width w (from x) into the columns' rectangles.
func tableColRects(x, y, w, h float32, cols []tableCol) []rect {
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
		share = max(0, w-fixed) / float32(flex)
	}
	out := make([]rect, len(cols))
	for i, c := range cols {
		cw := c.W
		if cw <= 0 {
			cw = share
		}
		out[i] = rect{x, y, cw, h}
		x += cw
	}
	return out
}

// uiTableSetSort sets a table's sort (col < 0 clears it).
func (a *App) uiTableSetSort(id string, col int, desc bool) {
	u := a.uiSt()
	if u.sorts == nil {
		u.sorts = map[string]sortState{}
	}
	if col < 0 {
		delete(u.sorts, id)
		return
	}
	u.sorts[id] = sortState{col, desc}
}

// uiTableSort is a table's current sort column (-1: none) and direction.
func (a *App) uiTableSort(id string) (col int, desc bool) {
	if s, ok := a.uiSt().sorts[id]; ok {
		return s.col, s.desc
	}
	return -1, false
}

// tableOrder is the data rows in display order for the current sort.
func (a *App) tableOrder(id string, cols []tableCol, n int) []int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	col, desc := a.uiTableSort(id)
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

// uiTable is a sortable table of n data rows in r, bound to the selected
// data row *sel (-1: none). Clicking a sortable header sorts by it, again
// reverses it. The body is a list (id+".body", row ids by display
// position) with hover and selection; cells with a Tip get tooltips. It
// reports the selection changed.
func (a *App) uiTable(id string, r rect, cols []tableCol, n int, sel *int, rowH float32) bool {
	u := a.uiSt()
	if rowH <= 0 {
		rowH = listRowH
	}
	head, body := r.splitV(tableHeaderH)
	cr := tableColRects(head.X, head.Y, head.W-scrollBarW, tableHeaderH, cols)
	for i, c := range cols {
		if c.Less == nil {
			continue
		}
		i := i
		cycle := func() {
			col, desc := a.uiTableSort(id)
			a.uiTableSetSort(id, i, col == i && !desc)
		}
		if u.add(uiWidget{id: id + ".col." + c.ID, r: cr[i], click: cycle}) && a.uiPress(cr[i]) {
			cycle()
			a.uiOK()
		}
	}
	order := a.tableOrder(id, cols, n)
	disp := -1
	for di, row := range order {
		if row == *sel {
			disp = di
		}
	}
	bodyID := id + ".body"
	rcols := tableColRects(body.X, 0, body.W-scrollBarW, rowH, cols)
	changed := a.uiList(bodyID, body, n, &disp, listSpec{
		RowH: rowH,
		Draw: func(dst *ebiten.Image, di int, row rect, hover, selected bool) {
			hover = hover || row.hit(u.mx, u.my) && body.hit(u.mx, u.my)
			switch {
			case selected:
				fillRect(dst, row, uiBtnSelected)
			case hover:
				fillRect(dst, row, uiBtnHover)
			case di%2 == 1:
				// a faint stripe: non-premultiplied, so the 16 % alpha scales the colour
				fillRect(dst, row, color.NRGBA{R: uiPanelBorder.R, G: uiPanelBorder.G, B: uiPanelBorder.B, A: 0x28})
			}
			for ci, c := range cols {
				cell := rect{rcols[ci].X, row.Y, rcols[ci].W, row.H}
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
	first, last := listVisible(u.scroll[bodyID], body.H, rowH, n)
	for ci, c := range cols {
		if c.Tip == nil {
			continue
		}
		for di := first; di <= last; di++ {
			cell := rect{rcols[ci].X, body.Y + float32(di)*rowH - u.scroll[bodyID], rcols[ci].W, rowH}
			u.add(uiWidget{id: id + ".cell." + uiNum(di) + "." + c.ID, r: cell.intersect(body), tip: c.Tip(order[di])})
		}
	}
	col, desc := a.uiTableSort(id)
	u.queue(func(dst *ebiten.Image) {
		fillRect(dst, head, uiBtn)
		strokeRect(dst, head, uiBtnBorder)
		for i, c := range cols {
			st := tsLabel
			st.Color = uiHeading
			cell := cr[i]
			if c.Less != nil && u.hot == id+".col."+c.ID {
				fillRect(dst, cell, uiBtnHover)
			}
			drawCell(dst, cell, c.Title, c.Align, st)
			if i == col {
				cx, cy := cell.right()-12, cell.Y+cell.H/2
				if desc {
					fillTri(dst, cx-4, cy-2, cx+4, cy-2, cx, cy+3)
				} else {
					fillTri(dst, cx-4, cy+2, cx+4, cy+2, cx, cy-3)
				}
			}
		}
	})
	return changed
}

// drawCell draws text in a cell with padding and alignment.
func drawCell(dst *ebiten.Image, cell rect, s string, al align, sts ...textStyle) {
	st := tsBody
	if len(sts) > 0 {
		st = sts[0]
	}
	s = fitText(s, st, cell.W-2*tableCellPad)
	st.Align = al
	x := float64(cell.X + tableCellPad)
	switch al {
	case alignCenter:
		x = float64(cell.X + cell.W/2)
	case alignRight:
		x = float64(cell.right() - tableCellPad)
	}
	drawText(dst, s, x, float64(cell.Y)+(float64(cell.H)-lineH(st))/2, st)
}

func fillTri(dst *ebiten.Image, x0, y0, x1, y1, x2, y2 float32) {
	var p vector.Path
	p.MoveTo(x0, y0)
	p.LineTo(x1, y1)
	p.LineTo(x2, y2)
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(uiHeading)
	vector.FillPath(dst, &p, nil, op)
}
