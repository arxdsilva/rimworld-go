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
	V          viewport
	MapW, MapH float64 // map size in tiles
	MinZoom    float64
}

// camSmooth is presentation-only camera motion: pan velocity with
// acceleration and critically damped friction, and eased zoom anchored at a
// screen point. It never touches domain state. With instant set (capture
// mode) pan and zoom apply at once and deterministically.
type camSmooth struct {
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
func (s *camSmooth) Halt() {
	s.Vx, s.Vy, s.DirX, s.DirY = 0, 0, 0, 0
	s.Zooming, s.PendingScale = false, 0
}

// Idle reports nothing is in motion: no pan velocity or input and no zoom
// ease in flight. A Draw-time step (stepCamera) skips an Idle smoother, so
// a camera that code placed directly stays exactly there until input moves
// it; Update's applyCamera still clamps every frame.
func (s *camSmooth) Idle() bool {
	return s.Vx == 0 && s.Vy == 0 && s.DirX == 0 && s.DirY == 0 && !s.Zooming &&
		(s.PendingScale == 0 || s.PendingScale == 1)
}

// PushPan records held-pan input for the next step; dx, dy are directions
// (-1, 0, 1), summed and clamped to unit range per axis.
func (s *camSmooth) PushPan(dx, dy float64) {
	s.DirX = math.Max(-1, math.Min(1, s.DirX+dx))
	s.DirY = math.Max(-1, math.Min(1, s.DirY+dy))
}

// ZoomAt requests a zoom change by factor, anchored at physical screen
// point (sx, sy).
func (s *camSmooth) ZoomAt(factor, sx, sy float64) {
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
func (s *camSmooth) Step(dt float64, c *camera, lim camLimits) {
	if dt < 0 {
		dt = 0
	}
	s.stepZoom(dt, c, lim)
	s.stepPan(dt, c)
	s.clamp(c, lim)
}

func (s *camSmooth) stepZoom(dt float64, c *camera, lim camLimits) {
	if s.PendingScale != 0 && s.PendingScale != 1 {
		base := c.Zoom
		if s.Zooming {
			base = s.TargetZoom
		}
		s.TargetZoom = math.Max(lim.MinZoom, math.Min(maxZoom, base*s.PendingScale))
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

func (s *camSmooth) stepPan(dt float64, c *camera) {
	boost := 1.0
	if s.Boost > 0 {
		boost = s.Boost
	}
	if c.Dev > 0 { // pan speeds are in points: the same apparent speed on a dense screen
		boost *= c.Dev
	}
	if s.Instant {
		c.OffsetX += s.DirX * panPixels * boost
		c.OffsetY += s.DirY * panPixels * boost
		s.Vx, s.Vy, s.DirX, s.DirY = 0, 0, 0, 0
		return
	}
	k := 1 - math.Exp(-dt/panTau)
	top := panMaxSpeed
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

// clamp bounds the offsets with clampAxis and kills velocity on an axis that
// hit its limit, so the camera never overshoots and then rebounds.
func (s *camSmooth) clamp(c *camera, lim camLimits) {
	t := c.Tile()
	cx := clampAxis(c.OffsetX, lim.MapW*t, lim.V.X0, lim.V.X1, edgeSlack*lim.V.W())
	cy := clampAxis(c.OffsetY, lim.MapH*t, lim.V.Y0, lim.V.Y1, edgeSlack*lim.V.H())
	if cx != c.OffsetX {
		s.Vx = 0
	}
	if cy != c.OffsetY {
		s.Vy = 0
	}
	c.OffsetX, c.OffsetY = cx, cy
}
