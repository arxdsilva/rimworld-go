package ebiten

import "math"

const (
	panMaxSpeed = panPixels * 60.0 // px/s, matches the old 8 px per 60 fps frame
	panTau      = 0.045            // s; velocity time constant (stop in ~200 ms)
	zoomTau     = 0.05             // s; zoom ease time constant (~150 ms to settle)
	zoomSnapEps = 0.0005           // relative gap at which zoom snaps to target
)

// camLimits are the bounds step applies, taken from the clamp helpers.
type camLimits struct {
	v          viewport
	mapW, mapH float64 // map size in tiles
	minZoom    float64
}

// camSmooth is presentation-only camera motion: pan velocity with
// acceleration and critically damped friction, and eased zoom anchored at a
// screen point. It never touches domain state. With instant set (capture
// mode) pan and zoom apply at once and deterministically.
type camSmooth struct {
	vx, vy       float64 // pan velocity, physical px/s
	dirX, dirY   float64 // pan input pushed this frame, consumed by step
	targetZoom   float64
	zooming      bool
	pendingScale float64 // product of zoomAt factors not yet folded into targetZoom
	anchorX      float64 // screen point kept fixed while zooming
	anchorY      float64
	instant      bool
	speed        float64 // pan speed multiplier (Options); 0 means 1
	boost        float64 // fast-pan multiplier this frame (Shift); 0 means 1
}

// halt stops all camera motion: pan velocity and input, and a zoom ease in
// flight (the zoom stays where it is). A jump to a point calls it so the
// camera does not drift off the target afterwards. The pan-speed setting
// is kept.
func (s *camSmooth) halt() {
	s.vx, s.vy, s.dirX, s.dirY = 0, 0, 0, 0
	s.zooming, s.pendingScale = false, 0
}

// idle reports nothing is in motion: no pan velocity or input and no zoom
// ease in flight. A Draw-time step (stepCamera) skips an idle smoother, so
// a camera that code placed directly stays exactly there until input moves
// it; Update's applyCamera still clamps every frame.
func (s *camSmooth) idle() bool {
	return s.vx == 0 && s.vy == 0 && s.dirX == 0 && s.dirY == 0 && !s.zooming &&
		(s.pendingScale == 0 || s.pendingScale == 1)
}

// pushPan records held-pan input for the next step; dx, dy are directions
// (-1, 0, 1), summed and clamped to unit range per axis.
func (s *camSmooth) pushPan(dx, dy float64) {
	s.dirX = math.Max(-1, math.Min(1, s.dirX+dx))
	s.dirY = math.Max(-1, math.Min(1, s.dirY+dy))
}

// zoomAt requests a zoom change by factor, anchored at physical screen
// point (sx, sy).
func (s *camSmooth) zoomAt(factor, sx, sy float64) {
	if factor <= 0 {
		return
	}
	if s.pendingScale == 0 {
		s.pendingScale = 1
	}
	s.pendingScale *= factor
	s.anchorX, s.anchorY = sx, sy
}

// step advances the camera by dt seconds and applies the bounds.
func (s *camSmooth) step(dt float64, c *camera, lim camLimits) {
	if dt < 0 {
		dt = 0
	}
	s.stepZoom(dt, c, lim)
	s.stepPan(dt, c)
	s.clamp(c, lim)
}

func (s *camSmooth) stepZoom(dt float64, c *camera, lim camLimits) {
	if s.pendingScale != 0 && s.pendingScale != 1 {
		base := c.zoom
		if s.zooming {
			base = s.targetZoom
		}
		s.targetZoom = math.Max(lim.minZoom, math.Min(maxZoom, base*s.pendingScale))
		s.zooming = true
	}
	s.pendingScale = 0
	if !s.zooming {
		return
	}
	next := s.targetZoom
	if !s.instant && math.Abs(s.targetZoom-c.zoom) > zoomSnapEps*s.targetZoom {
		next = c.zoom + (s.targetZoom-c.zoom)*(1-math.Exp(-dt/zoomTau))
	} else {
		s.zooming = false
	}
	old := c.tile()
	wx, wy := (s.anchorX+c.offsetX)/old, (s.anchorY+c.offsetY)/old
	c.zoom = next
	t := c.tile()
	c.offsetX, c.offsetY = wx*t-s.anchorX, wy*t-s.anchorY
}

func (s *camSmooth) stepPan(dt float64, c *camera) {
	boost := 1.0
	if s.boost > 0 {
		boost = s.boost
	}
	if c.dev > 0 { // pan speeds are in points: the same apparent speed on a dense screen
		boost *= c.dev
	}
	if s.instant {
		c.offsetX += s.dirX * panPixels * boost
		c.offsetY += s.dirY * panPixels * boost
		s.vx, s.vy, s.dirX, s.dirY = 0, 0, 0, 0
		return
	}
	k := 1 - math.Exp(-dt/panTau)
	top := panMaxSpeed
	if s.speed > 0 {
		top *= s.speed
	}
	top *= boost
	s.vx += (s.dirX*top - s.vx) * k
	s.vy += (s.dirY*top - s.vy) * k
	// dirX/dirY stay set: several Draws step one Update's intent (applyCamera clears them).
	if math.Abs(s.vx) < 0.5 {
		s.vx = 0
	}
	if math.Abs(s.vy) < 0.5 {
		s.vy = 0
	}
	c.offsetX += s.vx * dt
	c.offsetY += s.vy * dt
}

// clamp bounds the offsets with clampAxis and kills velocity on an axis that
// hit its limit, so the camera never overshoots and then rebounds.
func (s *camSmooth) clamp(c *camera, lim camLimits) {
	t := c.tile()
	cx := clampAxis(c.offsetX, lim.mapW*t, lim.v.x0, lim.v.x1, edgeSlack*lim.v.w())
	cy := clampAxis(c.offsetY, lim.mapH*t, lim.v.y0, lim.v.y1, edgeSlack*lim.v.h())
	if cx != c.offsetX {
		s.vx = 0
	}
	if cy != c.offsetY {
		s.vy = 0
	}
	c.offsetX, c.offsetY = cx, cy
}
