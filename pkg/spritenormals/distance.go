// Port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez), https://github.com/Hugo-Dz/spritefusion-sprite-normals @ d14b067. See THIRD_PARTY_NOTICES.md.

package spritenormals

import "math"

var inf32 = float32(math.Inf(1))

// squaredLine is the 1-D lower envelope of parabolas (Felzenszwalb).
// sites and starts are caller-owned scratch of capacity len(source).
func squaredLine(source, destination []float32, sites []int, starts []float64) {
	sites, starts = sites[:0], starts[:0]
	for q, height := range source {
		if math.IsInf(float64(height), 0) || height != height {
			continue
		}
		intersection := math.Inf(-1)
		for len(sites) > 0 {
			p := sites[len(sites)-1]
			qf, pf := float64(q), float64(p)
			intersection = (float64(height) + qf*qf - float64(source[p]) - pf*pf) / (2 * float64(q-p))
			if intersection > starts[len(starts)-1] {
				break
			}
			sites, starts = sites[:len(sites)-1], starts[:len(starts)-1]
		}
		if len(sites) == 0 {
			intersection = math.Inf(-1)
		}
		sites = append(sites, q)
		starts = append(starts, intersection)
	}
	if len(sites) == 0 {
		for i := range destination {
			destination[i] = inf32
		}
		return
	}
	active := 0
	for q := range destination {
		for active+1 < len(starts) && starts[active+1] < float64(q) {
			active++
		}
		delta := float32(q) - float32(sites[active])
		destination[q] = float32(math.FMA(float64(delta), float64(delta), float64(source[sites[active]])))
	}
}

// edt is the exact Euclidean distance from every pixel to the nearest
// pixel outside mask (infinite when there is none).
func edt(mask []bool, width, height int) field {
	result := newField(width, height)
	n := max(width, height)
	source, destination := make([]float32, n), make([]float32, n)
	sites, starts := make([]int, 0, n), make([]float64, 0, n)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if mask[y*width+x] {
				source[x] = inf32
			} else {
				source[x] = 0
			}
		}
		squaredLine(source[:width], destination[:width], sites, starts)
		copy(result.data[y*width:(y+1)*width], destination[:width])
	}
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			source[y] = result.data[y*width+x]
		}
		squaredLine(source[:height], destination[:height], sites, starts)
		for y := 0; y < height; y++ {
			result.data[y*width+x] = float32(math.Sqrt(float64(destination[y])))
		}
	}
	return result
}
