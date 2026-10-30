// Package camera is the presentation camera: offset, zoom, device density,
// eased pan and zoom, viewports and clamping. It never touches the
// simulation.
package camera

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// Viewport is the part of the screen the map can be seen through. Since
// PLAY-018 that is the whole window: the HUD floats over the map.
type Viewport struct{ X0, Y0, X1, Y1 float64 }

func (v Viewport) W() float64 { return v.X1 - v.X0 }

func (v Viewport) H() float64 { return v.Y1 - v.Y0 }

// ColonyViewport is the map Viewport for a sw x sh logical screen, in
// logical pixels (UI layout): the whole screen. The camera clamps, the edge
// scroll, the zoom anchor, "jump to" and the minimap's view frame all work
// against it, so tiles can be scrolled out from under the HUD's panels.
func ColonyViewport(sw, sh int) Viewport {
	return Viewport{0, 0, float64(sw), float64(sh)}
}

// ColonyViewportPhysical is ColonyViewport in physical pixels (logical x
// the UI scale s), for the renderer's culling, camera clamp and
// screenToWorld, so the map's visible area matches the scaled chrome.
func ColonyViewportPhysical(sw, sh int, s float64) Viewport {
	v := ColonyViewport(sw, sh)
	return Viewport{v.X0 * s, v.Y0 * s, v.X1 * s, v.Y1 * s}
}

// ScreenDev is the density the map renders at (1 when unset): the
// screen's device pixel density x the map's render scale. The map's tile
// size is TileSize x zoom x ScreenDev.
func ScreenDev() float64 {
	if d := gfx.CurrentScale().Dev * gfx.CurrentScale().WorldK(); d > 0 && !math.IsNaN(d) && !math.IsInf(d, 0) {
		return d
	}
	return 1
}

const (
	// EdgeSlack is how far past the map edge the camera may scroll, as a
	// fraction of the Viewport, so edge tiles can be pulled out from under
	// the HUD that floats over the map (pane, command buttons, the right
	// column, the tab strip). The gap shows as black.
	EdgeSlack = 0.18
	// fitSlack is the spare room around the whole map at full zoom-out.
	fitSlack = 0.08
	// MinCover is the least share of the Viewport's wider side the map must
	// fill at full zoom-out, so black never dominates the screen.
	MinCover = 0.6
)

// ClampAxis bounds a camera offset on one axis. The map spans
// [-off, -off+m] on screen; its near edge may sit at most slack inside v0
// and its far edge at most slack inside v1.
func ClampAxis(off, m, v0, v1, slack float64) float64 {
	lo := -(v0 + slack)  // near edge no further in than v0+slack
	hi := m - v1 + slack // far edge no further in than v1-slack
	if lo > hi {
		return (lo + hi) / 2
	}
	return math.Max(lo, math.Min(hi, off))
}

// Camera is presentation-only state: it never affects authoritative
// simulation, only where/how the world is drawn.
type Camera struct {
	OffsetX, OffsetY float64
	Zoom             float64
	Placed           bool // Clamp has centred it on the map once
	// Dev is the screen's device pixel density the tile size is
	// multiplied by (PLAY-012: the map renders at physical resolution);
	// 0 means 1, as in a bake's camera (lod.go withBake). SyncDev sets it.
	Dev float64
	// pend is a view Resume queued for the next Clamp (a loaded save's).
	pend    View
	hasPend bool
}

// View is where a camera looks, independent of pixel size, screen density
// and window size: the world point at the middle of the viewport in tiles,
// and the apparent zoom. It is what a save remembers (VIEW-001).
type View struct{ CX, CY, Zoom float64 }

// View is the world point at the middle of v, in tiles, with the zoom.
func (c Camera) View(v Viewport) View {
	t := c.Tile()
	if t <= 0 {
		return View{}
	}
	return View{
		CX:   (c.OffsetX + (v.X0+v.X1)/2) / t,
		CY:   (c.OffsetY + (v.Y0+v.Y1)/2) / t,
		Zoom: c.Zoom,
	}
}

// Valid reports the view is finite with a positive zoom.
func (w View) Valid() bool {
	ok := func(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }
	return ok(w.CX) && ok(w.CY) && ok(w.Zoom) && w.Zoom > 0
}

// Resume queues w for the next Clamp, which places the camera there
// (clamped to the zoom limits and the map) instead of centring it. An
// invalid view is ignored, leaving the usual centring.
func (c *Camera) Resume(w View) {
	if !w.Valid() {
		return
	}
	c.pend, c.hasPend, c.Placed = w, true, false
}

// Tile is the on-screen size of one Tile in physical pixels.
func (c *Camera) Tile() float64 {
	if c.Dev > 0 {
		return float64(TileSize) * c.Zoom * c.Dev
	}
	return float64(TileSize) * c.Zoom
}

const (
	// TileSize is the on-screen pixel size of one logical tile at zoom 1,
	// matching the art-spec reference of 64 px/tile.
	TileSize = 64
	// PanPixels is the pan distance of one 60 fps frame at normal speed.
	PanPixels = 8
	// MinZoom is the absolute zoom-out floor; the map-fit limit is MinZoomFor.
	MinZoom = 0.1
	// MaxZoom is the furthest zoom-in.
	MaxZoom = 3.0
)

const (
	// PanMaxSpeed is the pan speed cap in px/s.
	PanMaxSpeed = PanPixels * 60.0 // px/s, matches the old 8 px per 60 fps frame
	panTau      = 0.045            // s; velocity time constant (stop in ~200 ms)
	zoomTau     = 0.05             // s; zoom ease time constant (~150 ms to settle)
	zoomSnapEps = 0.0005           // relative gap at which zoom snaps to target
)

// Limits are the bounds step applies, taken from the clamp helpers.
type Limits struct {
	V          Viewport
	MapW, MapH float64 // map size in tiles
	MinZoom    float64
}

// Smooth is presentation-only camera motion: pan velocity with
// acceleration and critically damped friction, and eased zoom anchored at a
// screen point. It never touches domain state. With instant set (capture
// mode) pan and zoom apply at once and deterministically.
type Smooth struct {
	Vx, Vy       float64 // pan velocity, physical px/s
	DirX, DirY   float64 // pan input pushed this frame, consumed by step
	TargetZoom   float64
	Zooming      bool
	PendingScale float64 // product of zoomAt factors not yet folded into targetZoom
	AnchorX      float64 // screen point kept fixed while zooming
	AnchorY      float64
	Instant      bool
	Speed        float64 // pan speed multiplier (Options); 0 means 1
	Boost        float64 // fast-pan multiplier this frame (Shift); 0 means 1
}

// Halt stops all camera motion: pan velocity and input, and a zoom ease in
// flight (the zoom stays where it is). A jump to a point calls it so the
// camera does not drift off the target afterwards. The pan-speed setting
// is kept.
func (s *Smooth) Halt() {
	s.Vx, s.Vy, s.DirX, s.DirY = 0, 0, 0, 0
	s.Zooming, s.PendingScale = false, 0
}

// Idle reports nothing is in motion: no pan velocity or input and no zoom
// ease in flight. A Draw-time step (stepCamera) skips an Idle smoother, so
// a camera that code placed directly stays exactly there until input moves
// it; Update's applyCamera still clamps every frame.
func (s *Smooth) Idle() bool {
	return s.Vx == 0 && s.Vy == 0 && s.DirX == 0 && s.DirY == 0 && !s.Zooming &&
		(s.PendingScale == 0 || s.PendingScale == 1)
}

// PushPan records held-pan input for the next step; dx, dy are directions
// (-1, 0, 1), summed and clamped to unit range per axis.
func (s *Smooth) PushPan(dx, dy float64) {
	s.DirX = math.Max(-1, math.Min(1, s.DirX+dx))
	s.DirY = math.Max(-1, math.Min(1, s.DirY+dy))
}

// ZoomAt requests a zoom change by factor, anchored at physical screen
// point (sx, sy).
func (s *Smooth) ZoomAt(factor, sx, sy float64) {
	if factor <= 0 {
		return
	}
	if s.PendingScale == 0 {
		s.PendingScale = 1
	}
	s.PendingScale *= factor
	s.AnchorX, s.AnchorY = sx, sy
}

// Step advances the camera by dt seconds and applies the bounds.
func (s *Smooth) Step(dt float64, c *Camera, lim Limits) {
	if dt < 0 {
		dt = 0
	}
	s.stepZoom(dt, c, lim)
	s.stepPan(dt, c)
	s.clamp(c, lim)
}

func (s *Smooth) stepZoom(dt float64, c *Camera, lim Limits) {
	if s.PendingScale != 0 && s.PendingScale != 1 {
		base := c.Zoom
		if s.Zooming {
			base = s.TargetZoom
		}
		s.TargetZoom = math.Max(lim.MinZoom, math.Min(MaxZoom, base*s.PendingScale))
		s.Zooming = true
	}
	s.PendingScale = 0
	if !s.Zooming {
		return
	}
	next := s.TargetZoom
	if !s.Instant && math.Abs(s.TargetZoom-c.Zoom) > zoomSnapEps*s.TargetZoom {
		next = c.Zoom + (s.TargetZoom-c.Zoom)*(1-math.Exp(-dt/zoomTau))
	} else {
		s.Zooming = false
	}
	old := c.Tile()
	wx, wy := (s.AnchorX+c.OffsetX)/old, (s.AnchorY+c.OffsetY)/old
	c.Zoom = next
	t := c.Tile()
	c.OffsetX, c.OffsetY = wx*t-s.AnchorX, wy*t-s.AnchorY
}

func (s *Smooth) stepPan(dt float64, c *Camera) {
	boost := 1.0
	if s.Boost > 0 {
		boost = s.Boost
	}
	if c.Dev > 0 { // pan speeds are in points: the same apparent speed on a dense screen
		boost *= c.Dev
	}
	if s.Instant {
		c.OffsetX += s.DirX * PanPixels * boost
		c.OffsetY += s.DirY * PanPixels * boost
		s.Vx, s.Vy, s.DirX, s.DirY = 0, 0, 0, 0
		return
	}
	k := 1 - math.Exp(-dt/panTau)
	top := PanMaxSpeed
	if s.Speed > 0 {
		top *= s.Speed
	}
	top *= boost
	s.Vx += (s.DirX*top - s.Vx) * k
	s.Vy += (s.DirY*top - s.Vy) * k
	// dirX/dirY stay set: several Draws step one Update's intent (applyCamera clears them).
	if math.Abs(s.Vx) < 0.5 {
		s.Vx = 0
	}
	if math.Abs(s.Vy) < 0.5 {
		s.Vy = 0
	}
	c.OffsetX += s.Vx * dt
	c.OffsetY += s.Vy * dt
}

// clamp bounds the offsets with ClampAxis and kills velocity on an axis that
// hit its limit, so the camera never overshoots and then rebounds.
func (s *Smooth) clamp(c *Camera, lim Limits) {
	t := c.Tile()
	cx := ClampAxis(c.OffsetX, lim.MapW*t, lim.V.X0, lim.V.X1, EdgeSlack*lim.V.W())
	cy := ClampAxis(c.OffsetY, lim.MapH*t, lim.V.Y0, lim.V.Y1, EdgeSlack*lim.V.H())
	if cx != c.OffsetX {
		s.Vx = 0
	}
	if cy != c.OffsetY {
		s.Vy = 0
	}
	c.OffsetX, c.OffsetY = cx, cy
}

// SyncDev adopts the screen's density dev, rescaling the camera's physical
// offsets and the smoother's velocity so every point-space position keeps
// showing the same world point (HiDPI toggled, or the window moved to
// another screen). The zoom is apparent, so it does not change.
func (c *Camera) SyncDev(dev float64, s *Smooth) {
	old := c.Dev
	if old <= 0 {
		old = 1
	}
	if dev == c.Dev {
		return
	}
	k := dev / old
	c.OffsetX *= k
	c.OffsetY *= k
	s.Vx *= k
	s.Vy *= k
	s.AnchorX *= k
	s.AnchorY *= k
	c.Dev = dev
}

// MinZoomFor is the furthest zoom-out for a mapW x mapH tile map seen
// through v: the whole map fits with a small margin, unless that would
// leave more than 1-MinCover of the screen black along its wider side.
// v is in physical pixels (one tile is TileSize x dev at zoom 1), so the
// limit is the same apparent zoom at any density.
func (c *Camera) MinZoomFor(v Viewport, mapW, mapH int) float64 {
	unit := (&Camera{Zoom: 1, Dev: c.Dev}).Tile()
	mw := float64(mapW) * unit
	mh := float64(mapH) * unit
	if v.W() <= 0 || v.H() <= 0 || mw <= 0 || mh <= 0 {
		return MinZoom
	}
	fx, fy := v.W()/mw, v.H()/mh
	z := math.Max(math.Min(fx, fy)*(1-fitSlack), math.Max(fx, fy)*MinCover)
	return math.Max(0.1, math.Min(z, 1))
}

// Clamp keeps the zoom in range and the map in view. Each map edge may be
// scrolled at most EdgeSlack of the viewport inside it; on an axis where
// the map is smaller than that it is centred instead. The first call
// centres the camera on the map, or on landing (a tile) when it is set and
// not the map's centre. v is the physical map viewport.
func (c *Camera) Clamp(v Viewport, mapW, mapH int, landing [2]int) {
	if v.W() <= 0 || v.H() <= 0 {
		return
	}
	if mz := c.MinZoomFor(v, mapW, mapH); c.Zoom < mz {
		c.Zoom = mz
	}
	if c.hasPend && !c.Placed {
		c.Zoom = math.Max(c.MinZoomFor(v, mapW, mapH), math.Min(c.pend.Zoom, MaxZoom))
	}
	t := c.Tile()
	mw, mh := float64(mapW)*t, float64(mapH)*t
	if !c.Placed && c.hasPend {
		c.Placed, c.hasPend = true, false
		cx := math.Max(0, math.Min(c.pend.CX, float64(mapW))) * t
		cy := math.Max(0, math.Min(c.pend.CY, float64(mapH))) * t
		c.OffsetX = cx - (v.X0+v.X1)/2
		c.OffsetY = cy - (v.Y0+v.Y1)/2
	} else if !c.Placed {
		c.Placed = true
		cx, cy := mw/2, mh/2
		if landing != [2]int{} && landing != [2]int{mapW / 2, mapH / 2} {
			cx, cy = (float64(landing[0])+0.5)*t, (float64(landing[1])+0.5)*t // a new colony starts on its landing spot
		}
		c.OffsetX = cx - (v.X0+v.X1)/2
		c.OffsetY = cy - (v.Y0+v.Y1)/2
	}
	c.OffsetX = ClampAxis(c.OffsetX, mw, v.X0, v.X1, EdgeSlack*v.W())
	c.OffsetY = ClampAxis(c.OffsetY, mh, v.Y0, v.Y1, EdgeSlack*v.H())
}

// LimitsFor are the smoothing bounds for a mapW x mapH tile map seen through v.
func (c *Camera) LimitsFor(v Viewport, mapW, mapH int) Limits {
	return Limits{V: v, MapW: float64(mapW), MapH: float64(mapH), MinZoom: c.MinZoomFor(v, mapW, mapH)}
}
