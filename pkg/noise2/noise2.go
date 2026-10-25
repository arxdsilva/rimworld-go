// Package noise2 is deterministic, seeded 2-D gradient noise for map
// fields. The lattice gradients are a pure hash of the seed and the cell,
// so a field needs no storage or RNG draws: any point can be sampled in any
// order, at any map size, and the same seed always gives the same values.
// Every function returns values in [0, 1]. It has no game concepts.
package noise2

import "math"

// mix is the splitmix64 finaliser: a cheap, well-distributed 64-bit hash.
func mix(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Seed derives a field seed from a base seed and a name, so separate
// fields of one map never share a lattice.
func Seed(base uint64, name string) uint64 {
	h := uint64(0xcbf29ce484222325) // FNV-1a over the name
	for i := 0; i < len(name); i++ {
		h ^= uint64(name[i])
		h *= 0x100000001b3
	}
	return mix(base ^ mix(h))
}

// gradDirs are 16 unit gradient directions.
var gradDirs = func() (g [16][2]float64) {
	for i := range g {
		a := float64(i) * math.Pi / 8
		g[i] = [2]float64{math.Cos(a), math.Sin(a)}
	}
	return g
}()

func grad(seed uint64, ix, iy int64) [2]float64 {
	h := mix(seed ^ uint64(ix)*0x8cb92ba72f3d8dd7)
	h = mix(h ^ uint64(iy)*0xd6e8feb86659fd93)
	return gradDirs[h>>60]
}

// fade is the quintic fade curve (continuous second derivative).
func fade(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }

// raw is gradient noise at (x, y) in about [-0.71, 0.71], one lattice
// cell per unit.
func raw(seed uint64, x, y float64) float64 {
	fx, fy := math.Floor(x), math.Floor(y)
	ix, iy := int64(fx), int64(fy)
	dx, dy := x-fx, y-fy
	dot := func(cx, cy int64, ox, oy float64) float64 {
		g := grad(seed, cx, cy)
		return g[0]*ox + g[1]*oy
	}
	n00 := dot(ix, iy, dx, dy)
	n10 := dot(ix+1, iy, dx-1, dy)
	n01 := dot(ix, iy+1, dx, dy-1)
	n11 := dot(ix+1, iy+1, dx-1, dy-1)
	u, v := fade(dx), fade(dy)
	a := n00 + u*(n10-n00)
	b := n01 + u*(n11-n01)
	return a + v*(b-a)
}

// unit maps raw noise to [0, 1].
func unit(v float64) float64 {
	v = 0.5 + v*(1/math.Sqrt2)
	return min(1, max(0, v))
}

// Gradient is the noise at (x, y) with one lattice cell per cell units
// (cell <= 0 means 1), in [0, 1], 0.5 on average.
func Gradient(seed uint64, x, y, cell float64) float64 {
	if cell <= 0 {
		cell = 1
	}
	return unit(raw(seed, x/cell, y/cell))
}

// FBm sums octaves of gradient noise at (x, y): the first octave has
// lattice cells of cell units, each further octave half the cell size and
// gain times the weight. The result is in [0, 1], 0.5 on average. Octaves
// use separate lattices.
func FBm(seed uint64, x, y float64, octaves int, cell, gain float64) float64 {
	if cell <= 0 {
		cell = 1
	}
	sum, norm, amp, freq := 0.0, 0.0, 1.0, 1/cell
	for o := 0; o < octaves; o++ {
		s := mix(seed + uint64(o)*0x9e3779b97f4a7c15)
		sum += amp * raw(s, x*freq, y*freq)
		norm += amp
		amp *= gain
		freq *= 2
	}
	if norm == 0 {
		return 0.5
	}
	return unit(sum / norm)
}

// Ridged is ridged fBm in [0, 1]: sharp crests where the field crosses
// its middle value, low elsewhere.
func Ridged(seed uint64, x, y float64, octaves int, cell, gain float64) float64 {
	v := 1 - math.Abs(2*FBm(seed, x, y, octaves, cell, gain)-1)
	return v * v
}

// Warp is (x, y) displaced by up to amp units by two smooth noise fields
// of cell-sized lattice cells: sampling another field at the warped point
// bends straight contours into organic ones (domain warping).
func Warp(seed uint64, x, y, amp, cell float64) (float64, float64) {
	sx, sy := mix(seed^0x5bd1e995), mix(seed^0x27d4eb2f165667c5)
	ox := 2*FBm(sx, x, y, 2, cell, 0.5) - 1
	oy := 2*FBm(sy, x, y, 2, cell, 0.5) - 1
	return x + amp*ox, y + amp*oy
}
