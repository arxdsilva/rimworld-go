package rest

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
)

// Seats is where pawns sit (BED-3): the seat a hungry colonist carries
// its meal to (AtTable, moved from game/dining.go), the comfort of the
// seat under a pawn, and SitDown, free time spent resting a low comfort
// need in the best seat around.
//
// SitDown has hysteresis: a pawn looks for a seat only below SeekComfort,
// sits until its comfort reaches SitComfort or the seat's own comfort (or
// SitTicks pass), and after finding no seat waits SeatRetry before it
// looks again, so it never bounces between sitting and standing.
type Seats struct {
	h       Host
	n       *Needs
	scratch []seatSpot // AtTable's candidates, reused
}

// SitDown's thresholds and timings (original tuning: the wiki gives no
// numbers for when pawns choose to sit).
const (
	SeekComfort = 30 * needs.LevelPct
	SitComfort  = 70 * needs.LevelPct
	SitTicks    = 1500
	SeatRetry   = 2500
	// seatReach is how far (tiles) a pawn walks to a seat; tableReach how
	// far it carries a meal to a table.
	seatReach  = 30
	tableReach = 30
)

type seatSpot struct{ x, y, score int }

// IsSeat reports a piece a pawn sits on: dining chair, stool, armchair.
func IsSeat(k construction.Kind) bool {
	return k == construction.Chair || k == construction.Stool || k == construction.Armchair
}

// ComfortAt is the comfort (permille) of the seat on (x, y); ok false
// when there is none (a forbidden seat is still sat on by whoever is on
// it).
func (s *Seats) ComfortAt(x, y int) (comfort int, ok bool) {
	b, ok := s.h.Building(s.h.ObjectAt(x, y))
	if !ok || !IsSeat(b.Kind) {
		return 0, false
	}
	return construction.FurnitureOf(b.Kind, b.Stuff, qualityOf(b)).Comfort, true
}

// seatBonus is how many steps a seat's comfort is worth when choosing
// where to sit: a dining chair 6 (the old chair rule), a stool 4, an
// armchair 7.
func seatBonus(comfort int) int { return max(0, comfort/100-1) }

// AtTable is the best free place for pawn id to eat food lying at
// (fx, fy): a seat next to a table, else any free floor beside one, by
// distance less the seat's bonus; ties go to the upper-left cell. Only
// the eight best are tried for reach.
func (s *Seats) AtTable(id entity.EntityID, fx, fy int) (x, y int, ok bool) {
	spots := s.scratch[:0]
	for _, bid := range s.h.BuildingIDs() {
		b, ok := s.h.Building(bid)
		if !ok || b.Kind != construction.Table {
			continue
		}
		for _, d := range [4][2]int{{0, 1}, {1, 0}, {-1, 0}, {0, -1}} {
			x, y := b.X+d[0], b.Y+d[1]
			if !s.h.TileFree(id, x, y) {
				continue
			}
			score := pathing.Dist(fx, fy, x, y)
			if score > tableReach {
				continue
			}
			if c, seat := s.ComfortAt(x, y); seat {
				score -= seatBonus(c)
			} else if s.h.ObjectAt(x, y) != 0 {
				continue // something else stands there
			}
			spots = append(spots, seatSpot{x, y, score})
		}
	}
	s.scratch = spots
	slices.SortFunc(spots, func(a, b seatSpot) int {
		if a.score != b.score {
			return a.score - b.score
		}
		if a.y != b.y {
			return a.y - b.y
		}
		return a.x - b.x
	})
	for _, sp := range spots[:min(8, len(spots))] {
		if s.h.Reachable(id, sp.x, sp.y) {
			return sp.x, sp.y, true
		}
	}
	return 0, 0, false
}

// SitDown sends pawn id, free and with its comfort below SeekComfort, to
// the best free seat in reach (nearest, a comfortable seat worth a few
// steps more) at tick now. Never in Work hours, the time for work. False
// when it should not or found none (then it waits SeatRetry).
func (s *Seats) SitDown(id entity.EntityID, now uint64) bool {
	r := s.n.Of(id)
	if r.Comfort >= SeekComfort || now < r.SeatAt || s.h.Activity(id) == schedule.Work {
		return false
	}
	if x, y, ok := s.best(id); ok && s.h.StartJob(id, job.Record{Kind: job.SitDown, TX: x, TY: y, WorkLeft: SitTicks}) {
		s.h.ReserveTile(id, x, y) // one sitter per seat
		return true
	}
	r.SeatAt = now + SeatRetry
	s.n.m[id] = r
	return false
}

// best is the free seat pawn id sits in: within seatReach, inside its
// area, not forbidden, by distance less the seat's bonus, the first in
// building order on a tie; ok false when there is none or it is out of
// reach.
func (s *Seats) best(id entity.EntityID) (x, y int, ok bool) {
	px, py := s.h.Pos(id)
	bestScore := 1 << 30
	for _, bid := range s.h.BuildingIDs() {
		b, has := s.h.Building(bid)
		if !has || !IsSeat(b.Kind) || b.Forbidden || !s.h.AreaAllows(id, b.X, b.Y) || !s.h.TileFree(id, b.X, b.Y) {
			continue
		}
		d := pathing.Dist(px, py, b.X, b.Y)
		if d > seatReach {
			continue
		}
		if sc := d - seatBonus(construction.FurnitureOf(b.Kind, b.Stuff, qualityOf(b)).Comfort); sc < bestScore {
			x, y, bestScore = b.X, b.Y, sc
		}
	}
	return x, y, bestScore < 1<<30 && s.h.Reachable(id, x, y)
}

// sitting runs on a sitter's comfort beat (Needs.Tick): the sit ends
// when the pawn is not on a seat or its comfort has reached SitComfort or
// the seat's own comfort. The game's idle driver counts its SitTicks down.
func (s *Seats) sitting(id entity.EntityID, j job.Record, comfort int32, seat int, onSeat bool) {
	if !onSeat || comfort >= min(SitComfort, int32(seat)*1000) {
		j.State = job.Complete
		s.h.SetJob(id, j)
	}
}
