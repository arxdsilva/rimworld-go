package worldview

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// DrawSpot outlines the map cell under the pick-spot cursor (world
// drawing, in the map's pixels; the hint goes through the canvas).
func (v *View) DrawSpot(dst *ebiten.Image) {
	if !v.spot.on {
		return
	}
	mx, my := v.sh.Cursor()
	if v.sh.ChromeHit(mx, my) {
		return
	}
	tile := float32(v.sh.TilePx())
	tx, ty := v.sh.TileAt(v.sh.MapCursor())
	sx, sy := v.sh.WorldToScreen(tx, ty)
	vector.StrokeRect(dst, sx+1, sy+1, tile-2, tile-2, 2, uikit.Accent, false)
}
