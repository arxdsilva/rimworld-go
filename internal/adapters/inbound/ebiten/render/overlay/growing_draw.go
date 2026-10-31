package overlay

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/terrain"
)

// The fertility overlay (FOOD-5, MAIN_UI_SCREENS map overlays): every
// visible tile that can grow a plant is tinted by its fertility (poor
// gravel amber, soil olive, rich soil green), and the tiles of growing
// zones are marked on top, so the player sees where to sow before placing
// a zone. Presentation only; it reads WorldView.Terrain and Zones, and
// draws nothing while it is off. No allocation per frame.

// FertilityTint is the fill of a tile of fertility percent f (soil is
// 100): nothing for ground nothing grows on.
func FertilityTint(f int) color.NRGBA {
	switch {
	case f <= 0:
		return color.NRGBA{}
	case f < 100:
		return color.NRGBA{214, 168, 66, 84} // poor: gravel, sand
	case f == 100:
		return color.NRGBA{150, 170, 70, 84} // soil
	default:
		return color.NRGBA{66, 176, 84, 96} // rich
	}
}

// growZoneMark marks a growing zone's cells.
var growZoneMark = color.NRGBA{240, 240, 255, 70}

// DrawFertility draws the overlay when it is on.
func (o *Set) DrawFertility(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if !o.Fertility.Shown() || w.TerrainW <= 0 {
		return
	}
	x0, y0, x1, y1 := visibleTiles(v, sw, sh, tile)
	rows := len(w.Terrain) / w.TerrainW
	for y := y0; y <= y1 && y < rows; y++ {
		for x := x0; x <= x1 && x < w.TerrainW; x++ {
			c := FertilityTint(terrain.Type(w.Terrain[y*w.TerrainW+x]).Fertility())
			if c.A == 0 {
				continue
			}
			sx, sy := v.WorldToScreenF(float64(x), float64(y))
			o.solidRect(screen, sx, sy, float32(tile), float32(tile), c)
		}
	}
	for i := range w.Zones {
		z := &w.Zones[i]
		if z.Kind != game.ZoneGrow {
			continue
		}
		for _, c := range z.Cells {
			if c[0] < x0 || c[0] > x1 || c[1] < y0 || c[1] > y1 {
				continue
			}
			sx, sy := v.WorldToScreenF(float64(c[0]), float64(c[1]))
			o.solidRect(screen, sx, sy, float32(tile), float32(tile), growZoneMark)
		}
	}
}
