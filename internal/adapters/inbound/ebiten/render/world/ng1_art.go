package world

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// NG-1 art in the colony view: the hospital bed, pile sizes of the new
// items, and the cave mouths where a tunnel opens onto the ground.

// HospitalBedKind is the building kind name of the hospital bed.
const HospitalBedKind = "hospital bed"

// SizedItemKey picks the small, medium or large pile of an item with
// s/m/l art (components, industrial medicine) by the stack quantity; any
// other key is returned as is.
func SizedItemKey(key string, qty int) string {
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

// CaveMouthFacing is the side a cave mouth opens toward when the stone
// ground at (x, y) is a cave floor (natural roof) with open ground beside
// it: the first open side in N, E, S, W order; "" when it is not a mouth.
// Open ground is an unroofed tile that is neither rock nor water.
func CaveMouthFacing(w game.WorldView, x, y int) string {
	c, ok := env.TerrainCode(w, x, y)
	if !ok || int(c) >= len(TerrainNames) || TerrainNames[c] != "stone" || overlay.RoofCellAt(w, x, y)&overlay.RoofNatural == 0 {
		return ""
	}
	for i, d := range [...][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
		nx, ny := x+d[0], y+d[1]
		nc, ok := env.TerrainCode(w, nx, ny)
		if !ok || sprite.IsRockWallCode(nc) || IsWaterCode[nc] || overlay.Roofed(overlay.RoofCellAt(w, nx, ny)) {
			continue
		}
		return [...]string{"north", "east", "south", "west"}[i]
	}
	return ""
}

// addCaveMouths queues the cave mouths in view (stone ground under a
// natural roof that borders open ground), drawn as part of the rock face
// in the same depth order, tinted by their stone.
func (p *Pipeline) addCaveMouths(w game.WorldView, draws []ArtDraw, screen *ebiten.Image, tile float64, sw, sh int) []ArtDraw {
	if w.TerrainW <= 0 || len(w.RoofCells) == 0 {
		return draws
	}
	x0, y0, x1, y1 := p.visibleTiles()
	x0, y0 = max(x0-1, 0), max(y0-1, 0)
	x1, y1 = min(x1+1, p.v.MapW-1, w.TerrainW-1), min(y1+2, p.v.MapH-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			face := CaveMouthFacing(w, x, y)
			if face == "" {
				continue
			}
			key, x, y := "rock/cave_mouth_"+face, x, y
			if !p.art().Has(key) {
				continue
			}
			cs := sprite.TintCS(p.stoneTintAt(&w, x, y))
			draws = append(draws, ArtDraw{Row: float64(y), Col: float64(x), Fn: func() {
				p.drawSpriteCS(screen, key, color.NRGBA{}, float64(x), float64(y), tile, sw, sh, cs)
			}})
		}
	}
	return draws
}
