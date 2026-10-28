package architect

import "sort"

// Architect draw styles (UI-016): drag tools draw a line, a filled or
// empty rectangle, or a filled or empty oval. Q/E cycle the style on drag
// tools (rotatable single placements keep Q/E for rotation); Ctrl makes
// the box a square (circle). Walls, fences, sandbags and floors get all
// five; zones and orders a filled rectangle or a filled oval, sent as one
// rectangle command per row run.

// DrawStyle is the shape a drag tool draws.
type DrawStyle uint8

const (
	StyleDefault DrawStyle = iota // the tool's own default
	StyleLine
	StyleRect
	StyleRectEdge
	StyleOval
	StyleOvalEdge
)

// StyleNames are the styles' names in the placement hint.
var StyleNames = map[DrawStyle]string{
	StyleLine: "Line", StyleRect: "Filled rectangle", StyleRectEdge: "Empty rectangle",
	StyleOval: "Filled oval", StyleOvalEdge: "Empty oval",
}

// ToolStyles is the styles a tool offers (none for single placements).
func ToolStyles(t ToolDef) []DrawStyle {
	switch t.Mode {
	case PlaceLine, PlaceFill:
		return []DrawStyle{StyleLine, StyleRect, StyleRectEdge, StyleOval, StyleOvalEdge}
	case PlaceArea:
		return []DrawStyle{StyleRect, StyleOval}
	}
	return nil
}

func toolDefaultStyle(t ToolDef) DrawStyle {
	switch t.Mode {
	case PlaceLine:
		return StyleLine
	case PlaceFill, PlaceArea:
		return StyleRect
	}
	return StyleDefault
}

// CurStyle is the tool's style now: the player's pick when the tool
// offers it, else the tool's default.
func (t *Tool) CurStyle(d ToolDef) DrawStyle {
	for _, s := range ToolStyles(d) {
		if s == t.Style {
			return s
		}
	}
	return toolDefaultStyle(d)
}

// cycleStyle steps through styles by dir from cur.
func cycleStyle(styles []DrawStyle, cur DrawStyle, dir int) DrawStyle {
	for i, s := range styles {
		if s == cur {
			return styles[(i+dir+len(styles))%len(styles)]
		}
	}
	return styles[0]
}

// rotateOrStyle is Q/E with a tool in hand: drag tools change style,
// everything else rotates.
func (t *Tool) rotateOrStyle(dir int) {
	d, ok := ToolByID(t.ToolID)
	if !ok {
		return
	}
	if styles := ToolStyles(d); len(styles) > 1 {
		t.Style = cycleStyle(styles, t.CurStyle(d), dir)
		return
	}
	t.RotateTool(dir)
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 1
}

// constrainEnd moves the drag end so the box is square (Ctrl).
func constrainEnd(x0, y0, x1, y1 int) (int, int) {
	n := max(Abs(x1-x0), Abs(y1-y0))
	return x0 + sign(x1-x0)*n, y0 + sign(y1-y0)*n
}

// ovalPad widens the ellipse past the cell centres so small ovals stay
// round (a 3x3 is a plus, a 5x5 a disc without its corners).
const ovalPad = 0.4

// inOval reports cell (x, y) lies in the oval inscribed in the box.
func inOval(x, y, x0, y0, x1, y1 int) bool {
	cx, cy := float64(x0+x1)/2, float64(y0+y1)/2
	rx, ry := float64(x1-x0)/2+ovalPad, float64(y1-y0)/2+ovalPad
	dx, dy := (float64(x)-cx)/rx, (float64(y)-cy)/ry
	return dx*dx+dy*dy <= 1
}

// ShapeCells is the cells a drag from a to b covers in style, rows top to
// bottom and columns left to right (a line runs from the drag start).
// ctrl squares the box for rectangles and ovals.
func ShapeCells(style DrawStyle, a, b [2]int, ctrl bool) [][2]int {
	if style == StyleLine || style == StyleDefault {
		mode := PlaceLine
		if style == StyleDefault {
			mode = PlaceSingle
		}
		return PlacementCells(mode, a[0], a[1], b[0], b[1])
	}
	if ctrl {
		b[0], b[1] = constrainEnd(a[0], a[1], b[0], b[1])
	}
	x0, y0, x1, y1 := normRect(a[0], a[1], b[0], b[1])
	in := func(x, y int) bool {
		if x < x0 || x > x1 || y < y0 || y > y1 {
			return false
		}
		if style == StyleOval || style == StyleOvalEdge {
			return inOval(x, y, x0, y0, x1, y1)
		}
		return true
	}
	var out [][2]int
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !in(x, y) {
				continue
			}
			if style == StyleRectEdge || style == StyleOvalEdge {
				if in(x-1, y) && in(x+1, y) && in(x, y-1) && in(x, y+1) {
					continue // inside: only the outline is drawn
				}
			}
			out = append(out, [2]int{x, y})
		}
	}
	return out
}

// RowRun is one row's contiguous span of cells.
type RowRun struct{ Y, X0, X1 int }

// RowRuns splits cells into contiguous runs per row (an oval sent as
// rectangle commands).
func RowRuns(cells [][2]int) []RowRun {
	cs := append([][2]int(nil), cells...)
	sort.Slice(cs, func(i, j int) bool {
		if cs[i][1] != cs[j][1] {
			return cs[i][1] < cs[j][1]
		}
		return cs[i][0] < cs[j][0]
	})
	var out []RowRun
	for _, c := range cs {
		if n := len(out); n > 0 && out[n-1].Y == c[1] && out[n-1].X1+1 == c[0] {
			out[n-1].X1 = c[0]
			continue
		}
		out = append(out, RowRun{Y: c[1], X0: c[0], X1: c[0]})
	}
	return out
}
