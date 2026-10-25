package features

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// Placement numbers of the vents pass.
const (
	ventStartDist = 18.0 // cells from the landing spot
	ventApart     = 20.0 // cells between vents
	ventRuinGap   = 12.0 // cells from ruins
	ventRockNear  = 8    // "near a rock edge" in steps
	ventFarKeep   = 8    // 1 in ventFarKeep candidates far from rock are kept
	ventTriesEach = 400  // candidate positions per vent
)

// Vents is the "vents" pass (MAPGEN-2): writes Vents, Blocked.
type Vents struct{}

// Name is the step's stream and label key.
func (Vents) Name() string { return "vents" }

// ventCount draws how many vents the map gets.
func ventCount(c *mapgen.Ctx, rn interface{ IntN(int) int }) int {
	if c.Cfg.Site.Biome == biome.SeaIce {
		return 0
	}
	switch c.Cfg.Site.Hills {
	case site.SmallHills, site.LargeHills, site.Mountainous:
		return 1 + rn.IntN(3) // 1..3
	}
	return rn.IntN(3) // 0..2
}

// Run is the pass.
func (Vents) Run(c *mapgen.Ctx) error {
	rn := c.Rand("vents")
	n := ventCount(c, rn)
	if n == 0 || c.W < 4 || c.H < 4 {
		return nil
	}
	// Steps to the nearest rock cell (-1 when the map has none).
	var near []int32
	for _, r := range c.F.Rock {
		if r {
			near = mapgen.BFSDist(c.W, c.H, c.F.Rock, nil, false)
			break
		}
	}
	st := c.Start()
	startR := rect{st.X, st.Y, st.X, st.Y}
	for k := 0; k < n; k++ {
		ruins, vents := existing(c)
		for t := 0; t < ventTriesEach; t++ {
			x, y := rn.IntN(c.W-1), rn.IntN(c.H-1)
			b := rect{x, y, x + 1, y + 1}
			if !(openGround(c, x, y) && openGround(c, x+1, y) && openGround(c, x, y+1) && openGround(c, x+1, y+1)) {
				continue
			}
			if b.gap(startR) < ventStartDist || !farFrom(b, vents, ventApart) || !farFrom(b, ruins, ventRuinGap) {
				continue
			}
			if near != nil && !nearRock(near, c.W, b) && rn.IntN(ventFarKeep) != 0 {
				continue
			}
			c.R.Vents = append(c.R.Vents, mapgen.Point{X: x, Y: y})
			for _, p := range [4][2]int{{x, y}, {x + 1, y}, {x, y + 1}, {x + 1, y + 1}} {
				c.F.Blocked[c.Idx(p[0], p[1])] = true
			}
			break
		}
	}
	return nil
}

// nearRock reports any footprint cell within ventRockNear steps of rock.
func nearRock(dist []int32, w int, b rect) bool {
	for y := b.y0; y <= b.y1; y++ {
		for x := b.x0; x <= b.x1; x++ {
			if d := dist[y*w+x]; d >= 0 && d <= ventRockNear {
				return true
			}
		}
	}
	return false
}
