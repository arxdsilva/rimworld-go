package ebiten

import "math"

// viewport is the part of the screen the map can be seen through: the
// screen minus the fixed top bar and the colonist bar / tab strip below.
type viewport struct{ x0, y0, x1, y1 float64 }

func (v viewport) w() float64 { return v.x1 - v.x0 }
func (v viewport) h() float64 { return v.y1 - v.y0 }

// colonyViewport is the map viewport for a sw x sh logical screen, in
// logical pixels (UI layout).
func colonyViewport(sw, sh int) viewport {
	return viewport{0, hudTopH, float64(sw), float64(hudBarY(sh))}
}

// colonyViewportPhysical is colonyViewport in physical pixels (logical x
// the UI scale s), for the renderer's culling, camera clamp and
// screenToWorld, so the map's visible area matches the scaled chrome.
func colonyViewportPhysical(sw, sh int, s float64) viewport {
	v := colonyViewport(sw, sh)
	return viewport{v.x0 * s, v.y0 * s, v.x1 * s, v.y1 * s}
}

// mapViewport is the colony map's viewport in physical pixels: what the
// camera, edge scrolling, culling and the sound director use. The
// logical size is rounded up, so the right and bottom edges are capped at
// the physical screen.
func (a *App) mapViewport() viewport {
	v := colonyViewportPhysical(a.sw, a.sh, a.scale.s)
	if a.pw > 0 {
		v.x1 = math.Min(v.x1, float64(a.pw))
	}
	if a.ph > 0 {
		v.y1 = math.Min(v.y1, float64(a.ph))
	}
	return v
}

// screenDev is the screen's device pixel density the map renders at (1
// when unset): the map's tile size is TileSize x zoom x screenDev.
func screenDev() float64 {
	if d := curUIScale.dev; d > 0 && !math.IsNaN(d) && !math.IsInf(d, 0) {
		return d
	}
	return 1
}

// syncDev adopts the screen's density, rescaling the camera's physical
// offsets and velocity so every point-space position keeps showing the
// same world point (HiDPI toggled, or the window moved to another
// screen). The zoom is apparent, so it does not change.
func (r *Renderer) syncDev() {
	d := screenDev()
	old := r.cam.dev
	if old <= 0 {
		old = 1
	}
	if d == r.cam.dev {
		return
	}
	k := d / old
	r.cam.offsetX *= k
	r.cam.offsetY *= k
	r.smooth.vx *= k
	r.smooth.vy *= k
	r.smooth.anchorX *= k
	r.smooth.anchorY *= k
	r.cam.dev = d
}

const (
	// edgeSlack is how far past the map edge the camera may scroll, as a
	// fraction of the viewport, so edge tiles can be pulled out from under
	// side panels (architect, inspector, alerts). The gap shows as black.
	edgeSlack = 0.18
	// fitSlack is the spare room around the whole map at full zoom-out.
	fitSlack = 0.08
	// minCover is the least share of the viewport's wider side the map must
	// fill at full zoom-out, so black never dominates the screen.
	minCover = 0.6
)

// minZoomFor is the furthest zoom-out for this map and viewport: the whole
// map fits with a small margin, unless that would leave more than
// 1-minCover of the screen black along its wider side.
// v and the map are in physical pixels (one tile is TileSize x dev at
// zoom 1), so the limit is the same apparent zoom at any density.
func (r *Renderer) minZoomFor(v viewport) float64 {
	unit := (&camera{zoom: 1, dev: r.cam.dev}).tile()
	mw := float64(r.game.Width()) * unit
	mh := float64(r.game.Height()) * unit
	if v.w() <= 0 || v.h() <= 0 || mw <= 0 || mh <= 0 {
		return minZoom
	}
	fx, fy := v.w()/mw, v.h()/mh
	z := math.Max(math.Min(fx, fy)*(1-fitSlack), math.Max(fx, fy)*minCover)
	return math.Max(0.1, math.Min(z, 1))
}

// clampCamera keeps the zoom in range and the map in view. Each map edge
// may be scrolled at most edgeSlack of the viewport inside it; on an axis
// where the map is smaller than that it is centred instead. The first call
// centres the camera on the map.
//
// v is the physical map viewport (App.mapViewport).
func (r *Renderer) clampCamera(v viewport) {
	r.syncDev()
	if v.w() <= 0 || v.h() <= 0 {
		return
	}
	if mz := r.minZoomFor(v); r.cam.zoom < mz {
		r.cam.zoom = mz
	}
	t := r.tilePx()
	mw, mh := float64(r.game.Width())*t, float64(r.game.Height())*t
	if !r.cam.placed {
		r.cam.placed = true
		r.cam.offsetX = mw/2 - (v.x0+v.x1)/2
		r.cam.offsetY = mh/2 - (v.y0+v.y1)/2
	}
	r.cam.offsetX = clampAxis(r.cam.offsetX, mw, v.x0, v.x1, edgeSlack*v.w())
	r.cam.offsetY = clampAxis(r.cam.offsetY, mh, v.y0, v.y1, edgeSlack*v.h())
}

// clampAxis bounds a camera offset on one axis. The map spans
// [-off, -off+m] on screen; its near edge may sit at most slack inside v0
// and its far edge at most slack inside v1.
func clampAxis(off, m, v0, v1, slack float64) float64 {
	lo := -(v0 + slack)  // near edge no further in than v0+slack
	hi := m - v1 + slack // far edge no further in than v1-slack
	if lo > hi {
		return (lo + hi) / 2
	}
	return math.Max(lo, math.Min(hi, off))
}

// camLimits are the smoothing bounds for this map and viewport.
func (r *Renderer) camLimits(v viewport) camLimits {
	return camLimits{v: v, mapW: float64(r.game.Width()), mapH: float64(r.game.Height()), minZoom: r.minZoomFor(v)}
}
