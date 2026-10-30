package store

import (
	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
)

// Cells is what the storage search reads of the map (moved from the game
// with the search, STO-2).
type Cells interface {
	// Stores lists the storage zones of priority p (zone.Stores).
	Stores(p zone.Priority) []zone.Store
	Zones() *zone.Map
	// First is the first stack lying loose on (x, y).
	First(x, y int) (item.Stack, bool)
	// Shelf is how many stacks lie on shelf cell (x, y) and whether one
	// of kind k still takes a load (haul.Ground.Shelf).
	Shelf(k item.Kind, x, y int) (n int, top bool)
	// TileTaken reports a reservation on (x, y).
	TileTaken(x, y int) bool
	// Blocked reports (x, y) can't take a new stack: not walkable, a
	// planned object, or (off a storage building's own zone: a shelf, a
	// hopper) a building standing there.
	Blocked(x, y int, building bool) bool
}

// Search is the storage cell search (PLAY-023, PRD-4, STO-1): the cell
// stack st goes to in the storage zones above priority floor whose kind
// and special filters take it, among the cells keep admits (nil: all).
// Priorities are tried highest first; within one it is the nearest
// (Chebyshev from nx, ny; ties lower y, then x) top-up cell, else the
// nearest free cell. Any stops at the first cell found (a yes/no
// question).
func Search(c Cells, st item.Stack, nx, ny int, keep func(x, y int) bool, floor zone.Priority, any bool) (int, int, bool) {
	s := search{c: c, st: st, nx: nx, ny: ny, keep: keep, any: any}
	for _, p := range zone.Priorities {
		if p <= floor {
			break
		}
		zs := c.Stores(p)
		for top := 1; top >= 0; top-- { // top-up cells first, then free ones
			for i := range zs {
				if z := &zs[i]; z.AllowsStack(st) && !(s.ok && z.Box.Dist(nx, ny) > s.d) && !(any && s.ok) {
					s.zone(z.ID, z.Kind, top == 1)
				}
			}
			if s.ok {
				return s.x, s.y, true
			}
		}
	}
	return 0, 0, false
}

// search is one Search: the best cell so far is the minimum of
// (Chebyshev distance, y, x).
type search struct {
	c      Cells
	st     item.Stack
	nx, ny int
	keep   func(x, y int) bool
	any    bool
	x, y   int
	d      int
	ok     bool
}

// zone visits zone id's cells (row-major), stopping below the rows that
// could still hold a better cell.
func (s *search) zone(id zone.ID, kind zone.Kind, top bool) {
	shelf := kind == zone.KindShelf
	s.c.Zones().EachCell(id, func(cx, cy int) bool {
		if s.ok && (s.any || cy-s.ny > s.d) {
			return false
		}
		if top {
			s.topUp(cx, cy, shelf, kind == zone.KindHopper)
		} else {
			s.free(cx, cy, shelf, kind.Building())
		}
		return true
	})
}

func (s *search) better(cx, cy int) bool {
	d := pathing.Dist(s.nx, s.ny, cx, cy)
	return !s.ok || d < s.d || d == s.d && (cy < s.y || cy == s.y && cx < s.x)
}

func (s *search) open(cx, cy int) bool {
	return !s.c.TileTaken(cx, cy) && (s.keep == nil || s.keep(cx, cy))
}

func (s *search) take(cx, cy int) {
	s.x, s.y, s.ok, s.d = cx, cy, true, pathing.Dist(s.nx, s.ny, cx, cy)
}

// topUp takes (cx, cy) when a stack there still takes a load of the
// kind: its first stack on the ground, any of its stacks on a shelf. A
// hopper is refilled only once down to half its stack limit (STO-2,
// original tuning), then filled up: no hauler trickles in each portion
// the dispenser took.
func (s *search) topUp(cx, cy int, shelf, hopper bool) {
	if !s.better(cx, cy) {
		return
	}
	k := s.st.Kind
	if shelf {
		if _, top := s.c.Shelf(k, cx, cy); top && s.open(cx, cy) {
			s.take(cx, cy)
		}
		return
	}
	if o, ok := s.c.First(cx, cy); ok && o.Kind == k && haul.Toppable(o) && s.open(cx, cy) && !(hopper && o.Qty*2 > k.StackLimit()) {
		s.take(cx, cy)
	}
}

// free takes (cx, cy) when it has room for one more stack and nothing
// planned or built stands in the way.
func (s *search) free(cx, cy int, shelf, building bool) {
	if !s.better(cx, cy) {
		return
	}
	if shelf {
		if n, _ := s.c.Shelf(s.st.Kind, cx, cy); n >= zone.ShelfStacks {
			return
		}
	} else if _, ok := s.c.First(cx, cy); ok {
		return
	}
	if s.c.Blocked(cx, cy, building) || !s.open(cx, cy) {
		return
	}
	s.take(cx, cy)
}

// HasHopper reports some hopper's storage on the map.
func HasHopper(c Cells) bool {
	for _, p := range zone.Priorities {
		for _, z := range c.Stores(p) {
			if z.Kind == zone.KindHopper {
				return true
			}
		}
	}
	return false
}
