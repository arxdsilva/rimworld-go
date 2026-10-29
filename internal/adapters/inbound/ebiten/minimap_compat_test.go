package ebiten

import (
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
)

func (r *Renderer) minimapBox(ui camerapkg.Viewport) (bx, by float64, mx, my, mw, mh float64) {
	return r.minimap().Box(r.game, ui)
}

func (r *Renderer) minimapView(v camerapkg.Viewport, mx, my, mw, mh float64) (fx0, fy0, fx1, fy1 float64) {
	return r.minimap().View(r.minimapFrame(), r.game, v, mx, my, mw, mh)
}
