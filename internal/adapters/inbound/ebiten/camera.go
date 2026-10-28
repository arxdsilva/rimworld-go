package ebiten

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// viewport is the part of the screen the map can be seen through. Since
// PLAY-018 that is the whole window: the HUD floats over the map.
type viewport struct{ X0, Y0, X1, Y1 float64 }

func (v viewport) W() float64 { return v.X1 - v.X0 }
func (v viewport) H() float64 { return v.Y1 - v.Y0 }

// colonyViewport is the map viewport for a sw x sh logical screen, in
// logical pixels (UI layout): the whole screen. The camera clamps, the edge
// scroll, the zoom anchor, "jump to" and the minimap's view frame all work
// against it, so tiles can be scrolled out from under the HUD's panels.
func colonyViewport(sw, sh int) viewport {
	return viewport{0, 0, float64(sw), float64(sh)}
}

// colonyViewportPhysical is colonyViewport in physical pixels (logical x
// the UI scale s), for the renderer's culling, camera clamp and
// screenToWorld, so the map's visible area matches the scaled chrome.
func colonyViewportPhysical(sw, sh int, s float64) viewport {
	v := colonyViewport(sw, sh)
	return viewport{v.X0 * s, v.Y0 * s, v.X1 * s, v.Y1 * s}
}

// mapViewport is the colony map's viewport in map pixels (physical x the
// render scale, world_scale.go): what the camera, edge scrolling, culling
// and the sound director use. The logical size is rounded up, so the
// right and bottom edges are capped at the screen.
func (a *App) mapViewport() viewport {
	k := a.scale.WorldK()
	v := colonyViewportPhysical(a.sw, a.sh, a.mapScale())
	if a.pw > 0 {
		v.X1 = math.Min(v.X1, float64(a.pw)*k)
	}
	if a.ph > 0 {
		v.Y1 = math.Min(v.Y1, float64(a.ph)*k)
	}
	return v
}

// screenDev is the density the map renders at (1 when unset): the
// screen's device pixel density x the map's render scale. The map's tile
// size is TileSize x zoom x screenDev.
func screenDev() float64 {
	if d := gfx.CurrentScale().Dev * gfx.CurrentScale().WorldK(); d > 0 && !math.IsNaN(d) && !math.IsInf(d, 0) {
		return d
	}
	return 1
}

// SyncDev adopts the screen's density, rescaling the camera's physical
// offsets and velocity so every point-space position keeps showing the
// same world point (HiDPI toggled, or the window moved to another
// screen). The zoom is apparent, so it does not change.
func (r *Renderer) SyncDev() {
	d := screenDev()
	old := r.cam.Dev
	if old <= 0 {
		old = 1
	}
	if d == r.cam.Dev {
		return
	}
	k := d / old
	r.cam.OffsetX *= k
	r.cam.OffsetY *= k
	r.smooth.Vx *= k
	r.smooth.Vy *= k
	r.smooth.AnchorX *= k
	r.smooth.AnchorY *= k
	r.cam.Dev = d
}

const (
	// edgeSlack is how far past the map edge the camera may scroll, as a
	// fraction of the viewport, so edge tiles can be pulled out from under
	// the HUD that floats over the map (pane, command buttons, the right
	// column, the tab strip). The gap shows as black.
	edgeSlack = 0.18
	// fitSlack is the spare room around the whole map at full zoom-out.
	fitSlack = 0.08
	// minCover is the least share of the viewport's wider side the map must
	// fill at full zoom-out, so black never dominates the screen.
	minCover = 0.6
)

// MinZoomFor is the furthest zoom-out for this map and viewport: the whole
// map fits with a small margin, unless that would leave more than
// 1-minCover of the screen black along its wider side.
// v and the map are in physical pixels (one tile is TileSize x dev at
// zoom 1), so the limit is the same apparent zoom at any density.
func (r *Renderer) MinZoomFor(v viewport) float64 {
	unit := (&camera{Zoom: 1, Dev: r.cam.Dev}).Tile()
	mw := float64(r.game.Width()) * unit
	mh := float64(r.game.Height()) * unit
	if v.W() <= 0 || v.H() <= 0 || mw <= 0 || mh <= 0 {
		return minZoom
	}
	fx, fy := v.W()/mw, v.H()/mh
	z := math.Max(math.Min(fx, fy)*(1-fitSlack), math.Max(fx, fy)*minCover)
	return math.Max(0.1, math.Min(z, 1))
}

// ClampCamera keeps the zoom in range and the map in view. Each map edge
// may be scrolled at most edgeSlack of the viewport inside it; on an axis
// where the map is smaller than that it is centred instead. The first call
// centres the camera on the map.
//
// v is the physical map viewport (App.mapViewport).
func (r *Renderer) ClampCamera(v viewport) {
	r.SyncDev()
	if v.W() <= 0 || v.H() <= 0 {
		return
	}
	if mz := r.MinZoomFor(v); r.cam.Zoom < mz {
		r.cam.Zoom = mz
	}
	t := r.tilePx()
	mw, mh := float64(r.game.Width())*t, float64(r.game.Height())*t
	if !r.cam.Placed {
		r.cam.Placed = true
		cx, cy := mw/2, mh/2
		if l := r.game.World().Landing; l != [2]int{} && l != [2]int{r.game.Width() / 2, r.game.Height() / 2} {
			cx, cy = (float64(l[0])+0.5)*t, (float64(l[1])+0.5)*t // a new colony starts on its landing spot
		}
		r.cam.OffsetX = cx - (v.X0+v.X1)/2
		r.cam.OffsetY = cy - (v.Y0+v.Y1)/2
	}
	r.cam.OffsetX = clampAxis(r.cam.OffsetX, mw, v.X0, v.X1, edgeSlack*v.W())
	r.cam.OffsetY = clampAxis(r.cam.OffsetY, mh, v.Y0, v.Y1, edgeSlack*v.H())
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

// CamLimits are the smoothing bounds for this map and viewport.
func (r *Renderer) CamLimits(v viewport) camLimits {
	return camLimits{V: v, MapW: float64(r.game.Width()), MapH: float64(r.game.Height()), MinZoom: r.MinZoomFor(v)}
}

// camera is presentation-only state: it never affects authoritative
// simulation, only where/how the world is drawn.
type camera struct {
	OffsetX, OffsetY float64
	Zoom             float64
	Placed           bool // clampCamera has centred it on the map once
	// Dev is the screen's device pixel density the tile size is
	// multiplied by (PLAY-012: the map renders at physical resolution);
	// 0 means 1, as in a bake's camera (lod.go withBake). syncDev sets it.
	Dev float64
}

// Tile is the on-screen size of one Tile in physical pixels.
func (c *camera) Tile() float64 {
	if c.Dev > 0 {
		return float64(TileSize) * c.Zoom * c.Dev
	}
	return float64(TileSize) * c.Zoom
}
