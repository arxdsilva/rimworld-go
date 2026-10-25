package landform

import (
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/roof"
)

// Rocks is the "rocks" pass (MAPGEN-2 §2.5): writes final Rock,
// RockDepth and the natural Roof.
//
// Rock is the provisional mask minus caves, water and every water band.
// Cleanup, in order: one-cell spikes (≤ 1 rock 4-neighbour; off the map
// counts as rock) are trimmed twice, bodies of ≤ 5 cells are dropped
// (6+ cell outcrops stay, which keeps flat maps' small outcrops), and
// open holes of ≤ 3 cells fully inside rock are filled (never caves or
// water). Roofs use depth over rock and caves together: Natural from
// depth 2, Natural|NaturalThick from 7, nothing on cave mouths open to
// the sky. Roof is written only on rock and cave cells.
type Rocks struct{}

// Name is the step's stream and label key.
func (Rocks) Name() string { return "rocks" }

const (
	trimPasses = 2
	tinyBody   = 5 // bodies this small or smaller are dropped
	smallHole  = 3 // holes this small or smaller are filled
	thinRoof   = 2 // roof depth thresholds
	thickRoof  = 7
)

// Run is the pass.
func (Rocks) Run(c *mapgen.Ctx) error {
	f := c.F
	W, H := c.W, c.H
	rock := f.Rock
	for i := range rock {
		rock[i] = rock[i] && !f.Cave[i] && f.Water[i] == mapgen.Dry
	}
	for range trimPasses {
		trimSpikes(W, H, rock)
	}
	dropTiny(W, H, rock)
	fillHoles(W, H, rock, f.Cave, f.Water)
	copy(f.RockDepth, mapgen.ChebDepth(W, H, rock))

	solid := make([]bool, len(rock))
	for i := range solid {
		solid[i] = rock[i] || f.Cave[i]
	}
	d := mapgen.ChebDepth(W, H, solid)
	for i, s := range solid {
		if !s {
			continue
		}
		var r roof.Cell
		if d[i] >= thinRoof {
			r = roof.Natural
		}
		if d[i] >= thickRoof {
			r |= roof.NaturalThick
		}
		c.R.Roof[i] = r
	}
	return nil
}

// trimSpikes removes, at once, every rock cell with at most one rock
// 4-neighbour (off the map counts as rock).
func trimSpikes(w, h int, rock []bool) {
	var cut []int
	for i, r := range rock {
		if !r {
			continue
		}
		x, y := i%w, i/w
		n := 0
		if x == 0 || rock[i-1] {
			n++
		}
		if x == w-1 || rock[i+1] {
			n++
		}
		if y == 0 || rock[i-w] {
			n++
		}
		if y == h-1 || rock[i+w] {
			n++
		}
		if n <= 1 {
			cut = append(cut, i)
		}
	}
	for _, i := range cut {
		rock[i] = false
	}
}

// dropTiny removes rock bodies of tinyBody cells or fewer.
func dropTiny(w, h int, rock []bool) {
	label, sizes := mapgen.Components(w, h, rock, false)
	for i, k := range label {
		if k >= 0 && sizes[k] <= tinyBody {
			rock[i] = false
		}
	}
}

// fillHoles fills open pockets of smallHole cells or fewer that rock
// encloses (not touching the map edge) and that hold no cave or water.
func fillHoles(w, h int, rock, cave []bool, water []mapgen.Water) {
	open := make([]bool, len(rock))
	for i, r := range rock {
		open[i] = !r
	}
	label, sizes := mapgen.Components(w, h, open, false)
	keep := make([]bool, len(sizes))
	for i, k := range label {
		if k < 0 || keep[k] {
			continue
		}
		x, y := i%w, i/w
		if sizes[k] > smallHole || x == 0 || y == 0 || x == w-1 || y == h-1 || cave[i] || water[i] != mapgen.Dry {
			keep[k] = true
		}
	}
	for i, k := range label {
		if k >= 0 && !keep[k] {
			rock[i] = true
		}
	}
}
