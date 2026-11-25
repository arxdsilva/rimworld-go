// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package care

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// CrawlDelay and CrawlStepTicks are local timing choices, not sourced
// vanilla constants. They are intentionally slower than walking.
const CrawlDelay = 1200
const CrawlStepTicks = 120

func CrawlReady(v health.Record, now uint64, conscious, manip int32) bool {
	return v.Status == health.Downed && v.Cause == health.CauseInjury &&
		now >= v.DownedTick+CrawlDelay && (now-v.DownedTick-CrawlDelay)%CrawlStepTicks == 0 &&
		conscious >= health.DownConsciousness && manip >= 100
}

// CrawlBed returns the next actual path step to the nearest accessible
// appropriate bed. No bed teleport: the slot is claimed only when its
// sleep cell has been reached. A rescuer/doctor's reservation wins.
func (c *Care) CrawlBed(patient entity.EntityID) (next pathing.Point, move, settled bool) {
	h := c.h
	k := c.kindOf(patient)
	if k != targetColonist && k != targetPrisoner && k != targetGuest && k != targetStranger {
		return
	}
	if c.Carry.Carrier(patient) != 0 || h.BedUnder(patient) != 0 {
		return next, false, true
	}
	if _, ok := h.Owner(patient); ok {
		return next, false, true
	}
	x, y, ok := h.Pos(patient)
	if !ok {
		return
	}
	q := rescueQuery(c, patient, x, y)
	q.carrier = patient
	bestDist := int(^uint(0) >> 1)
	var best entity.EntityID
	var route []pathing.Point
	for _, bid := range h.BuildingIDs() {
		b, ok := h.Building(bid)
		if !ok || !q.eligible(c, bid, b) || q.bedWhy(c, bid, b) != "" {
			continue
		}
		sx, sy := h.SleepCell(patient, b)
		d := manhattan(x, y, sx, sy)
		if d > bestDist || d == bestDist && best != 0 && bid >= best {
			continue
		}
		path, ok := h.PathTo(patient, sx, sy)
		if !ok || len(path) == 0 && (x != sx || y != sy) || len(path) > 0 && path[len(path)-1] != (pathing.Point{X: sx, Y: sy}) {
			continue
		}
		best, bestDist, route = bid, d, path
	}
	if best == 0 {
		return
	}
	if len(route) == 0 {
		c.layInBed(patient, best, k != targetPrisoner)
		return next, false, true
	}
	return route[0], true, false
}

// FleeCrawl chooses a cardinal safe step that increases distance to the
// nearest immediate threat. Direction order is stable.
func FleeCrawl(x, y, tx, ty int, free func(int, int) bool) (pathing.Point, bool) {
	best := mathx.Abs(x-tx) + mathx.Abs(y-ty)
	out := pathing.Point{X: x, Y: y}
	for _, d := range [4][2]int{{0, 1}, {1, 0}, {-1, 0}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		dist := mathx.Abs(nx-tx) + mathx.Abs(ny-ty)
		if dist > best && free(nx, ny) {
			best, out = dist, pathing.Point{X: nx, Y: ny}
		}
	}
	return out, out.X != x || out.Y != y
}
