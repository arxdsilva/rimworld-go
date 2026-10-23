package planet

import (
	"math"
)

// Coverage cap: the pole sits within ±poleLatMax of the equator so every
// coverage spans both the tropics and the cold latitudes.
const poleLatMax = 15.0

// CoverageCos is the cosine of the cap's angular radius acos(1−2c) for a
// coverage percentage (≥ 100 keeps everything).
func CoverageCos(coverage int) float64 {
	c := float64(clampInt(coverage, 1, 100)) / 100
	if c >= 1 {
		return -2
	}
	return 1 - 2*c
}

func beginShape(b *Builder) int {
	m := sharedMesh()
	p, g := b.w, &b.g
	r := b.phaseRand(saltShape)
	lat := (float64(r.IntN(3001))/100 - poleLatMax) * math.Pi / 180
	lon := float64(r.IntN(36000)) / 100 * math.Pi / 180
	pole := [3]float64{math.Cos(lat) * math.Cos(lon), math.Cos(lat) * math.Sin(lon), math.Sin(lat)}
	capCos := CoverageCos(b.p.Coverage)
	p.Pole, p.CapCos = f32(pole), float32(capCos)

	g.fullIdx = make([]int32, len(m.pos))
	g.keep = g.keep[:0]
	for v := range m.pos {
		if dot64(m.pos[v], pole) >= capCos {
			g.fullIdx[v] = int32(len(g.keep))
			g.keep = append(g.keep, int32(v))
		} else {
			g.fullIdx[v] = -1
		}
	}
	n := len(g.keep)
	p.Center = make([][3]float32, n)
	p.CornerOff = make([]uint32, n+1)
	p.NeighOff = make([]uint32, n+1)
	p.Corners = make([][3]float32, 0, n*6)
	p.Neigh = make([]int32, 0, n*6)
	p.Lat, p.Lon = make([]int16, n), make([]int16, n)
	p.radCos = make([]float32, n)
	p.Elev = make([]int16, n)
	p.Hill = make([]siteHills, n)
	p.Biome = make([]biomeID, n)
	p.Water = make([]uint8, n)
	p.Temp, p.Rain = make([]int16, n), make([]int16, n)
	p.River = make([]siteRiver, n)
	p.RiverTo = make([]int32, n)
	p.Caves = make([]bool, n)
	p.Stones = make([][3]stone, n)
	p.Settlement = make([]int32, n)
	p.CoastDist = make([]uint8, n)
	p.stepCos = 1
	return n
}

func itemShape(b *Builder, i int) {
	m, p, g := sharedMesh(), b.w, &b.g
	v := g.keep[i]
	c := m.pos[v]
	p.Center[i] = f32(c)
	p.Lat[i] = int16(math.Round(math.Asin(clampF(c[2], -1, 1)) * 1800 / math.Pi))
	p.Lon[i] = int16(math.Round(math.Atan2(c[1], c[0]) * 1800 / math.Pi))
	d := int(m.deg[v])
	rc := 1.0
	for k := 0; k < d; k++ {
		a, bb := m.pos[m.neigh[v][k]], m.pos[m.neigh[v][(k+1)%d]]
		corner := norm64([3]float64{c[0] + a[0] + bb[0], c[1] + a[1] + bb[1], c[2] + a[2] + bb[2]})
		p.Corners = append(p.Corners, f32(corner))
		rc = math.Min(rc, dot64(corner, c))
		if ni := g.fullIdx[m.neigh[v][k]]; ni >= 0 {
			p.Neigh = append(p.Neigh, ni)
			p.stepCos = math.Min(p.stepCos, dot64(a, c))
		}
	}
	p.radCos[i] = float32(rc)
	p.CornerOff[i+1] = uint32(len(p.Corners))
	p.NeighOff[i+1] = uint32(len(p.Neigh))
	p.RiverTo[i] = -1
	p.Settlement[i] = -1
}

func endShape(b *Builder) { b.w.buildBuckets() }

// Neighbours are tile i's kept neighbours (counter-clockwise).
func (p *Planet) Neighbours(i int) []int32 { return p.Neigh[p.NeighOff[i]:p.NeighOff[i+1]] }

// TileCorners is tile i's polygon (counter-clockwise from outside).
func (p *Planet) TileCorners(i int) [][3]float32 { return p.Corners[p.CornerOff[i]:p.CornerOff[i+1]] }

func clampInt(v, lo, hi int) int { return max(lo, min(hi, v)) }

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
