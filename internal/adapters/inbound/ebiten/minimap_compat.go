package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/minimap"
)

// The minimap lives in render/minimap (ARCH-004 wave 4.3). These aliases
// and one-line wrappers keep the root's call sites unchanged; the last
// mover deletes this file (SOLID_GO.md §11). The renderer holds the Map by
// value (its zero value is a shown, empty minimap), so no New call is
// needed.

type minimapState = minimap.Map

const (
	minimapSide = minimap.Side
	minimapPoll = minimap.Poll
)

var minimapStructureClr = minimap.StructureClr

func minimapColor(cell uint8) color.RGBA { return minimap.Color(cell) }

// minimap is the renderer's minimap layer.
func (r *Renderer) minimap() *minimap.Map { return &r.mini }

// minimapFrame is the frame view the minimap reads, with the renderer's
// camera and presentation frame copied in.
func (r *Renderer) minimapFrame() *render.View {
	v := &r.view
	v.Cam, v.Present = r.cam, r.present
	return v
}

func (r *Renderer) minimapOn() bool { return r != nil && r.minimap().On(r.game) }

func (r *Renderer) toggleMinimap() {
	if r != nil {
		r.minimap().Toggle()
	}
}

func (r *Renderer) minimapBox(ui viewport) (bx, by float64, mx, my, mw, mh float64) {
	return r.minimap().Box(r.game, ui)
}

// minimapClick centres the camera on the tile under the logical point
// (sx, sy) when it is on the minimap (minimap.Map.Click returns the
// offsets; the camera, smoothing and clamp stay the renderer's).
func (r *Renderer) minimapClick(sx, sy int, ui, v viewport) bool {
	ok, ox, oy := r.minimap().Click(r.minimapFrame(), r.game, sx, sy, ui, v)
	if !ok {
		return false
	}
	r.cam.OffsetX, r.cam.OffsetY = ox, oy
	r.smooth.Vx, r.smooth.Vy = 0, 0
	r.ClampCamera(v)
	return true
}

func (r *Renderer) minimapView(v viewport, mx, my, mw, mh float64) (fx0, fy0, fx1, fy1 float64) {
	return r.minimap().View(r.minimapFrame(), r.game, v, mx, my, mw, mh)
}

func (r *Renderer) drawMinimap(screen *ebiten.Image, ui, v viewport) {
	r.minimap().Draw(r.minimapFrame(), screen, r.game, ui, v)
}
