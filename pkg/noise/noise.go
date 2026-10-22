// Package noise is deterministic, seeded 2D value noise: a random lattice
// smoothly interpolated between its points. It has no game concepts; the
// caller supplies the RNG, so the same draws always give the same field.
package noise

import (
	"math"

	"rimworld-go/pkg/randx"
)

// latticeSteps is the resolution of each random lattice value.
const latticeSteps = 1 << 20

// Value is smooth lattice noise in [0, 1] with a fixed cell size.
type Value struct {
	cell    float64
	gw, gh  int
	lattice []float64
}

// New builds a w×h noise field with lattice spacing cell (> 0). It draws
// exactly (int(w/cell)+2)*(int(h/cell)+2) values from r, in row-major
// order, so layers built from one shared RNG stay stable.
func New(r *randx.Rand, w, h int, cell float64) Value {
	gw := int(float64(w)/cell) + 2
	gh := int(float64(h)/cell) + 2
	l := make([]float64, gw*gh)
	for i := range l {
		l[i] = float64(r.IntN(latticeSteps)) / latticeSteps
	}
	return Value{cell: cell, gw: gw, gh: gh, lattice: l}
}

// At is the noise at cell (x, y), for 0 ≤ x < w and 0 ≤ y < h.
func (n Value) At(x, y int) float64 {
	fx, fy := (float64(x)+0.5)/n.cell, (float64(y)+0.5)/n.cell
	ix, iy := int(math.Floor(fx)), int(math.Floor(fy))
	tx, ty := fx-float64(ix), fy-float64(iy)
	tx = tx * tx * (3 - 2*tx)
	ty = ty * ty * (3 - 2*ty)
	g := func(a, b int) float64 { return n.lattice[b*n.gw+a] }
	top := g(ix, iy)*(1-tx) + g(ix+1, iy)*tx
	bot := g(ix, iy+1)*(1-tx) + g(ix+1, iy+1)*tx
	return top*(1-ty) + bot*ty
}
