package ebiten

import camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"

// The presentation camera lives in camera (ARCH-003). These aliases and
// one-line wrappers keep the root's call sites unchanged while its
// features move out one package at a time; each mover repoints its own
// files to camera and the last one deletes this file (SOLID_GO.md §11).

type (
	viewport  = camerapkg.Viewport
	camera    = camerapkg.Camera
	camSmooth = camerapkg.Smooth
	camLimits = camerapkg.Limits
)

// TileSize is the on-screen pixel size of one logical tile at zoom 1,
// matching the art-spec reference of 64 px/tile.
const TileSize = camerapkg.TileSize

const (
	edgeSlack   = camerapkg.EdgeSlack
	minCover    = camerapkg.MinCover
	panMaxSpeed = camerapkg.PanMaxSpeed
)

func colonyViewport(sw, sh int) viewport { return camerapkg.ColonyViewport(sw, sh) }

func colonyViewportPhysical(sw, sh int, s float64) viewport {
	return camerapkg.ColonyViewportPhysical(sw, sh, s)
}

func screenDev() float64 { return camerapkg.ScreenDev() }

func clampAxis(off, m, v0, v1, slack float64) float64 {
	return camerapkg.ClampAxis(off, m, v0, v1, slack)
}

// SyncDev adopts the screen's density (Camera.SyncDev).
func (r *Renderer) SyncDev() { r.cam.SyncDev(screenDev(), &r.smooth) }

// MinZoomFor is the furthest zoom-out for this map and viewport.
func (r *Renderer) MinZoomFor(v viewport) float64 {
	return r.cam.MinZoomFor(v, r.game.Width(), r.game.Height())
}

// ClampCamera keeps the zoom in range and the map in view.
func (r *Renderer) ClampCamera(v viewport) {
	r.SyncDev()
	r.cam.Clamp(v, r.game.Width(), r.game.Height(), r.game.World().Landing)
}

// CamLimits are the smoothing bounds for this map and viewport.
func (r *Renderer) CamLimits(v viewport) camLimits {
	return r.cam.LimitsFor(v, r.game.Width(), r.game.Height())
}
