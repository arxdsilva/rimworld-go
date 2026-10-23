// Package noise3 is deterministic, seeded 3D value noise and fractal
// Brownian motion (fBm), meant for sampling fields on the unit sphere. The
// lattice is a pure hash of the seed and the cell coordinates, so a field
// needs no storage, any point can be sampled in any order, and the same
// seed always gives the same values. It has no game concepts.
package noise3

import "math"

// Field is one seeded noise field. The zero Field is valid (seed 0).
type Field struct {
	seed uint64
}

// New returns the field for seed.
func New(seed uint64) Field { return Field{seed: mix(seed ^ 0x9e3779b97f4a7c15)} }

// mix is the splitmix64 finaliser: a cheap, well-distributed 64-bit hash.
func mix(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// lattice is the value in [0, 1) at integer cell (x, y, z).
func (f Field) lattice(x, y, z int64) float64 {
	h := f.seed
	h = mix(h ^ uint64(x)*0x8cb92ba72f3d8dd7)
	h = mix(h ^ uint64(y)*0xd6e8feb86659fd93)
	h = mix(h ^ uint64(z)*0xa0761d6478bd642f)
	return float64(h>>11) / (1 << 53)
}

// smooth is the cubic smoothstep fade.
func smooth(t float64) float64 { return t * t * float64(3-2*t) }

func lerp(a, b, t float64) float64 { return a + float64((b-a)*t) }

// At is the noise at (x, y, z), in [0, 1): trilinear smoothstep
// interpolation of the hashed lattice, one lattice cell per unit.
func (f Field) At(x, y, z float64) float64 {
	fx, fy, fz := math.Floor(x), math.Floor(y), math.Floor(z)
	ix, iy, iz := int64(fx), int64(fy), int64(fz)
	tx, ty, tz := smooth(x-fx), smooth(y-fy), smooth(z-fz)
	c000 := f.lattice(ix, iy, iz)
	c100 := f.lattice(ix+1, iy, iz)
	c010 := f.lattice(ix, iy+1, iz)
	c110 := f.lattice(ix+1, iy+1, iz)
	c001 := f.lattice(ix, iy, iz+1)
	c101 := f.lattice(ix+1, iy, iz+1)
	c011 := f.lattice(ix, iy+1, iz+1)
	c111 := f.lattice(ix+1, iy+1, iz+1)
	a := lerp(lerp(c000, c100, tx), lerp(c010, c110, tx), ty)
	b := lerp(lerp(c001, c101, tx), lerp(c011, c111, tx), ty)
	return lerp(a, b, tz)
}

// FBm sums octaves of the field at (x, y, z) scaled by freq, each octave
// at double the frequency and persistence times the weight of the last.
// The result is normalised to [0, 1). Octaves are offset from each other
// so they never share lattice points.
func (f Field) FBm(x, y, z float64, octaves int, freq, persistence float64) float64 {
	sum, norm, amp := 0.0, 0.0, 1.0
	for o := 0; o < octaves; o++ {
		off := float64(o) * 17.31
		sum += float64(amp * f.At(float64(x*freq)+off, float64(y*freq)-off, float64(z*freq)+float64(off*0.5)))
		norm += amp
		amp *= persistence
		freq *= 2
	}
	if norm == 0 {
		return 0
	}
	return sum / norm
}

// Ridged is ridged fBm in [0, 1]: sharp crests where the field crosses its
// middle value (mountain ridges), low elsewhere.
func (f Field) Ridged(x, y, z float64, octaves int, freq, persistence float64) float64 {
	v := 1 - math.Abs(2*f.FBm(x, y, z, octaves, freq, persistence)-1)
	return v * v
}
