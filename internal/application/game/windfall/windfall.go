// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windfall

import (
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/pkg/mathx"
)

// Known limits (#13, recorded in the handoff): the windfalls fit only on
// the primary map, so colonists on another map get no sky song thought;
// only colonists present when the song starts are cheered; a meteor
// cluster may wall off a narrow path near home (nothing checks paths).

// side is the meteor box's side: every cell within MeteorRadius of home.
const side = 2*incident.MeteorRadius + 1

// Windfall runs the windfall incidents over a host, for one call.
type Windfall struct {
	h Host
	d Dice
}

// New is a Windfall over host h drawing from d.
func New(h Host, d Dice) Windfall { return Windfall{h: h, d: d} }

// Fits reports whether windfall id can fire at tick now. It never draws.
func (w Windfall) Fits(id incident.ID, now uint64) bool {
	if w.h.Colonists() <= 0 {
		return false
	}
	switch id {
	case incident.OreMeteor:
		_, _, ok := w.site().seed()
		return ok
	case incident.SupplyDrop:
		return len(w.dropSpots(incident.SupplyStacksMin)) >= incident.SupplyStacksMin
	case incident.SkySong:
		return w.h.SongEnds(now) == 0
	}
	return false
}

// Fire fires windfall id at tick now; false when it does not fit (nothing
// drawn, nothing changed). It checks the same precondition as Fits before
// its first draw.
func (w Windfall) Fire(id incident.ID, now uint64) bool {
	if w.h.Colonists() <= 0 {
		return false
	}
	switch id {
	case incident.OreMeteor:
		return w.meteor()
	case incident.SupplyDrop:
		return w.supply()
	case incident.SkySong:
		return w.song(now)
	}
	return false
}

// meteor raises 4-7 cells of ore-bearing rough rock at the meteor site.
func (w Windfall) meteor() bool {
	s := w.site()
	x, y, ok := s.seed()
	if !ok {
		return false
	}
	ore := incident.PickOre(w.d.IntN)
	for _, c := range s.cluster(x, y, incident.MeteorCells(w.d.IntN)) {
		w.h.Rock(c[0], c[1], ore)
	}
	w.h.Letter("Ore meteor", x, y, "A meteor has struck open ground near the colony and left a mound of rock laced with ore. Mine it to recover the ore.")
	return true
}

// supply lands 2-3 forbidden stacks of distinct kinds on the drop spots.
func (w Windfall) supply() bool {
	spots := w.dropSpots(incident.SupplyStacksMax)
	if len(spots) < incident.SupplyStacksMin {
		return false
	}
	supplies := incident.PickSupplies(w.d.IntN)
	n := min(len(supplies), len(spots))
	for i, s := range supplies[:n] {
		w.h.Drop(s.Kind, s.Qty, spots[i][0], spots[i][1])
	}
	w.h.Letter("Supply drop", spots[0][0], spots[0][1], "A battered supply pod broke apart near the colony and spilled %d stacks of goods. They stay forbidden until you allow them.", n)
	return true
}

// song starts the sky song and cheers every colonist until it ends.
func (w Windfall) song(now uint64) bool {
	if w.h.SongEnds(now) != 0 {
		return false
	}
	ticks := incident.SongTicks(w.d.IntN)
	if !w.h.StartSong(now, ticks) {
		return false
	}
	w.h.Cheer(now, now+ticks)
	hx, hy := w.h.Home()
	w.h.Letter("Sky song", hx, hy, "A soft, steady tone is ringing down from the clear sky. For about %d days the whole colony feels lighter.", incident.SongDays(ticks))
	return true
}

// dropSpots is up to k spots near home, each the first ring cell around
// home that DropFree accepts beside the spots already taken.
func (w Windfall) dropSpots(k int) [][2]int {
	hx, hy := w.h.Home()
	taken := make([][2]int, 0, k)
	for len(taken) < k {
		x, y, ok := travel.Ring(hx, hy, incident.SupplyRadius, func(x, y int) bool { return w.h.DropFree(x, y, taken) })
		if !ok {
			break
		}
		taken = append(taken, [2]int{x, y})
	}
	return taken
}

// meteorSite is one meteor site search: the band MeteorMinDist..
// MeteorRadius around home, with each cell's MeteorFree asked once.
type meteorSite struct {
	g      Ground
	hx, hy int
	memo   [side * side]int8 // 0 unasked, 1 free, -1 not
	buf    [incident.MeteorCellsMax][2]int
}

func (w Windfall) site() *meteorSite {
	s := &meteorSite{g: w.h}
	s.hx, s.hy = w.h.Home()
	return s
}

// free reports (x, y) is in the band and MeteorFree.
func (s *meteorSite) free(x, y int) bool {
	dx, dy := x-s.hx, y-s.hy
	if d := max(mathx.Abs(dx), mathx.Abs(dy)); d < incident.MeteorMinDist || d > incident.MeteorRadius {
		return false
	}
	i := (dy+incident.MeteorRadius)*side + dx + incident.MeteorRadius
	if s.memo[i] == 0 {
		s.memo[i] = -1
		if s.g.MeteorFree(x, y) {
			s.memo[i] = 1
		}
	}
	return s.memo[i] == 1
}

// seed is the first ring cell around home whose cluster reaches
// MeteorCellsMin cells (deterministic, no draw).
func (s *meteorSite) seed() (int, int, bool) {
	return travel.Ring(s.hx, s.hy, incident.MeteorRadius, func(x, y int) bool {
		return len(s.cluster(x, y, incident.MeteorCellsMin)) >= incident.MeteorCellsMin
	})
}

// cluster is up to n (at most MeteorCellsMax) free cells four-connected to
// (x, y), breadth first, each cell's neighbours north, east, south, west;
// none when (x, y) is not free. It reuses the search's buffer.
func (s *meteorSite) cluster(x, y, n int) [][2]int {
	out := s.buf[:0]
	if !s.free(x, y) {
		return out
	}
	n = min(n, len(s.buf))
	out = append(out, [2]int{x, y})
	for i := 0; i < len(out) && len(out) < n; i++ {
		for _, d := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			c := [2]int{out[i][0] + d[0], out[i][1] + d[1]}
			if len(out) < n && s.free(c[0], c[1]) && !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	return out
}
