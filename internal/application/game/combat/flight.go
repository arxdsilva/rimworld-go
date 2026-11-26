// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

const fleeDist = 8

// Flight considers all nearby attackers, including those just beyond the
// release radius that a fleeing pawn might otherwise run into.
func (gd *Guard) flightThreats(id entity.EntityID, x, y int, primary Seen, now uint64) []Seen {
	gd.escape = gd.h.Near(x, y, RaiderRelease+fleeDist, Animals|Raiders, gd.escape[:0])
	out := gd.escape[:0]
	for _, s := range gd.escape {
		if s.ID == primary.ID {
			continue
		}
		if _, _, _, ok := gd.ThreatTo(id, s.ID, x, y, FleeRelease+fleeDist+1, RaiderRelease+fleeDist+1); ok ||
			gd.potentialFoe(s.ID, now) && gd.st.sight.Line(x, y, s.X, s.Y) {
			out = append(out, s)
		}
	}
	gd.escape = append(out, primary)
	return gd.escape
}

func flightClearance(foes []Seen, x, y int) int {
	d := int(^uint(0) >> 1)
	for _, s := range foes {
		d = min(d, weapon.Distance(x, y, s.X, s.Y))
	}
	return d
}

func flightPathClearance(foes []Seen, path []pathing.Point, here int) int {
	d := here
	for _, p := range path {
		d = min(d, flightClearance(foes, p.X, p.Y))
	}
	return d
}

// Keep a viable route rather than restarting it every recheck. Recheck
// immediately when the next step would enter contact with a threat.
func (gd *Guard) runFlight(id entity.EntityID, j job.Record, x, y int, primary Seen, calm bool, now uint64) bool {
	foes := gd.flightThreats(id, x, y, primary, now)
	here := flightClearance(foes, x, y)
	running := calm && j.Kind == job.Move && j.State == job.Goto && !j.Forced
	if running && j.PathI >= len(j.Path) {
		running = false
	}
	safeRoute := false
	if running {
		rest := j.Path[j.PathI:]
		next := rest[0]
		safeRoute = flightPathClearance(foes, rest, max(here, 1)) >= max(1, min(here, 2))
		if flightClearance(foes, next.X, next.Y) >= max(1, min(here, 2)) {
			if (now+uint64(id))%30 != 0 ||
				flightClearance(foes, j.TX, j.TY) >= here && flightPathClearance(foes, rest, here) >= max(1, here-1) {
				return true
			}
		}
	} else if calm && !j.Active() && (now+uint64(id))%30 != 0 {
		return true // a genuinely cornered pawn retries, without a search every tick
	}
	fx, fy, ok := gd.fleeGoal(id, x, y, primary, foes, here)
	if running && safeRoute && ok && fx == j.TX && fy == j.TY {
		return true
	}
	gd.fl.DropForFlight(id, j)
	if ok {
		gd.h.StartJob(id, job.Record{Kind: job.Move, TX: fx, TY: fy})
	}
	return true
}

type flightChoice struct {
	x, y, end, clear int
	good, home, ok   bool
}

func (c flightChoice) better(b flightChoice) bool {
	if !b.ok || c.good != b.good {
		return !b.ok || c.good
	}
	if c.good && c.home != b.home {
		return c.home
	}
	if c.clear != b.clear {
		return c.clear > b.clear
	}
	return c.end > b.end
}

// Sample the usual away/sideways directions, then the other directions
// and shorter escapes. A clamped edge, obstructed goal or dangerous route
// is not an escape. Compare actual native paths, not just endpoints.
func (gd *Guard) fleeGoal(id entity.EntityID, x, y int, threat Seen, foes []Seen, here int) (int, int, bool) {
	dx, dy := x-threat.X, y-threat.Y
	if dx == 0 && dy == 0 {
		dx = 1
	}
	dirs := [...][2]int{{dx, dy}, {dx - dy, dx + dy}, {dx + dy, dy - dx}, {-dy, dx}, {dy, -dx}, {-dx, -dy}, {dy - dx, -dx - dy}, {-dx - dy, dx - dy}}
	var best flightChoice
	var usual flightChoice
	for _, dist := range [...]int{fleeDist, fleeDist / 2, 1} {
		for i, d := range dirs {
			fx, fy := animal.FleeGoal(x, y, x-d[0], y-d[1], dist)
			fx, fy, ok := gd.tiles.WalkableNear(fx, fy, 3)
			if !ok || fx == x && fy == y || !gd.fl.Reachable(id, fx, fy) {
				continue
			}
			end := flightClearance(foes, fx, fy)
			if end < max(1, min(here, 2)) {
				continue
			}
			path, ok := gd.h.FlightPath(id, fx, fy)
			if !ok || len(path) == 0 || len(path) > 2*fleeDist || path[len(path)-1] != (pathing.Point{X: fx, Y: fy}) {
				continue
			}
			clear := flightPathClearance(foes, path, max(here, 1))
			if clear < max(1, min(here, 2)) {
				continue
			}
			c := flightChoice{x: fx, y: fy, end: end, clear: clear, good: end >= here && clear >= here-1, home: gd.fl.InHome(fx, fy), ok: true}
			// Keep the original preference for a safe forward or sideways
			// escape. Only use the extended search when that choice fails.
			if dist == fleeDist && i < 5 && c.good {
				if c.home {
					return c.x, c.y, true
				}
				if !usual.ok {
					usual = c
				}
			}
			if c.better(best) {
				best = c
			}
		}
		if usual.ok {
			return usual.x, usual.y, true
		}
		if best.good {
			break
		}
	}
	return best.x, best.y, best.ok
}
