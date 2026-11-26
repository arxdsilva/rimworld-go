// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windfall

import (
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/geology"
)

// Windfall fires the 25.2 helpful incidents against its host.
type Windfall struct {
	h Host
	d Dice
}

// New is a Windfall on host h rolling on d.
func New(h Host, d Dice) Windfall { return Windfall{h: h, d: d} }

// Fits reports whether incident id can fire at tick now. It never rolls.
func (w Windfall) Fits(id incident.ID, now uint64) bool {
	switch id {
	case incident.EscapePodCrash, incident.OreMeteor, incident.SupplyDrop:
		return w.h.Colonists() > 0
	case incident.SkySong:
		return w.h.Colonists() > 0 && !w.h.SongOn(now)
	}
	return false
}

// Fire makes incident id happen at tick now; false when it could not.
func (w Windfall) Fire(id incident.ID, now uint64) bool {
	switch id {
	case incident.EscapePodCrash:
		return w.crash()
	case incident.SupplyDrop:
		return w.supply()
	case incident.OreMeteor:
		return w.meteor()
	case incident.SkySong:
		return w.song(now)
	}
	return false
}

func (w Windfall) crash() bool {
	hx, hy := w.h.Home()
	x, y, ok := w.h.PodSpot(hx, hy, nil)
	if !ok {
		return false
	}
	id, ok := w.h.Crash(x, y, incident.CrashHurt)
	if !ok {
		return false
	}
	w.h.Letter("Escape pod crash", x, y, id, "An escape pod has come down hard near the colony. %s, the stranger inside, is badly hurt. Carry them to a bed and tend them, and once they recover they may ask to join.", w.h.Name(id))
	return true
}

func (w Windfall) supply() bool {
	hx, hy := w.h.Home()
	n := incident.SupplyStacksMin + w.d.IntN(incident.SupplyStacksMax-incident.SupplyStacksMin+1)
	rest := make([]incident.Supply, len(incident.SupplyLoot))
	copy(rest, incident.SupplyLoot[:])
	var taken [][2]int
	x0, y0 := 0, 0
	for i := 0; i < n && len(rest) > 0; i++ {
		j := w.d.IntN(len(rest))
		s := rest[j]
		rest = append(rest[:j], rest[j+1:]...)
		qty := s.Min + w.d.IntN(s.Max-s.Min+1)
		x, y, ok := w.h.PodSpot(hx, hy, taken)
		if !ok {
			break
		}
		if len(taken) == 0 {
			x0, y0 = x, y
		}
		taken = append(taken, [2]int{x, y})
		w.h.Drop(s.Kind, qty, x, y)
	}
	if len(taken) == 0 {
		return false
	}
	w.h.Letter("Supply drop", x0, y0, 0, "A sealed supply pod has broken open near the colony and scattered %d stacks of goods. They lie forbidden until you allow them.", len(taken))
	return true
}

func (w Windfall) meteor() bool {
	ore := w.pickOre()
	n := incident.MeteorCellsMin + w.d.IntN(incident.MeteorCellsMax-incident.MeteorCellsMin+1)
	hx, hy := w.h.Home()
	span := 2*incident.MeteorRadius + 1
	for t := 0; t < incident.MeteorTries; t++ {
		cx := hx + w.d.IntN(span) - incident.MeteorRadius
		cy := hy + w.d.IntN(span) - incident.MeteorRadius
		if !w.h.MeteorFree(cx, cy) {
			continue
		}
		cells := w.grow(cx, cy, n)
		if len(cells) < incident.MeteorCellsMin {
			continue
		}
		for _, c := range cells {
			w.h.Rock(c[0], c[1], ore)
		}
		w.h.Letter("Ore meteor", cx, cy, 0, "A meteor has fallen on open ground near the colony and left a lump of ore-bearing rock. Mark it for mining to dig the ore out.")
		return true
	}
	return false
}

// pickOre draws an ore by weight.
func (w Windfall) pickOre() geology.Ore {
	total := 0
	for _, o := range incident.MeteorOres {
		total += o.Weight
	}
	r := w.d.IntN(total)
	for _, o := range incident.MeteorOres {
		if r < o.Weight {
			return o.Ore
		}
		r -= o.Weight
	}
	return incident.MeteorOres[0].Ore
}

// grow is a breadth-first 4-connected cluster of free cells from (cx, cy),
// neighbours N, E, S, W, of at most n cells.
func (w Windfall) grow(cx, cy, n int) [][2]int {
	cells := [][2]int{{cx, cy}}
	seen := map[[2]int]bool{{cx, cy}: true}
	dirs := [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}
	for i := 0; i < len(cells) && len(cells) < n; i++ {
		for _, d := range dirs {
			c := [2]int{cells[i][0] + d[0], cells[i][1] + d[1]}
			if seen[c] {
				continue
			}
			seen[c] = true
			if len(cells) < n && w.h.MeteorFree(c[0], c[1]) {
				cells = append(cells, c)
			}
		}
	}
	return cells
}

func (w Windfall) song(now uint64) bool {
	hx, hy := w.h.Home()
	r := w.d.IntN(incident.SongSpan() + 1)
	ticks := incident.SongTicks(r)
	if !w.h.StartSong(now, ticks) {
		return false
	}
	days := int((ticks + clock.TicksPerDay/2) / clock.TicksPerDay)
	if days < 1 {
		days = 1
	}
	w.h.Letter("Sky song", hx, hy, 0, "A low, warm hum is rolling down from the open sky and settling over the colony. For about %d days everyone will feel a little lighter.", days)
	return true
}
