package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/minimap"
)

// The minimap lives in render/minimap (ARCH-004 wave 4.3). These aliases
// and one-line wrappers keep the root's call sites unchanged; the last
// mover deletes this file (SOLID_GO.md §11). The renderer holds the Map by
// value (its zero value is a shown, empty minimap), so no New call is
// needed.

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

// minimapClick centres the camera on the tile under the logical point
// (sx, sy) when it is on the minimap (minimap.Map.Click returns the
// offsets; the camera, smoothing and clamp stay the renderer's).
func (r *Renderer) minimapClick(sx, sy int, ui, v camerapkg.Viewport) bool {
	ok, ox, oy := r.minimap().Click(r.minimapFrame(), r.game, sx, sy, ui, v)
	if !ok {
		return false
	}
	r.cam.OffsetX, r.cam.OffsetY = ox, oy
	r.smooth.Vx, r.smooth.Vy = 0, 0
	r.ClampCamera(v)
	return true
}

func (r *Renderer) drawMinimap(screen *ebiten.Image, ui, v camerapkg.Viewport) {
	r.minimap().Draw(r.minimapFrame(), screen, r.game, ui, v)
}
