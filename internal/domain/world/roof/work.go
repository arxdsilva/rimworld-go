// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package roof

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
)

// NearestPlans selects the bounded stable nearest plans (row-major ties).
const PlanTries = 3

func NearestPlans(plans []int32, w, px, py int, admit func(int) bool) ([PlanTries]int, int) {
	type cand struct{ i, d int }
	var best [PlanTries]cand
	n := 0
	for _, p := range plans {
		i := int(p)
		if !admit(i) {
			continue
		}
		c := cand{i, pathing.Dist(px, py, i%w, i/w)}
		k := n
		for k > 0 && c.d < best[k-1].d {
			k--
		}
		if k >= PlanTries {
			continue
		}
		if n < PlanTries {
			n++
		}
		copy(best[k+1:n], best[k:n-1])
		best[k] = c
	}
	var out [PlanTries]int
	for i, c := range best[:n] {
		out[i] = c.i
	}
	return out, n
}

func Work(c Cell) (job.Kind, bool) {
	switch {
	case c.Has(PlanBuild|Supported) && !c.Roofed():
		return job.Roof, true
	case c.Has(PlanRemove) && Removable(c):
		return job.Unroof, true
	}
	return job.None, false
}

func WorkSpot(px, py, x, y int, walkable, reachable func(int, int) bool) (int, int, bool) {
	if walkable(x, y) {
		return x, y, true
	}
	bx, by, bd := 0, 0, 1<<30
	for _, d := range [8][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		nx, ny := x+d[0], y+d[1]
		if !walkable(nx, ny) {
			continue
		}
		if dd := pathing.Dist(px, py, nx, ny); dd < bd && reachable(nx, ny) {
			bx, by, bd = nx, ny, dd
		}
	}
	return bx, by, bd < 1<<30
}

// ClipRect normalizes and clips a rectangle to map bounds; max < min is empty.
func ClipRect(w, h, x1, y1, x2, y2 int) (int, int, int, int) {
	return max(min(x1, x2), 0), max(min(y1, y2), 0), min(max(x1, x2), w-1), min(max(y1, y2), h-1)
}
