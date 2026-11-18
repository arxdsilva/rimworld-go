// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package craft

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeWorkshop is the fake host's Workshop role (PRD-3): gloom takes
// light away (0: full daylight everywhere), clean is every room's
// cleanliness, outside puts every bench outdoors, blocked cells refuse
// a building, burnt the fuel ticks used, burns the worked ticks that used it. The zero value works.
type fakeWorkshop struct {
	gloom   int
	clean   int
	outside bool
	blocked map[[2]int]bool
	burnt   int // fuel ticks burnt
	burns   int // UseFuel calls: worked ticks at a burning bench
}

func (f *fakeHost) Light(x, y int) int { return 1000 - f.gloom }
func (f *fakeHost) Cleanliness(x, y int) (int, bool) {
	return f.clean, !f.outside
}

// fakeWorkBurn is the fuel ticks one worked tick burns in the fake (the
// stove's industry.FuelSpec WorkBurn: 6000 / 375).
const fakeWorkBurn = 16

func (f *fakeHost) UseFuel(id entity.EntityID) {
	f.burns++
	f.burnt += fakeWorkBurn
	f.fuel[id] = max(0, f.fuel[id]-fakeWorkBurn)
}
func (f *fakeHost) CanInstall(k construction.Kind, x, y int) bool {
	cells, _, _ := construction.Footprint(k, construction.South, x, y)
	for _, c := range cells {
		if f.blocked[c] || f.ObjectAt(c[0], c[1]) != 0 {
			return false
		}
	}
	return true
}
func (f *fakeHost) Install(k construction.Kind, stuff item.Kind, q uint8, x, y int) entity.EntityID {
	if !f.CanInstall(k, x, y) {
		return 0
	}
	f.next++
	f.build(f.next, k, x, y)
	b := f.bldgs[f.next]
	b.Stuff, b.Quality = stuff, q
	f.bldgs[f.next] = b
	return f.next
}
