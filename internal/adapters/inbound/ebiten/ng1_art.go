package ebiten

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// NG-1 art in the colony view: the hospital bed, pile sizes of the new
// items, and the cave mouths where a tunnel opens onto the ground.

// hospitalBedKind is the building kind name of the hospital bed.
const hospitalBedKind = "hospital bed"

// sizedItemKey picks the small, medium or large pile of an item with
// s/m/l art (components, industrial medicine) by the stack quantity; any
// other key is returned as is.
func sizedItemKey(key string, qty int) string {
	for _, base := range [...]string{"item/components_", "item/industrial_medicine_"} {
		if !strings.HasPrefix(key, base) {
			continue
		}
		switch {
		case qty <= 4:
			return base + "s"
		case qty <= 14:
			return base + "m"
		}
		return base + "l"
	}
	return key
}

// caveMouthFacing is the side a cave mouth opens toward when the stone
// ground at (x, y) is a cave floor (natural roof) with open ground beside
// it: the first open side in N, E, S, W order; "" when it is not a mouth.
// Open ground is an unroofed tile that is neither rock nor water.
func caveMouthFacing(w game.WorldView, x, y int) string {
	c, ok := terrainCode(w, x, y)
	if !ok || int(c) >= len(terrainNames) || terrainNames[c] != "stone" || roofCellAt(w, x, y)&roofNatural == 0 {
		return ""
	}
	for i, d := range [...][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
		nx, ny := x+d[0], y+d[1]
		nc, ok := terrainCode(w, nx, ny)
		if !ok || isRockWallCode(nc) || isWaterCode[nc] || roofed(roofCellAt(w, nx, ny)) {
			continue
		}
		return [...]string{"north", "east", "south", "west"}[i]
	}
	return ""
}

// addCaveMouths queues the cave mouths in view (stone ground under a
// natural roof that borders open ground), drawn as part of the rock face
// in the same depth order, tinted by their stone.
func (r *Renderer) addCaveMouths(w game.WorldView, draws []artDraw, screen *ebiten.Image, tile float64, sw, sh int) []artDraw {
	if w.TerrainW <= 0 || len(w.RoofCells) == 0 {
		return draws
	}
	x0, y0, x1, y1 := r.visibleTiles()
	x0, y0 = max(x0-1, 0), max(y0-1, 0)
	x1, y1 = min(x1+1, r.game.Width()-1, w.TerrainW-1), min(y1+2, r.game.Height()-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			face := caveMouthFacing(w, x, y)
			if face == "" {
				continue
			}
			key, x, y := "rock/cave_mouth_"+face, x, y
			if !r.art.Has(key) {
				continue
			}
			cs := tintCS(r.stoneTintAt(w, x, y))
			draws = append(draws, artDraw{Row: float64(y), Col: float64(x), Fn: func() {
				r.drawSpriteCS(screen, key, color.NRGBA{}, float64(x), float64(y), tile, sw, sh, cs)
			}})
		}
	}
	return draws
}
