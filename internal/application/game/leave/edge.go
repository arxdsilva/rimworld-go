// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package leave owns on-foot map departures shared by visitors and raiders.
package leave

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is the movement and structural pathing needed to leave a map.
type Host interface {
	Size() (int, int)
	Free(x, y int) bool
	StartJob(id entity.EntityID, j job.Record) bool
	StartBreach(id entity.EntityID, x, y int) bool
	BreachRoute(id entity.EntityID, x, y int) bool
}

type goal struct{ x, y int }
type side struct{ edge, distance int }

const (
	north = iota
	west
	east
	south
)

// Try plans a reachable edge, or breaches toward a walkable exit.
// A previously unreachable move is skipped on the next plan to avoid
// choosing the same failing route over and over.
func Try(h Host, id entity.EntityID, x, y, inset int, failed job.Record) bool {
	goals := exits(h, x, y, inset)
	var skipped goal
	hasSkipped := failed.Kind == job.Move && failed.State == job.Fail && failed.Fail == job.Unreachable
	if hasSkipped {
		skipped = goal{failed.TX, failed.TY}
	}
	for _, at := range goals {
		if hasSkipped && at == skipped {
			continue
		}
		if h.StartJob(id, job.Record{Kind: job.Move, TX: at.x, TY: at.y}) {
			return true
		}
	}
	for _, at := range goals {
		if h.StartBreach(id, at.x, at.y) {
			return true
		}
	}
	// A changed map may reopen the original route after a retry interval.
	// The caller limits retries to its normal cadence.
	if hasSkipped {
		return h.StartJob(id, job.Record{Kind: job.Move, TX: skipped.x, TY: skipped.y})
	}
	return false
}

// BreachRoute reports whether one of the map exits is structurally reachable.
func BreachRoute(h Host, id entity.EntityID, x, y, inset int) bool {
	for _, at := range exits(h, x, y, inset) {
		if h.BreachRoute(id, at.x, at.y) {
			return true
		}
	}
	return false
}

func exits(h Host, x, y, inset int) []goal {
	w, ht := h.Size()
	sides := [...]side{{north, y}, {west, x}, {east, w - 1 - x}, {south, ht - 1 - y}}
	slices.SortStableFunc(sides[:], func(a, b side) int { return cmp.Compare(a.distance, b.distance) })
	var goals []goal
	for _, s := range sides {
		if gx, gy, ok := edgeSpot(h, s.edge, x, y, w, ht, inset); ok {
			goals = append(goals, goal{gx, gy})
		}
	}
	return goals
}

func edgeSpot(h Host, side, x, y, w, ht, inset int) (int, int, bool) {
	var center, length int
	if side == north || side == south {
		center, length = x, w
	} else {
		center, length = y, ht
	}
	for band := min(inset, min(w, ht)-1); band >= 0; band-- {
		for offset := 0; offset < length; offset++ {
			for _, at := range [...]int{center + offset, center - offset} {
				if at < 0 || at >= length {
					continue
				}
				tx, ty := at, band
				switch side {
				case west:
					tx, ty = band, at
				case east:
					tx, ty = w-1-band, at
				case south:
					tx, ty = at, ht-1-band
				}
				if h.Free(tx, ty) {
					return tx, ty, true
				}
			}
		}
	}
	return 0, 0, false
}
