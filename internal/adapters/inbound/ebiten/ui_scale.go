package ebiten

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"slices"

	"rimworld-go/internal/platform/config"
)

// UI scaling (PLAY-012 B.1, B.2; UIS-001 to UIS-006). Two scales:
//
//   - dev: the screen's device pixel density (1, or 2 on Retina), so the
//     game renders at the screen's true resolution (Graphics.HiDPI);
//   - step: how big the interface looks, in physical window pixels per
//     logical pixel (the reference game's steps, config.UIScalePresets), from
//     Auto or the player's choice. A logical window is the physical window
//     divided by the step, whatever the screen's density.
//
// The frame Draw receives is the outside size x dev (dev may be lower than
// the screen's own factor: HiDPI off or the render cap). The total UI scale
// is s = step x dev / devRaw (frame pixels per logical pixel), and ui =
// s / dev = step / devRaw. The UI lays itself out in logical pixels (frame /
// s), exactly as at s = 1, and draws through the scaling canvas (gfx). The
// map renders at the frame's resolution; its tiles never follow the step.
// Captures force s = 1.
//
// A third factor, world (PERF-002, Graphics.RenderScale), draws the map
// into an offscreen at world x the physical pixels (world_scale.go): the
// camera then works in "map pixels" (physical x world, screenDev() =
// dev x world) and the HUD and every label stay native.

// autoMinH is the logical height Auto keeps (UIS-005): it picks the largest
// step that leaves at least uikit.MinStepW x autoMinH logical pixels.
const autoMinH = 900

// uiStepAllowed reports whether a step leaves at least the minimum logical
// window (UIS-003) on a physW x physH window: step 1 always does.
func uiStepAllowed(physW, physH, step float64) bool {
	return step <= 1 || (physW/step >= uikit.MinStepW-1e-6 && physH/step >= uikit.MinStepH-1e-6)
}

// effectiveStep is the largest allowed step not above the stored one
// (UIS-004): the stored value stays, a smaller window uses a smaller step.
func effectiveStep(physW, physH, stored float64) float64 {
	best := 1.0
	for _, p := range config.UIScalePresets {
		if st := float64(p) / 100; st <= stored+1e-9 && uiStepAllowed(physW, physH, st) {
			best = st
		}
	}
	return best
}

// autoUIStep is the Auto step for a physW x physH window in physical
// pixels: the largest preset leaving at least MinStepW x autoMinH logical
// pixels, else 1 (UIS-005). 1920x1080 gives 1, 2560x1440 1.5, 3840x2160 2.
func autoUIStep(physW, physH float64) float64 {
	best := 1.0
	for _, p := range config.UIScalePresets {
		if st := float64(p) / 100; physW/st >= uikit.MinStepW-1e-6 && physH/st >= autoMinH-1e-6 {
			best = st
		}
	}
	return best
}

// The render cap (PERF-003). One frame never holds more physical pixels
// than a 5K screen at 2x, nor a side above what Metal accepts on every
// Mac: above either, dev shrinks (below 1 if it must) and the window system
// scales the frame up to the screen. A display mode above the panel's
// native size (a 4K panel "looking like" 3360x1890 draws 6720x3780, 25 M
// pixels a frame) therefore costs the GPU and the memory no more than the
// biggest frame that has run here for months. The UI is laid out in points
// (physical window / step), so nothing changes size; only the map's sharpness.
const (
	maxRenderPixels = 5120 * 2880
	maxRenderSide   = 8192
)

// capDev is dev lowered until a w x h window in points fits the render cap.
func capDev(w, h, dev float64) float64 {
	if w <= 0 || h <= 0 || dev <= 0 {
		return dev
	}
	if (w*dev)*(h*dev) > maxRenderPixels {
		dev = math.Sqrt(maxRenderPixels / (w * h))
	}
	if side := math.Max(w, h); side*dev > maxRenderSide {
		dev = (maxRenderSide - 0.5) / side // the physical size rounds up
	}
	return dev
}

// computeUIScale is the scale for the settings, a w x h window in points
// and the screen's raw device scale factor. Captures are always 1. The map's
// render scale never takes the map below one pixel per point (1/dev): at
// dev 1 (HiDPI off, or a 1x screen) a smaller map would only blur.
func computeUIScale(st config.Settings, w, h, devRaw float64, capture bool) gfx.Scale {
	if capture {
		return gfx.UnitScale
	}
	if devRaw <= 0 || math.IsNaN(devRaw) {
		devRaw = 1
	}
	dev := devRaw
	if !st.Graphics.HiDPI {
		dev = 1
	}
	dev = capDev(w, h, dev)
	world := 1.0
	if dev > 1 {
		world = math.Max(renderScaleOf(st.Graphics), 1/dev)
	}
	step := uiStep(st.Accessibility, w*devRaw, h*devRaw, devRaw)
	ui := step / devRaw
	return gfx.Scale{Dev: dev, UI: ui, S: dev * ui, World: world}
}

// stepInUse is the UI step in use for scale sc at the raw device density
// devRaw: physical window pixels per logical pixel (the Options label shows
// it).
func stepInUse(sc gfx.Scale, devRaw float64) float64 {
	if devRaw > 0 {
		return sc.UI * devRaw
	}
	return sc.UI
}

// uiStep is the step in use on a physW x physH window: Auto's, or the
// player's (a legacy share of the points resolved with devRaw) lowered to
// what the window allows.
func uiStep(a config.Accessibility, physW, physH, devRaw float64) float64 {
	if a.UIScaleAuto {
		return autoUIStep(physW, physH)
	}
	return effectiveStep(physW, physH, float64(a.Step(devRaw))/100)
}

// renderScaleOf is the settings' map render scale: one of
// config.RenderScales, else 1.
func renderScaleOf(g config.Graphics) float64 {
	if slices.Contains(config.RenderScales, g.RenderScale) {
		return g.RenderScale
	}
	return 1
}

// deviceScale is the screen's raw device scale factor: AppConfig.DeviceScale
// (the display adapter's live monitor read; tests inject their own), 1
// without one. The App never asks Ebitengine for the monitor itself: since
// v2.10 that starts GLFW on the calling goroutine, which macOS traps off the
// main thread (every test that builds an App).
func (a *App) deviceScale() float64 {
	if a.devFn != nil {
		return a.devFn()
	}
	return 1
}

// LayoutF (ebiten.LayoutFer) makes the screen physical: the outside size
// (points) x dev. It records the physical size, the UI scale and the
// logical size (physical / s, rounded up) every layout and hit test reads.
func (a *App) LayoutF(outsideW, outsideH float64) (float64, float64) {
	if s := a.captureScale(); s > 0 {
		outsideW, outsideH = DefaultWindowWidth*s, DefaultWindowHeight*s
	}
	if outsideW < 1 {
		outsideW = DefaultWindowWidth
	}
	if outsideH < 1 {
		outsideH = DefaultWindowHeight
	}
	a.outW, a.outH = outsideW, outsideH
	a.devRaw = a.deviceScale()
	a.updateScale()
	pw, ph := outsideW*a.scale.Dev, outsideH*a.scale.Dev
	a.setPhysical(int(math.Ceil(pw)), int(math.Ceil(ph)))
	return pw, ph
}

// updateScale recomputes the scale from the settings, the last window
// size and the last device factor (LayoutF and applySettings).
func (a *App) updateScale() {
	a.scale = computeUIScale(a.settings, a.outW, a.outH, a.devRaw, a.cfg.Capture != nil)
	if s := a.captureScale(); s > 0 {
		a.scale = gfx.Scale{Dev: 1, UI: s, S: s, World: 1}
	}
	if a.cfg.Capture != nil {
		// dev escape hatch: captures load the defaults (render scale 1), so
		// only an explicit -render-scale reaches here; it shows the scaled
		// map path in a screenshot
		a.scale.World = renderScaleOf(a.settings.Graphics)
	}
	if a.pw > 0 && a.ph > 0 {
		a.setPhysical(a.pw, a.ph)
	}
	a.syncUIScale()
}

// setPhysical records a physical screen size and its logical size.
func (a *App) setPhysical(pw, ph int) {
	a.pw, a.ph = pw, ph
	a.sw, a.sh = logicalPx(pw, a.scale.S), logicalPx(ph, a.scale.S)
}

// logicalPx is a physical length in logical pixels, rounded up.
func logicalPx(p int, s float64) int {
	if s == 1 || s <= 0 {
		return p
	}
	return int(math.Ceil(float64(p)/s - 1e-9))
}

// syncUIScale mirrors the App's scale into the gfx scale (gfx.SetScale;
// main goroutine, once per Update and Draw; written only on a change).
func (a *App) syncUIScale() {
	if a.scale.S <= 0 {
		a.scale = gfx.UnitScale
	}
	gfx.SetScale(a.scale)
}

// mapScale is logical UI pixels -> map pixels (s x the render scale): the
// one conversion from a logical point or size to the camera's pixels.
func (a *App) mapScale() float64 { return a.scale.S * a.scale.WorldK() }

// toLogical maps a physical (raw cursor) point to logical UI pixels.
func (a *App) toLogical(px, py int) (int, int) {
	s := a.scale.S
	if s == 1 || s <= 0 {
		return px, py
	}
	return int(math.Floor(float64(px) / s)), int(math.Floor(float64(py) / s))
}

// mapToLogical maps a map-pixel point (mapCursor, the map press) to
// logical UI pixels.
func (a *App) mapToLogical(px, py int) (int, int) {
	s := a.mapScale()
	if s == 1 || s <= 0 {
		return px, py
	}
	return int(math.Floor(float64(px) / s)), int(math.Floor(float64(py) / s))
}

// Layout keeps the int path consistent with LayoutF (Ebitengine calls
// LayoutF when it exists).
func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	pw, ph := a.LayoutF(float64(outsideWidth), float64(outsideHeight))
	return int(math.Ceil(pw)), int(math.Ceil(ph))
}
