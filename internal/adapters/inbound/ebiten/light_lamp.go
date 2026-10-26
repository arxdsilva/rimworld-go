package ebiten

import (
	"math"

	"rimworld-go/internal/domain/world/room"
)

// Point lamps (PLAY-020 §3b). A lamp lights the tiles within its radius
// by a smooth falloff: a room lamp only its own room, an outdoor lamp the
// open ground it can see (walls, doors and rock stop it; it never lights
// into an enclosed room). The light grid keeps, per tile, the brightest
// light and the direction toward the dominant lamp, packed into the one
// vertex float the lamp light already rode in (Custom1), so every batch
// that passes lightTile's value through stays unchanged and Ebiten keeps
// batching (no per-sprite uniforms, no lamp list in the shader).
//
// Packing (lit.kage's lampDecode is the twin of decodeLamp):
//
//	value = level                         no direction (campfires, fires, under a lamp)
//	value = 4·(code+1) + 1 + level        code = ix·33 + iy
//
// level is the light 0..1 and (ix, iy) the direction vector V toward the
// lamp, each axis quantised to 33 steps over ±lampDirMax. The ±1 margin on
// the value and ±0.5 on the code keep the decode exact under varying
// interpolation and GPU division error; level keeps ~1e-3 precision.
//
// Shading: lamp light × (1 + n.xy·V). A flat normal (ground, floors, the
// chunk bakes, lighting off) gets exactly level; a face toward the lamp up
// to 1+lampDirMax, a face away at least 1−lampDirMax, never black. V is 0
// under the lamp (flat light) and grows to lampDirMax with distance, as
// the lamp sinks toward the horizon of the lit point.

const (
	lampDirMax   = 0.6  // the strongest directional swing of lamp light
	lampDirSteps = 16   // quantisation steps per half axis of V
	lampDirH     = 1.0  // tiles: the lamp's height over a lit sprite, for V's strength
	lampCore     = 1.0  // tiles: full light within this distance of a lamp
	lampDirCodes = 33.0 // 2·lampDirSteps + 1
)

// lampFalloff is a lamp's light at distance d (tiles) for radius r: full
// within lampCore, then (1 − t²)² with t the share of the way out to r.
func lampFalloff(d, r float64) float32 {
	if d >= r {
		return 0
	}
	if d <= lampCore || r <= lampCore {
		return 1
	}
	t := (d - lampCore) / (r - lampCore)
	u := 1 - t*t
	return float32(u * u)
}

// packLamp is the vertex value of lamp light level (0..1) with direction
// vector (vx, vy) (screen space, |v| ≤ lampDirMax); a direction that
// quantises to zero packs as the plain level.
func packLamp(level float32, vx, vy float64) float32 {
	q := func(v float64) int {
		return min(max(int(math.Round(v/lampDirMax*lampDirSteps)), -lampDirSteps), lampDirSteps)
	}
	ix, iy := q(vx), q(vy)
	if level <= 0 || ix == 0 && iy == 0 {
		return level
	}
	code := (ix+lampDirSteps)*int(lampDirCodes) + iy + lampDirSteps
	return float32(4*(code+1)+1) + min(level, 1)
}

// decodeLamp is lit.kage's lampDecode: the light level and the direction
// vector of a packed vertex value.
func decodeLamp(v float32) (level, vx, vy float32) {
	q := float32(math.Floor(float64(v / 4)))
	if q < 1 {
		return min(max(v, 0), 1), 0, 0
	}
	c := q - 1
	ix := float32(math.Floor(float64((c + 0.5) / lampDirCodes)))
	iy := c - lampDirCodes*ix
	return min(max(v-4*q-1, 0), 1), (ix - lampDirSteps) / lampDirSteps * lampDirMax, (iy - lampDirSteps) / lampDirSteps * lampDirMax
}

// lampLevel is the light level of a packed vertex value.
func lampLevel(v float32) float32 {
	l, _, _ := decodeLamp(v)
	return l
}

// lampAt is source k's (a lamp's) light on tile (x, y): within its radius,
// in its room (a room lamp) or on open ground in its line of sight (an
// outdoor lamp). Boundary tiles (walls, doors, rock) get none of their own;
// buildChunk lights them from their neighbours.
func (g *lightGrid) lampAt(k, x, y int) float32 {
	s := &g.srcs[k]
	if x < 0 || y < 0 || x >= g.w || y >= g.h {
		return 0
	}
	dx, dy := float64(x-s.X), float64(y-s.Y)
	if math.Abs(dx) >= s.Radius || math.Abs(dy) >= s.Radius {
		return 0
	}
	d := math.Hypot(dx, dy)
	if d >= s.Radius {
		return 0
	}
	rg := g.view.RoomGrid
	if len(rg) >= g.w*g.h {
		id := rg[y*g.w+x]
		switch home := g.srcRoom[k]; {
		case id == room.Wall:
			return 0
		case home >= 0:
			if id != home {
				return 0
			}
		case id != room.Outdoors || !g.sees(rg, s.X, s.Y, x, y):
			return 0
		}
	}
	return lampFalloff(d, s.Radius)
}

// lampReaches reports whether source k (a lamp) lights tile (x, y), or, on
// a boundary tile (a creature in a doorway), one of its neighbours.
func (g *lightGrid) lampReaches(k, x, y int) bool {
	if g.lampAt(k, x, y) > 0 {
		return true
	}
	rg := g.view.RoomGrid
	if x < 0 || y < 0 || x >= g.w || y >= g.h || len(rg) < g.w*g.h || rg[y*g.w+x] != room.Wall {
		return false
	}
	for _, d := range [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		if g.lampAt(k, x+d[0], y+d[1]) > 0 {
			return true
		}
	}
	return false
}

// sees reports whether the line from tile (x0, y0) to (x1, y1) crosses no
// boundary tile strictly between them (Bresenham), and never slips
// diagonally between two boundary tiles touching at a corner.
func (g *lightGrid) sees(rg []int, x0, y0, x1, y1 int) bool {
	wall := func(x, y int) bool { return rg[y*g.w+x] == room.Wall }
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x1 < x0 {
		sx = -1
	}
	if y1 < y0 {
		sy = -1
	}
	e := dx + dy
	x, y := x0, y0
	for x != x1 || y != y1 {
		nx, ny := x, y
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			nx += sx
		}
		if e2 <= dx {
			e += dx
			ny += sy
		}
		if nx != x && ny != y && wall(nx, y) && wall(x, ny) {
			return false
		}
		x, y = nx, ny
		if (x != x1 || y != y1) && wall(x, y) {
			return false
		}
	}
	return true
}

// pack is tile (x, y)'s packed lamp value: light level, with the direction
// toward source k (the dominant lamp, lampLvl of the light) scaled by the
// lamp's share of the level, so a campfire outshining it flattens it.
func (g *lightGrid) pack(x, y int, level, lampLvl float32, k int) float32 {
	if k < 0 || lampLvl <= 0 || level <= 0 {
		return level
	}
	s := &g.srcs[k]
	hx, hy := float64(s.X-x), 0.5*float64(s.Y-y) // y foreshortened like lightDir
	h := math.Hypot(hx, hy)
	if h == 0 {
		return level
	}
	str := lampDirMax * h / math.Hypot(h, lampDirH) * float64(lampLvl/level)
	return packLamp(level, str*hx/h, str*hy/h)
}
