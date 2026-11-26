// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package beacon is the cells an orbital trade beacon trades from. It
// imports nothing from the game package.
package beacon

import (
	"iter"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Diameter is a trade beacon's reach across, in cells.
const Diameter = 15 // wiki: Orbital_trade_beacon ("15 tiles in diameter")

const (
	// reachR is the reach's half-width in whole cells, and reachR2 the
	// largest dx²+dy² inside the disc of diameter 15 (7.5² = 56.25).
	reachR  = Diameter / 2
	reachR2 = Diameter * Diameter / 4
	// side is the reach's bounding box edge.
	side = 2*reachR + 1
)

// Reach calls add with the index (y*w+x) of every cell of the w x h map a
// beacon at (bx, by) trades from, in ascending index order. The cells are
// those of the disc 15 across (dx²+dy² ≤ 56) that a 4-neighbour flood from
// the beacon reaches without entering a cell blocked reports (a solid
// wall: wiki, Orbital trade beacon, "solid walls will break the area");
// nil blocks nothing. The beacon's own cell is always in, and blocked is
// never asked about it; an off-map beacon reaches nothing. It allocates
// nothing: the queue and the marks are fixed arrays over the box.
func Reach(w, h, bx, by int, blocked func(x, y int) bool, add func(i int)) {
	if bx < 0 || by < 0 || bx >= w || by >= h {
		return
	}
	var seen [side * side]bool
	var queue [side * side]int // box cells, (dy+reachR)*side + dx+reachR
	start := reachR*side + reachR
	seen[start], queue[0] = true, start
	for head, tail := 0, 1; head < tail; head++ {
		c := queue[head]
		for _, d := range [4][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} { // E, S, W, N
			dx, dy := c%side-reachR+d[0], c/side-reachR+d[1]
			x, y, n := bx+dx, by+dy, (dy+reachR)*side+dx+reachR
			if dx*dx+dy*dy > reachR2 || x < 0 || y < 0 || x >= w || y >= h || seen[n] {
				continue
			}
			if blocked != nil && blocked(x, y) {
				continue
			}
			seen[n], queue[tail] = true, n
			tail++
		}
	}
	for n, in := range seen {
		if in {
			add((by+n/side-reachR)*w + bx + n%side - reachR)
		}
	}
}

// Powered yields the live powered trade equipment in the original
// building ID order. A console caller can stop at the first one.
func Powered(ids []entity.EntityID, kind construction.Kind, building func(entity.EntityID) *construction.Building, powered func(entity.EntityID) bool) iter.Seq[entity.EntityID] {
	return func(yield func(entity.EntityID) bool) {
		for _, id := range ids {
			if b := building(id); b != nil && b.Kind == kind && powered(id) {
				if !yield(id) {
					return
				}
			}
		}
	}
}
