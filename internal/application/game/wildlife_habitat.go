// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	habitatsel "rimworld-go/internal/application/game/habitat"
)

type habitat = habitatsel.Selector

const habitatMinDist = habitatsel.MinDist

// The world-map adapter for habitat selection. Predator positions and the
// landing anchor are taken from the same native owners as the old selector.
type habitatCells struct{ g *Game }

func (h habitatCells) Walkable(x, y int) bool    { return h.g.grid.Walkable(x, y) }
func (h habitatCells) DryWalkable(x, y int) bool { return h.g.dryWalkable(x, y) }
func (h habitatCells) Roofed(x, y int) bool      { return h.g.roofAt(x, y).Roofed() }
func (h habitatCells) Fogged(x, y int) bool      { return h.g.fogged(x, y) }
func (h habitatCells) HasGrass(x, y int) bool    { return h.g.hasGrass(x, y) }
func (h habitatCells) HasPlant(x, y int) bool    { return h.g.plantAt(x, y) != 0 }
func (h habitatCells) Roll(n int) int            { return h.g.rng.IntN(n) }

func (g *Game) newHabitat() *habitat {
	w, h := g.grid.Width(), g.grid.Height()
	lx, ly := g.landingSpot()
	startX, startY := lx, ly
	if !g.grid.Walkable(lx, ly) {
		x, y, ok := g.combat.Tiles.WalkableNear(lx, ly, 6)
		if !ok {
			startX, startY = -1, -1
		} else {
			startX, startY = x, y
		}
	}
	var predators [][2]int
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && !a.Tame && isPredator(a.Species) {
			if p, ok := g.positions.Get(id); ok {
				predators = append(predators, [2]int{p.X, p.Y})
			}
		}
	}
	return habitatsel.New(habitatCells{g}, w, h, lx, ly, startX, startY, predators)
}
