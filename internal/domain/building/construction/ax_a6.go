package construction

// plannedA6 is A6's kinds not live yet (recreation; AX-0 contract). This
// file is A6's, and so are the predicates below. Every A6 kind is live
// since AX-6 (.agents/handoffs/AX-6-recreation.md).
func plannedA6(Kind) bool { return false }

// Recreation reports a piece colonists enjoy their free time at (AX-6):
// the horseshoe pin, the game tables and boards, the televisions and the
// telescope.
func (k Kind) Recreation() bool {
	switch k {
	case HorseshoePin, RingToss, UrBoard, ChessTable, BilliardsTable, PokerTable, TubeTV, FlatscreenTV, BigScreenTV, Telescope:
		return true
	}
	return false
}

// IsTelevision reports a screen watched from in front of it.
func (k Kind) IsTelevision() bool { return k == TubeTV || k == FlatscreenTV || k == BigScreenTV }

// WatchArea is a television's watch area: width cells across, centred on
// the set, and depth rows out from it, the first one beside it (wiki:
// Tube television 3x5 = 15 tiles, Flatscreen television 5x6 = 30,
// Megascreen television 7x6 = 42). Zero for anything else.
func WatchArea(k Kind) (width, depth int) {
	switch k {
	case TubeTV:
		return 3, 5
	case FlatscreenTV:
		return 5, 6
	case BigScreenTV:
		return 7, 6
	}
	return 0, 0
}

// WatchCells are the cells colonists watch television k (facing f,
// top-left at x, y) from: the watch area in front of the screen, nearest
// rows first. Not a television: nil.
func WatchCells(k Kind, f Facing, x, y int) [][2]int {
	w, d := WatchArea(k)
	if w == 0 {
		return nil
	}
	return frontCells(k, f, x, y, w, d)
}

// frontCells is the w x d rectangle in front of k's footprint (facing f,
// at x, y), centred across it, row by row outward.
func frontCells(k Kind, f Facing, x, y, w, d int) [][2]int {
	def := DefOf(k)
	bw, bh := max(def.Size[0], 1), max(def.Size[1], 1)
	if !def.Rotatable {
		f = South
	}
	// south frame: the set covers (0..bw-1, 0..bh-1); the front is +y.
	minX, minY := 1<<30, 1<<30
	for cy := 0; cy < bh; cy++ {
		for cx := 0; cx < bw; cx++ {
			rx, ry := rotate(f, cx, cy)
			minX, minY = min(minX, rx), min(minY, ry)
		}
	}
	x0 := (bw - w) / 2
	if (bw-w)%2 != 0 && bw < w {
		x0-- // odd margin: the extra column goes left
	}
	out := make([][2]int, 0, w*d)
	for row := 0; row < d; row++ {
		for col := 0; col < w; col++ {
			rx, ry := rotate(f, x0+col, bh+row)
			out = append(out, [2]int{x + rx - minX, y + ry - minY})
		}
	}
	return out
}

// RingCells are the cells around k's footprint (at x, y facing f) a
// player stands or sits on: one beside each side cell, corners left out,
// in a fixed order (north edge, south, west, east).
func RingCells(k Kind, f Facing, x, y int) [][2]int {
	cells, _, _ := Footprint(k, f, x, y)
	if len(cells) == 0 {
		return nil
	}
	x0, y0, x1, y1 := cells[0][0], cells[0][1], cells[0][0], cells[0][1]
	for _, c := range cells {
		x0, y0, x1, y1 = min(x0, c[0]), min(y0, c[1]), max(x1, c[0]), max(y1, c[1])
	}
	out := make([][2]int, 0, 2*(x1-x0+1)+2*(y1-y0+1))
	for cx := x0; cx <= x1; cx++ {
		out = append(out, [2]int{cx, y0 - 1})
	}
	for cx := x0; cx <= x1; cx++ {
		out = append(out, [2]int{cx, y1 + 1})
	}
	for cy := y0; cy <= y1; cy++ {
		out = append(out, [2]int{x0 - 1, cy})
	}
	for cy := y0; cy <= y1; cy++ {
		out = append(out, [2]int{x1 + 1, cy})
	}
	return out
}
