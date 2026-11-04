package recreate

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Choosing a recreation piece (original tuning: the wiki gives no
// weighting). A colonist walks up to Reach tiles; each percent of the
// joy kind's tolerance counts as a tile farther (the variety rule: the
// kind it has had least lately wins), a bored kind is never sought, and
// joining a game already being played is worth JoinBonus tiles.
const (
	Reach      = 30
	JoinBonus  = 5
	RetryTicks = clock.TicksPerHour
	// SeatBonus is how many steps a seat is worth when choosing where to
	// play or watch from.
	SeatBonus = 6
	// A telescope is used only at night: a colonist starts stargazing at
	// NightStart daylight or darker and stops when it passes NightEnd
	// (permille; the gap keeps dusk from flapping).
	NightStart = 250
	NightEnd   = 400
	maxPieces  = 4 // pieces tried for a free cell, best first
	maxCells   = 4 // cells tried for reach, best first
	// A colonist craves the outdoors below CravingOn percent of its need and
	// until it passes CravingOff (ours; the gap keeps it from flapping,
	// CSE-001).
	CravingOn  = 40
	CravingOff = 60
)

type pick struct {
	bid   entity.EntityID
	score int
}

// Seek sends colonist id, free for linger ticks, to the best recreation
// piece in reach. False when there is none it may use, none has a free
// cell, or it is waiting out a cut-short session.
func (s *State) Seek(id entity.EntityID, linger int) bool {
	h := s.h
	now := h.Now()
	p := s.pawn(id, now)
	if now < p.Retry {
		return false
	}
	px, py := h.Pos(id)
	craving := s.craves(id)
	var best, outside [maxPieces]pick
	n, out := 0, 0
	for _, bid := range h.BuildingIDs() {
		b, ok := h.Building(bid)
		sp, rec := SpecOf(b.Kind)
		if !ok || !rec || b.Forbidden || b.Ruin || b.Ancient {
			continue
		}
		d := pathing.Dist(px, py, b.X, b.Y)
		if d > Reach || p.Tol.IsBored(sp.Joy) || !h.AreaAllows(id, b.X, b.Y) || !s.usable(id, bid, b, sp, false) {
			continue
		}
		users := usersOf(h, bid)
		if users >= sp.Users {
			continue
		}
		score := d + p.Tol.Pct(sp.Joy)
		if users > 0 && sp.Users > 1 && sp.Mode == Play {
			score -= JoinBonus
		}
		n = insertPick(&best, n, pick{bid, score})
		if craving && h.Outside(b.X, b.Y) {
			out = insertPick(&outside, out, pick{bid, score})
		}
	}
	for _, c := range outside[:out] { // CSE-001: out in the open first
		if b, ok := h.Building(c.bid); ok && s.start(id, c.bid, b, linger) {
			return true
		}
	}
	for _, c := range best[:n] {
		if tried(outside[:out], c.bid) {
			continue
		}
		if b, ok := h.Building(c.bid); ok && s.start(id, c.bid, b, linger) {
			return true
		}
	}
	return false
}

// tried reports bid is among picks.
func tried(picks []pick, bid entity.EntityID) bool {
	for _, p := range picks {
		if p.bid == bid {
			return true
		}
	}
	return false
}

// craves reads id's outdoors need into its craving: on below CravingOn, off
// from CravingOff, and held between. A pawn on bed rest keeps its craving but
// stays by it.
func (s *State) craves(id entity.EntityID) bool {
	h := s.h
	if !h.OutdoorsJoy() {
		return false
	}
	i, on := slices.BinarySearch(s.s.Craving, id)
	switch pct := h.OutdoorsPct(id); {
	case !on && pct < CravingOn:
		s.s.Craving, on = slices.Insert(s.s.Craving, i, id), true
	case on && pct >= CravingOff:
		s.s.Craving, on = slices.Delete(s.s.Craving, i, i+1), false
	}
	return on && !h.OnBedRest(id)
}

// insertPick keeps the maxPieces best picks, lowest score first (ties:
// the earlier building).
func insertPick(best *[maxPieces]pick, n int, c pick) int {
	i := n
	for i > 0 && best[i-1].score > c.score {
		i--
	}
	if i >= maxPieces {
		return n
	}
	if n < maxPieces {
		n++
	}
	copy(best[i+1:n], best[i:n-1])
	best[i] = c
	return n
}

// usable reports colonist id may enjoy piece b (id bid) now: a screen
// needs power, a telescope an open sky over it and night (keep: one
// already looking holds on till NightEnd), and both need sight.
func (s *State) usable(id, bid entity.EntityID, b construction.Building, sp Spec, keep bool) bool {
	h := s.h
	switch sp.Mode {
	case Watch:
		return h.Powered(bid) && h.Sight(id) > 0
	case Gaze:
		dark := NightStart
		if keep {
			dark = NightEnd
		}
		roofed, _ := h.Roof(b.X, b.Y)
		return !roofed && h.Sky() <= dark && h.Sight(id) > 0
	case Admire:
		return h.Sight(id) > 0 && displays(h, b)
	}
	return true
}

// displays reports piece b has something to admire: a sculpture always, a
// display case or pedestal only while a stack lies on its cell.
func displays(h Displays, b construction.Building) bool {
	if b.Kind != construction.DisplayCase && b.Kind != construction.RoundPedestal {
		return true
	}
	fp, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range fp {
		if h.Holds(c[0], c[1]) {
			return true
		}
	}
	return false
}

// cells are where a colonist enjoys piece b from.
func cells(b construction.Building, sp Spec) [][2]int {
	switch sp.Mode {
	case Toss:
		out := make([][2]int, len(tossSpots))
		for i, d := range tossSpots {
			out[i] = [2]int{b.X + d[0], b.Y + d[1]}
		}
		return out
	case Watch:
		return construction.WatchCells(b.Kind, b.Facing, b.X, b.Y)
	case Gaze:
		if _, spot, ok := construction.Footprint(b.Kind, b.Facing, b.X, b.Y); ok {
			return [][2]int{spot}
		}
		return nil
	}
	return construction.RingCells(b.Kind, b.Facing, b.X, b.Y)
}

// isSeat reports a piece a colonist sits on (rest.IsSeat: chair, stool,
// armchair, couch).
func isSeat(k construction.Kind) bool { return construction.Seats(k) > 0 && !k.IsTable() }

// start gives colonist id a session at piece b (id bid) from its best
// free cell: nearest, a seat worth SeatBonus steps for games and
// screens; a thrower or stargazer stands on bare ground.
func (s *State) start(id, bid entity.EntityID, b construction.Building, linger int) bool {
	h := s.h
	sp, _ := SpecOf(b.Kind)
	px, py := h.Pos(id)
	var best [maxCells]pick
	cs := cells(b, sp)
	n := 0
	for i, c := range cs {
		x, y := c[0], c[1]
		if !h.TileFree(id, x, y) || !h.AreaAllows(id, x, y) {
			continue
		}
		score := pathing.Dist(px, py, x, y)
		if o := h.ObjectAt(x, y); o != 0 {
			ob, _ := h.Building(o)
			if !isSeat(ob.Kind) || ob.Forbidden || sp.Mode == Toss || sp.Mode == Gaze || sp.Mode == Admire {
				continue
			}
			score -= SeatBonus
		}
		n = insertPick(&best, n, pick{entity.EntityID(i), score})
	}
	for _, c := range best[:n] {
		x, y := cs[c.bid][0], cs[c.bid][1]
		if !h.Reachable(id, x, y) {
			continue
		}
		dx, dy := faceCell(b, x, y)
		if h.StartJob(id, job.Record{Kind: JobOf(b.Kind, sp.Mode), TX: x, TY: y, Dest: bid, DestX: dx, DestY: dy, WorkLeft: linger}) {
			h.ReserveTile(id, x, y) // one colonist per cell
			return true
		}
	}
	return false
}

// faceCell is the cell of piece b a colonist on (x, y) faces: the
// footprint cell nearest it.
func faceCell(b construction.Building, x, y int) (int, int) {
	fp, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	bx, by, bd := b.X, b.Y, 1<<30
	for _, c := range fp {
		if d := pathing.Dist(x, y, c[0], c[1]); d < bd {
			bx, by, bd = c[0], c[1], d
		}
	}
	return bx, by
}

// usersOf is how many colonists are on their way to or enjoying piece bid.
func usersOf(h Host, bid entity.EntityID) int {
	n := 0
	for _, pid := range h.PawnIDs() {
		if j := h.Job(pid); j.Dest == bid && Uses(j.Kind) && j.Active() {
			n++
		}
	}
	return n
}
