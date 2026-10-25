package mapgen

import (
	"math"
	"slices"
)

// Shared grid helpers for the steps (lead-owned). Grids are row-major w×h.

var (
	dirs4 = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	dirs8 = [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}}
)

// Components labels the connected components of the cells with in[i]
// (4-connected, or 8 with diag). label is -1 outside; components are
// numbered in row-major order of their first cell, sizes[k] counts
// component k.
func Components(w, h int, in []bool, diag bool) (label []int32, sizes []int) {
	label = make([]int32, w*h)
	for i := range label {
		label[i] = -1
	}
	ds := dirs4[:]
	if diag {
		ds = dirs8[:]
	}
	var queue []int32
	for s := range label {
		if !in[s] || label[s] >= 0 {
			continue
		}
		k := int32(len(sizes))
		label[s] = k
		queue = append(queue[:0], int32(s))
		for head := 0; head < len(queue); head++ {
			i := int(queue[head])
			x, y := i%w, i/w
			for _, d := range ds {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h {
					continue
				}
				j := ny*w + nx
				if in[j] && label[j] < 0 {
					label[j] = k
					queue = append(queue, int32(j))
				}
			}
		}
		sizes = append(sizes, len(queue))
	}
	return label, sizes
}

// ChebDepth is, for every solid cell, the Chebyshev distance to the
// nearest non-solid cell (1 = touching open ground, 8-neighbourhood),
// capped at 255; open cells are 0. Off the map counts as solid (rock runs
// on past the edge); with no open cell at all every solid cell is 255.
func ChebDepth(w, h int, solid []bool) []uint8 {
	d := make([]uint8, w*h)
	var queue []int32
	for i := range d {
		if solid[i] {
			d[i] = 255
		} else {
			queue = append(queue, int32(i))
		}
	}
	for head := 0; head < len(queue); head++ {
		i := int(queue[head])
		x, y := i%w, i/w
		for _, dd := range dirs8 {
			nx, ny := x+dd[0], y+dd[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			j := ny*w + nx
			if nd := d[i] + 1; d[j] == 255 && solid[j] && nd < 255 {
				d[j] = nd
				queue = append(queue, int32(j))
			}
		}
	}
	return d
}

// BFSDist is the step distance from the nearest src cell through cells
// with pass (nil: everything passes), 4-connected or 8 with diag; -1 where
// unreached. Sources need not pass.
func BFSDist(w, h int, src, pass []bool, diag bool) []int32 {
	d := make([]int32, w*h)
	var queue []int32
	for i := range d {
		d[i] = -1
		if src[i] {
			d[i] = 0
			queue = append(queue, int32(i))
		}
	}
	ds := dirs4[:]
	if diag {
		ds = dirs8[:]
	}
	for head := 0; head < len(queue); head++ {
		i := int(queue[head])
		x, y := i%w, i/w
		for _, dd := range ds {
			nx, ny := x+dd[0], y+dd[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			j := ny*w + nx
			if d[j] < 0 && (pass == nil || pass[j]) {
				d[j] = d[i] + 1
				queue = append(queue, int32(j))
			}
		}
	}
	return d
}

// Quantile is the value below which a share q (0..1) of vals lies.
func Quantile(vals []float32, q float64) float32 {
	if len(vals) == 0 {
		return 0
	}
	s := slices.Clone(vals)
	slices.Sort(s)
	k := int(Clamp01(q) * float64(len(s)-1))
	return s[k]
}

// RankNormalise replaces every value by its rank share in [0, 1] (ties
// broken by index, -0 equal to 0), so a threshold t keeps exactly a share
// 1-t above it. It sorts packed (order-preserving bits, index) keys: much
// faster than a comparator sort on big maps.
func RankNormalise(vals []float32) {
	n := len(vals)
	if n < 2 {
		for i := range vals {
			vals[i] = 0
		}
		return
	}
	keys := make([]uint64, n)
	for i, v := range vals {
		b := math.Float32bits(v)
		if v == 0 {
			b = 0
		}
		if b&(1<<31) != 0 { // negatives below positives
			b = ^b
		} else {
			b |= 1 << 31
		}
		keys[i] = uint64(b)<<32 | uint64(i)
	}
	slices.Sort(keys)
	for r, k := range keys {
		vals[uint32(k)] = float32(r) / float32(n-1)
	}
}

// Smoothstep is 0 below a, 1 above b and a smooth S-curve between.
func Smoothstep(a, b, x float64) float64 {
	if a == b {
		if x < a {
			return 0
		}
		return 1
	}
	t := Clamp01((x - a) / (b - a))
	return t * t * (3 - 2*t)
}

// Clamp01 clamps v to [0, 1].
func Clamp01(v float64) float64 { return min(1, max(0, v)) }
