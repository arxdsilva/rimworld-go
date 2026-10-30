package forage

import "rimworld-go/internal/domain/world/region"

// Labels is the region map's component lookup (region.Map).
type Labels interface {
	Label(l region.Layer, x, y int) int32
}

// dirs is the fixed 8-neighbour order.
var dirs = [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

// Reach is a filter of the cells an animal standing at (px, py) of a w×h
// map could walk to on layer l, or stand beside for cells it cannot enter
// (a tree, a trough); nil when it stands off the map. fenced marks fence
// cells (nonzero; nil when none stand): food on a fence is behind it. A
// region-label compare per cell, never a search.
func Reach(m Labels, l region.Layer, px, py, w, h int, fenced []uint8) func(x, y int) bool {
	if px < 0 || py < 0 || px >= w || py >= h {
		return nil
	}
	own := m.Label(l, px, py)
	for _, d := range dirs { // standing on a shut cell (a door, a fence built around it): a neighbour's area
		if own >= 0 {
			break
		}
		own = m.Label(l, px+d[0], py+d[1])
	}
	if own < 0 {
		return func(int, int) bool { return false } // boxed in: nothing in reach
	}
	return func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		if lb := m.Label(l, x, y); lb >= 0 {
			return lb == own
		}
		if len(fenced) > 0 && fenced[y*w+x] != 0 {
			return false
		}
		for _, d := range dirs {
			if m.Label(l, x+d[0], y+d[1]) == own {
				return true
			}
		}
		return false
	}
}
