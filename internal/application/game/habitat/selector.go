// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package habitat chooses reachable starting positions for wild herds.
package habitat

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
)

const (
	MinDist      = 25
	samples      = 48
	forageRadius = 3
	plantForage  = 3
)

// Cells is the map information needed to choose a herd location.
type Cells interface {
	Walkable(x, y int) bool
	DryWalkable(x, y int) bool
	Roofed(x, y int) bool
	Fogged(x, y int) bool
	HasGrass(x, y int) bool
	HasPlant(x, y int) bool
	Roll(n int) int
}

type Selector struct {
	c         Cells
	w, h      int
	lx, ly    int
	reach     []bool
	predators [][2]int
}

// New floods from the nearest walkable landing cell. The original landing
// coordinates remain the distance anchor even when the start cell moves.
func New(c Cells, w, h, lx, ly, startX, startY int, predators [][2]int) *Selector {
	s := &Selector{
		c: c, w: w, h: h, lx: lx, ly: ly, reach: make([]bool, w*h),
		predators: predators,
	}
	if startX < 0 || startX >= w || startY < 0 || startY >= h || !c.Walkable(startX, startY) {
		return s
	}
	queue := [][2]int{{startX, startY}}
	s.reach[startY*w+startX] = true
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			x, y := cell[0]+d[0], cell[1]+d[1]
			if !s.inBounds(x, y) || s.reach[y*w+x] || !c.Walkable(x, y) {
				continue
			}
			s.reach[y*w+x] = true
			queue = append(queue, [2]int{x, y})
		}
	}
	return s
}

func (s *Selector) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < s.w && y < s.h
}

func (s *Selector) open(x, y int) bool {
	return s.inBounds(x, y) && s.reach[y*s.w+x] &&
		s.c.DryWalkable(x, y) && !s.c.Roofed(x, y) && !s.c.Fogged(x, y)
}

func (s *Selector) Open(x, y int) bool { return s.open(x, y) }

func (s *Selector) forage(x, y int) int {
	n := 0
	for dy := -forageRadius; dy <= forageRadius; dy++ {
		for dx := -forageRadius; dx <= forageRadius; dx++ {
			if s.c.HasGrass(x+dx, y+dy) {
				n++
			}
			if s.inBounds(x+dx, y+dy) && s.c.HasPlant(x+dx, y+dy) {
				n += plantForage
			}
		}
	}
	return n
}

func (s *Selector) Reachable(x, y int) bool {
	return s.inBounds(x, y) && s.reach[y*s.w+x]
}

func (s *Selector) Forage(x, y int) int { return s.forage(x, y) }

func (s *Selector) PredatorCount() int { return len(s.predators) }

func (s *Selector) Pick(species animal.Species) (int, int, bool) {
	return s.pickAt(species, false)
}

func (s *Selector) PickEdge(species animal.Species) (int, int, bool) {
	return s.pickAt(species, true)
}

func (s *Selector) pickAt(species animal.Species, edge bool) (int, int, bool) {
	type candidate struct{ x, y, weight int }
	var candidates []candidate
	for i := 0; i < samples; i++ {
		x, y := s.c.Roll(s.w), s.c.Roll(s.h)
		if edge {
			switch s.c.Roll(4) {
			case 0:
				y = 0
			case 1:
				y = s.h - 1
			case 2:
				x = 0
			default:
				x = s.w - 1
			}
		}
		if weapon.Distance(x, y, s.lx, s.ly) < MinDist || !s.open(x, y) {
			continue
		}
		candidates = append(candidates, candidate{x: x, y: y})
	}
	if len(candidates) == 0 {
		return 0, 0, false
	}
	if animal.DefOf(species).Predator {
		best, bestDist := 0, -1
		for i, c := range candidates {
			dist := 1 << 30
			for _, p := range s.predators {
				dist = min(dist, weapon.Distance(c.x, c.y, p[0], p[1]))
			}
			if dist > bestDist {
				best, bestDist = i, dist
			}
		}
		chosen := candidates[best]
		s.predators = append(s.predators, [2]int{chosen.x, chosen.y})
		return chosen.x, chosen.y, true
	}
	total := 0
	for i := range candidates {
		candidates[i].weight = 1 + s.forage(candidates[i].x, candidates[i].y)
		total += candidates[i].weight
	}
	roll := s.c.Roll(total)
	for _, c := range candidates {
		if roll -= c.weight; roll < 0 {
			return c.x, c.y, true
		}
	}
	last := candidates[len(candidates)-1]
	return last.x, last.y, true
}
