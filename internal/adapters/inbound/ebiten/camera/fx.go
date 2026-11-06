package camera

import "math"

// Fx is the camera's transient presentation offset: the glide that eases a
// jump (OPT-022) and the screen shake of blasts and lightning (OPT-027). It
// is added to the camera's offset only while a frame draws, never to the
// camera itself, so hit tests, clamps and saves see the real position. It
// runs on the presentation clock (wall seconds per Draw), never on sim ticks,
// and uses no RNG: the shake is two fixed-frequency waves.
type Fx struct {
	GlideX, GlideY float64 // px between where the camera is and where the frame still shows it
	amp            float64 // shake amplitude, physical px
	t              float64 // shake clock, s
	rx, ry, rz     float64 // the offset and zoom the last Observe saw
	seen           bool
	ox, oy         float64 // the offsets Apply found, for Restore
	applied        bool
}

// Observe notes where the camera rests after a frame's step and, when glide
// is on, starts a glide for a move since the last call that no pan, drag or
// zoom ease explains: a jump of more than thresh px at the same zoom (a
// "jump to", the minimap, a chase on a threat pause).
func (f *Fx) Observe(x, y, zoom, thresh float64, glide bool) {
	if glide && f.seen && zoom == f.rz && math.Hypot(x-f.rx, y-f.ry) > thresh {
		f.Jump(f.rx, f.ry, x, y)
	}
	f.rx, f.ry, f.rz, f.seen = x, y, zoom, true
}

// Apply adds the current offset to the camera's offsets (a frame's draw) and
// Restore puts the exact offsets back, so nothing outside the frame sees it.
func (f *Fx) Apply(offX, offY *float64) {
	dx, dy := f.Offset()
	f.ox, f.oy, f.applied = *offX, *offY, true
	*offX += dx
	*offY += dy
}

// Restore undoes the last Apply.
func (f *Fx) Restore(offX, offY *float64) {
	if f.applied {
		*offX, *offY, f.applied = f.ox, f.oy, false
	}
}

const (
	glideTau   = 0.07 // s; a jump settles in ~200 ms
	glideMax   = 1200 // px; a longer jump glides only the last stretch
	shakeTau   = 0.16 // s; a shake dies away in ~0.5 s
	shakeMax   = 40   // px; however many blasts land together
	shakeEps   = 0.25 // px; below this the shake is off
	glideEps   = 0.5  // px
	shakeFreqX = 43.0 // rad/s
	shakeFreqY = 57.0
)

// Jump starts a glide from the offset the camera had to the one it has now.
func (f *Fx) Jump(oldX, oldY, newX, newY float64) {
	f.GlideX = clampAbs(f.GlideX+oldX-newX, glideMax)
	f.GlideY = clampAbs(f.GlideY+oldY-newY, glideMax)
}

// Shake adds px of amplitude (physical pixels).
func (f *Fx) Shake(px float64) {
	if px > 0 {
		f.amp = math.Min(shakeMax, f.amp+px)
	}
}

// Active reports an offset is in flight.
func (f *Fx) Active() bool { return f.amp > 0 || f.GlideX != 0 || f.GlideY != 0 }

// Clear drops everything in flight (reduced motion, a captured frame, a map switch).
func (f *Fx) Clear() { *f = Fx{} }

// Step advances the effects by dt wall seconds.
func (f *Fx) Step(dt float64) {
	if dt <= 0 || !f.Active() {
		return
	}
	k := math.Exp(-dt / glideTau)
	f.GlideX, f.GlideY = f.GlideX*k, f.GlideY*k
	if math.Abs(f.GlideX) < glideEps {
		f.GlideX = 0
	}
	if math.Abs(f.GlideY) < glideEps {
		f.GlideY = 0
	}
	if f.amp > 0 {
		f.t += dt
		f.amp *= math.Exp(-dt / shakeTau)
		if f.amp < shakeEps {
			f.amp, f.t = 0, 0
		}
	}
}

// Offset is what to add to the camera's offset for this frame.
func (f *Fx) Offset() (dx, dy float64) {
	dx, dy = f.GlideX, f.GlideY
	if f.amp > 0 {
		dx += f.amp * math.Sin(f.t*shakeFreqX)
		dy += f.amp * math.Sin(f.t*shakeFreqY+1.3)
	}
	return dx, dy
}

func clampAbs(v, lim float64) float64 { return math.Max(-lim, math.Min(lim, v)) }

// WantsWorld reports a zoom-out request that has nowhere left to go: the
// camera sits at its zoom-out floor with no ease in flight, so one more
// notch out means "show me the world" (OPT-024).
func WantsWorld(zoom, floor float64, easing, zoomingOut bool) bool {
	return zoomingOut && !easing && zoom <= floor*1.001
}

// DragScale is the map-drag multiplier for a percentage (DragPct; 0 = 100).
func DragScale(pct int) float64 {
	if pct <= 0 {
		return 1
	}
	return float64(pct) / 100
}

// ZoomAnchor is the screen point a zoom keeps fixed: the cursor (ZoomToMouse
// on) or the middle of the viewport.
func ZoomAnchor(toMouse bool, mx, my float64, v Viewport) (x, y float64) {
	if toMouse {
		return mx, my
	}
	return (v.X0 + v.X1) / 2, (v.Y0 + v.Y1) / 2
}

// Prefs are the camera and pause options the renderer applies (Options:
// Controls, General). The zero value is the old behaviour, except the
// shake, which is off.
type Prefs struct {
	CenterZoom  bool    // wheel zooms about the viewport middle (ZoomToMouse off)
	Glide       bool    // SmoothJumps
	Drag        int     // DragPct; 0 = 100
	WorldOut    bool    // ZoomOutToWorld
	Shake       float64 // ShakePct / 100
	Mode        string  // auto-pause mode (config.AutoPause*)
	UnpauseLast bool    // Space resumes at the speed before the pause
	LastSpeed   int     // that speed (0: none)
	Started     bool    // the renderer was synced once (PauseOnLoad is a start-up rule)
	LetterInit  bool    // the letters present at start are noted, not paused on
	LastLetter  uint64  // newest letter id handled
	BlastTick   uint64  // newest explosion shaken for
	BlastInit   bool    // the explosions present at start are noted, not shaken for
}
