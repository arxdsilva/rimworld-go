package planet

import (
	"fmt"
	"math"
	"sync"
)

// mesh is the full Goldberg hex sphere for one frequency f: the vertices
// of an icosahedron subdivided f times, projected onto the unit sphere.
// Each vertex is a tile (the dual polyhedron's faces): 10f²+2 tiles, 12 of
// them pentagons (the icosahedron's corners), the rest hexagons. A mesh is
// immutable once built and shared between planets.
type mesh struct {
	f     int
	pos   [][3]float64
	neigh [][6]int32 // ordered counter-clockwise seen from outside
	deg   []uint8
}

// icoVerts and icoFaces are the regular icosahedron (unnormalised).
var (
	phi      = (1 + math.Sqrt(5)) / 2
	icoVerts = [12][3]float64{
		{-1, phi, 0}, {1, phi, 0}, {-1, -phi, 0}, {1, -phi, 0},
		{0, -1, phi}, {0, 1, phi}, {0, -1, -phi}, {0, 1, -phi},
		{phi, 0, -1}, {phi, 0, 1}, {-phi, 0, -1}, {-phi, 0, 1},
	}
	icoFaces = [20][3]int{
		{0, 11, 5}, {0, 5, 1}, {0, 1, 7}, {0, 7, 10}, {0, 10, 11},
		{1, 5, 9}, {5, 11, 4}, {11, 10, 2}, {10, 7, 6}, {7, 1, 8},
		{3, 9, 4}, {3, 4, 2}, {3, 2, 6}, {3, 6, 8}, {3, 8, 9},
		{4, 9, 5}, {2, 4, 11}, {6, 2, 10}, {8, 6, 7}, {9, 8, 1},
	}
)

var (
	meshOnce sync.Once
	meshF    *mesh
)

// sharedMesh is the cached mesh at Frequency (the Shape phase's cache: a
// planet only changes the coverage cut, never the mesh).
func sharedMesh() *mesh {
	meshOnce.Do(func() { meshF = buildMesh(Frequency) })
	return meshF
}

// buildMesh subdivides every icosahedron face into f² triangles. Points on
// the icosahedron's corners and edges are shared between faces through a
// global numbering (corners, then f-1 points per edge, then face
// interiors), so the vertex count is exactly 10f²+2.
func buildMesh(f int) *mesh {
	if f < 1 {
		panic(fmt.Sprintf("planet: frequency %d < 1", f))
	}
	n := 10*f*f + 2
	m := &mesh{f: f, pos: make([][3]float64, n), neigh: make([][6]int32, n), deg: make([]uint8, n)}
	set := make([]bool, n)
	var corners [12][3]float64
	for i, v := range icoVerts {
		corners[i] = norm64(v)
	}
	type edgeKey struct{ a, b int }
	edgeIDs := make(map[edgeKey]int, 30)
	edgePt := func(u, v, t int) int {
		k := edgeKey{min(u, v), max(u, v)}
		e, ok := edgeIDs[k]
		if !ok {
			e = len(edgeIDs)
			edgeIDs[k] = e
		}
		if u > v {
			t = f - t
		}
		return 12 + e*(f-1) + t - 1
	}
	link := func(a, b int32) {
		m.addNeigh(a, b)
		m.addNeigh(b, a)
	}
	interior := 12 + 30*(f-1)
	idx := make([]int32, (f+1)*(f+1))
	for _, fc := range icoFaces {
		a, b, c := fc[0], fc[1], fc[2]
		A, B, C := corners[a], corners[b], corners[c]
		for i := 0; i <= f; i++ {
			for j := 0; j <= f-i; j++ {
				var id int
				switch {
				case i == 0 && j == 0:
					id = a
				case i == f:
					id = b
				case j == f:
					id = c
				case j == 0:
					id = edgePt(a, b, i)
				case i == 0:
					id = edgePt(a, c, j)
				case i+j == f:
					id = edgePt(b, c, j)
				default:
					id = interior
					interior++
				}
				idx[i*(f+1)+j] = int32(id)
				if !set[id] {
					set[id] = true
					wa, wb, wc := float64(f-i-j), float64(i), float64(j)
					var p [3]float64
					for k := 0; k < 3; k++ {
						p[k] = float64(A[k]*wa) + float64(B[k]*wb) + float64(C[k]*wc)
					}
					m.pos[id] = norm64(p)
				}
			}
		}
		for i := 0; i < f; i++ {
			for j := 0; j < f-i; j++ {
				p, q, r := idx[i*(f+1)+j], idx[(i+1)*(f+1)+j], idx[i*(f+1)+j+1]
				link(p, q)
				link(q, r)
				link(r, p)
				if i+j < f-1 {
					s := idx[(i+1)*(f+1)+j+1]
					link(q, s)
					link(s, r)
				}
			}
		}
	}
	if interior != n {
		panic(fmt.Sprintf("planet: mesh numbered %d of %d vertices", interior, n))
	}
	for v := range m.pos {
		m.orderNeigh(v)
	}
	return m
}

func (m *mesh) addNeigh(a, b int32) {
	d := m.deg[a]
	for k := uint8(0); k < d; k++ {
		if m.neigh[a][k] == b {
			return
		}
	}
	if d == 6 {
		panic("planet: mesh vertex with more than 6 neighbours")
	}
	m.neigh[a][d] = b
	m.deg[a] = d + 1
}

// orderNeigh sorts v's neighbours counter-clockwise (seen from outside the
// sphere) by their angle in v's tangent plane.
func (m *mesh) orderNeigh(v int) {
	c := m.pos[v]
	u, w := tangentBasis(c)
	d := int(m.deg[v])
	var ang [6]float64
	for k := 0; k < d; k++ {
		q := sub64(m.pos[m.neigh[v][k]], c)
		ang[k] = math.Atan2(dot64(q, w), dot64(q, u))
	}
	for i := 1; i < d; i++ { // insertion sort: d ≤ 6
		for j := i; j > 0 && ang[j] < ang[j-1]; j-- {
			ang[j], ang[j-1] = ang[j-1], ang[j]
			m.neigh[v][j], m.neigh[v][j-1] = m.neigh[v][j-1], m.neigh[v][j]
		}
	}
}

// tangentBasis returns two unit vectors u, w with (u, w, c) right-handed.
func tangentBasis(c [3]float64) (u, w [3]float64) {
	axis := [3]float64{1, 0, 0}
	if math.Abs(c[0]) > 0.6 {
		axis = [3]float64{0, 1, 0}
	}
	u = norm64(cross64(axis, c))
	w = cross64(c, u)
	return u, w
}

func norm64(v [3]float64) [3]float64 {
	l := math.Sqrt(dot64(v, v))
	if l == 0 {
		return v
	}
	return [3]float64{v[0] / l, v[1] / l, v[2] / l}
}

// dot64 rounds each product so the compiler cannot fuse it into an FMA:
// results are then identical on amd64 and arm64.
func dot64(a, b [3]float64) float64 {
	return float64(a[0]*b[0]) + float64(a[1]*b[1]) + float64(a[2]*b[2])
}

func sub64(a, b [3]float64) [3]float64 { return [3]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }

func cross64(a, b [3]float64) [3]float64 {
	return [3]float64{
		float64(a[1]*b[2]) - float64(a[2]*b[1]),
		float64(a[2]*b[0]) - float64(a[0]*b[2]),
		float64(a[0]*b[1]) - float64(a[1]*b[0]),
	}
}

func f64(v [3]float32) [3]float64 { return [3]float64{float64(v[0]), float64(v[1]), float64(v[2])} }

func f32(v [3]float64) [3]float32 { return [3]float32{float32(v[0]), float32(v[1]), float32(v[2])} }
