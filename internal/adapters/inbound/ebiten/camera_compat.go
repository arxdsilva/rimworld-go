package ebiten

import camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"

// The presentation camera lives in camera (ARCH-003). These aliases and
// one-line wrappers keep the root's call sites unchanged while its
// features move out one package at a time; each mover repoints its own
// files to camera and the last one deletes this file (SOLID_GO.md §11).

// SyncDev adopts the screen's density (Camera.SyncDev).
func (r *Renderer) SyncDev() { r.cam.SyncDev(camerapkg.ScreenDev(), &r.smooth) }

// MinZoomFor is the furthest zoom-out for this map and viewport.
func (r *Renderer) MinZoomFor(v camerapkg.Viewport) float64 {
	return r.cam.MinZoomFor(v, r.game.Width(), r.game.Height())
}

// ClampCamera keeps the zoom in range and the map in view.
func (r *Renderer) ClampCamera(v camerapkg.Viewport) {
	r.SyncDev()
	r.cam.Clamp(v, r.game.Width(), r.game.Height(), r.game.World().Landing)
}

// CamLimits are the smoothing bounds for this map and viewport.
func (r *Renderer) CamLimits(v camerapkg.Viewport) camerapkg.Limits {
	return r.cam.LimitsFor(v, r.game.Width(), r.game.Height())
}
