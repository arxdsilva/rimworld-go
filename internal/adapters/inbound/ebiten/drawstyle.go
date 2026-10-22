package ebiten

import "sort"

// Architect draw styles (UI-016): drag tools draw a line, a filled or
// empty rectangle, or a filled or empty oval. Q/E cycle the style on drag
// tools (rotatable single placements keep Q/E for rotation); Ctrl makes
// the box a square (circle). Walls, fences, sandbags and floors get all
// five; zones and orders a filled rectangle or a filled oval, sent as one
// rectangle command per row run.

type drawStyle uint8

const (
	styleDefault drawStyle = iota // the tool's own default
	styleLine
	styleRect
	styleRectEdge
	styleOval
	styleOvalEdge
)

var styleNames = map[drawStyle]string{
	styleLine: "Line", styleRect: "Filled rectangle", styleRectEdge: "Empty rectangle",
	styleOval: "Filled oval", styleOvalEdge: "Empty oval",
}

// toolStyles is the styles a tool offers (none for single placements).
func toolStyles(t archTool) []drawStyle {
	switch t.mode {
	case placeLine, placeFill:
		return []drawStyle{styleLine, styleRect, styleRectEdge, styleOval, styleOvalEdge}
	case placeArea:
		return []drawStyle{styleRect, styleOval}
	}
	return nil
}

func toolDefaultStyle(t archTool) drawStyle {
	switch t.mode {
	case placeLine:
		return styleLine
	case placeFill, placeArea:
		return styleRect
	}
	return styleDefault
}

// curStyle is the tool's style now: the player's pick when the tool
// offers it, else the tool's default.
func (a *App) curStyle(t archTool) drawStyle {
	for _, s := range toolStyles(t) {
		if s == a.archStyle {
			return s
		}
	}
	return toolDefaultStyle(t)
}

// cycleStyle steps through styles by dir from cur.
func cycleStyle(styles []drawStyle, cur drawStyle, dir int) drawStyle {
	for i, s := range styles {
		if s == cur {
			return styles[(i+dir+len(styles))%len(styles)]
		}
	}
	return styles[0]
}

// rotateOrStyle is Q/E with a tool in hand: drag tools change style,
// everything else rotates.
func (a *App) rotateOrStyle(dir int) {
	t, ok := toolByID(a.archTool)
	if !ok {
		return
	}
	if styles := toolStyles(t); len(styles) > 1 {
		a.archStyle = cycleStyle(styles, a.curStyle(t), dir)
		return
	}
	a.rotateTool(dir)
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
	n := max(abs(x1-x0), abs(y1-y0))
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

// shapeCells is the cells a drag from a to b covers in style, rows top to
// bottom and columns left to right (a line runs from the drag start).
// ctrl squares the box for rectangles and ovals.
func shapeCells(style drawStyle, a, b [2]int, ctrl bool) [][2]int {
	if style == styleLine || style == styleDefault {
		mode := placeLine
		if style == styleDefault {
			mode = placeSingle
		}
		return placementCells(mode, a[0], a[1], b[0], b[1])
	}
	if ctrl {
		b[0], b[1] = constrainEnd(a[0], a[1], b[0], b[1])
	}
	x0, y0, x1, y1 := normRect(a[0], a[1], b[0], b[1])
	in := func(x, y int) bool {
		if x < x0 || x > x1 || y < y0 || y > y1 {
			return false
		}
		if style == styleOval || style == styleOvalEdge {
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
			if style == styleRectEdge || style == styleOvalEdge {
				if in(x-1, y) && in(x+1, y) && in(x, y-1) && in(x, y+1) {
					continue // inside: only the outline is drawn
				}
			}
			out = append(out, [2]int{x, y})
		}
	}
	return out
}

// rowRun is one row's contiguous span of cells.
type rowRun struct{ y, x0, x1 int }

// rowRuns splits cells into contiguous runs per row (an oval sent as
// rectangle commands).
func rowRuns(cells [][2]int) []rowRun {
	cs := append([][2]int(nil), cells...)
	sort.Slice(cs, func(i, j int) bool {
		if cs[i][1] != cs[j][1] {
			return cs[i][1] < cs[j][1]
		}
		return cs[i][0] < cs[j][0]
	})
	var out []rowRun
	for _, c := range cs {
		if n := len(out); n > 0 && out[n-1].y == c[1] && out[n-1].x1+1 == c[0] {
			out[n-1].x1 = c[0]
			continue
		}
		out = append(out, rowRun{y: c[1], x0: c[0], x1: c[0]})
	}
	return out
}
