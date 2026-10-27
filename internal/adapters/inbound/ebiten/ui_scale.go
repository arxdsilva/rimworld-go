package ebiten

import (
	"math"
	"slices"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/config"
)

// UI scaling (PLAY-012 B.1, B.2). Two scales:
//
//   - dev: the screen's device pixel density (1, or 2 on Retina), so the
//     game renders at the screen's true resolution (Graphics.HiDPI);
//   - ui: how big the interface looks (Auto from the window, or a preset).
//
// The total UI scale is s = dev x ui. The screen Draw receives is physical
// (outside size x dev). The UI lays itself out in logical pixels
// (physical / s), exactly as at s = 1, and draws through the scaling
// canvas (ui_canvas.go, font.go). The map renders at physical resolution
// and scales its tiles by dev only. Captures force s = 1.
//
// A third factor, world (PERF-002, Graphics.RenderScale), draws the map
// into an offscreen at world x the physical pixels (world_scale.go): the
// camera then works in "map pixels" (physical x world, screenDev() =
// dev x world) and the HUD and every label stay native.

// uiScaleState is one App's scale. world is the map's render scale (0 or
// 1: the map draws at every physical pixel; see worldK).
type uiScaleState struct{ dev, ui, s, world float64 }

var unitUIScale = uiScaleState{dev: 1, ui: 1, s: 1, world: 1}

// worldK is the map's render scale: world when it is a reduced one
// (0 < world < 1), else 1.
func (u uiScaleState) worldK() float64 {
	if u.world > 0 && u.world < 1 {
		return u.world
	}
	return 1
}

// curUIScale is the scale the draw helpers and the font read. The App
// writes it on the main goroutine at the start of Update and Draw, and
// only when it changed, so parallel tests at the default scale only read
// it. Tests that need another scale use withUIScale (never in parallel).
var curUIScale = unitUIScale

// Auto UI scale bounds and rounding.
const (
	autoUIMin  = 1.0
	autoUIMax  = 2.0
	autoUIStep = 0.05
)

// autoUIScale is the Auto scale for a w x h window in device-independent
// points: the factor that keeps the 1280x720 baseline layout's share of
// the screen, min(w/1280, h/720), clamped to 1..2 and floored to 0.05 (a
// 1080p window gives 1.5). Flooring and the width term keep the logical
// window at least the setup screens' minimum (1280x720) on any window
// that is at least that big, including 16:10 ones.
func autoUIScale(w, h float64) float64 {
	if w <= 0 || h <= 0 {
		return autoUIMin
	}
	f := math.Min(w/minSetupW, h/minSetupH)
	f = math.Floor(f/autoUIStep+1e-6) * autoUIStep
	f = math.Round(f*100) / 100 // exact hundredths (1.5, not 1.5000000000000002)
	return math.Max(autoUIMin, math.Min(autoUIMax, f))
}

// computeUIScale is the scale for the settings, a w x h window in points
// and the screen's device scale factor. Captures are always 1. The map's
// render scale never takes the map below one pixel per point (1/dev): at
// dev 1 (HiDPI off, or a 1x screen) a smaller map would only blur.
func computeUIScale(st config.Settings, w, h, dev float64, capture bool) uiScaleState {
	if capture {
		return unitUIScale
	}
	if !st.Graphics.HiDPI || dev <= 0 || math.IsNaN(dev) {
		dev = 1
	}
	world := 1.0
	if dev > 1 {
		world = math.Max(renderScaleOf(st.Graphics), 1/dev)
	}
	ui := float64(st.Accessibility.UIScale) / 100
	if st.Accessibility.UIScaleAuto {
		ui = autoUIScale(w, h)
	}
	if ui <= 0 {
		ui = 1
	}
	return uiScaleState{dev: dev, ui: ui, s: dev * ui, world: world}
}

// renderScaleOf is the settings' map render scale: one of
// config.RenderScales, else 1.
func renderScaleOf(g config.Graphics) float64 {
	if slices.Contains(config.RenderScales, g.RenderScale) {
		return g.RenderScale
	}
	return 1
}

// monitorScale is the current monitor's device scale factor (1 when
// there is no monitor yet).
func monitorScale() float64 {
	if m := ebiten.Monitor(); m != nil {
		return m.DeviceScaleFactor()
	}
	return 1
}

// deviceScale is the screen's raw device scale factor (tests inject devFn).
func (a *App) deviceScale() float64 {
	if a.devFn != nil {
		return a.devFn()
	}
	return monitorScale()
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
	pw, ph := outsideW*a.scale.dev, outsideH*a.scale.dev
	a.setPhysical(int(math.Ceil(pw)), int(math.Ceil(ph)))
	return pw, ph
}

// Layout keeps the int path consistent with LayoutF (Ebitengine calls
// LayoutF when it exists).
func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	pw, ph := a.LayoutF(float64(outsideWidth), float64(outsideHeight))
	return int(math.Ceil(pw)), int(math.Ceil(ph))
}

// updateScale recomputes the scale from the settings, the last window
// size and the last device factor (LayoutF and applySettings).
func (a *App) updateScale() {
	a.scale = computeUIScale(a.settings, a.outW, a.outH, a.devRaw, a.cfg.Capture != nil)
	if s := a.captureScale(); s > 0 {
		a.scale = uiScaleState{dev: 1, ui: s, s: s, world: 1}
	}
	if a.cfg.Capture != nil {
		// dev escape hatch: captures load the defaults (render scale 1), so
		// only an explicit -render-scale reaches here; it shows the scaled
		// map path in a screenshot
		a.scale.world = renderScaleOf(a.settings.Graphics)
	}
	if a.pw > 0 && a.ph > 0 {
		a.setPhysical(a.pw, a.ph)
	}
	a.syncUIScale()
}

// setPhysical records a physical screen size and its logical size.
func (a *App) setPhysical(pw, ph int) {
	a.pw, a.ph = pw, ph
	a.sw, a.sh = logicalPx(pw, a.scale.s), logicalPx(ph, a.scale.s)
}

// logicalPx is a physical length in logical pixels, rounded up.
func logicalPx(p int, s float64) int {
	if s == 1 || s <= 0 {
		return p
	}
	return int(math.Ceil(float64(p)/s - 1e-9))
}

// syncUIScale mirrors the App's scale into curUIScale (main goroutine,
// once per Update and Draw; written only on a change).
func (a *App) syncUIScale() {
	if a.scale.s <= 0 {
		a.scale = unitUIScale
	}
	if curUIScale != a.scale {
		curUIScale = a.scale
	}
}

// uiScale is the total UI scale the canvas draws with. During a world
// pass into the render-scaled map image (world_scale.go) it is s x world,
// so a world pass's hand conversion (map pixels / uiScale()) still gives
// logical pixels.
func uiScale() float64 {
	if worldPassK != 0 {
		return curUIScale.s * worldPassK
	}
	return curUIScale.s
}

// hudScale is the UI scale the HUD and every label draw at, also during a
// world pass: text is measured at the scale it is drawn at.
func hudScale() float64 { return curUIScale.s }

// mapUIScale is map pixels per logical UI pixel (s x the render scale),
// in and out of a world pass: for map <-> logical conversions that also run
// in Update (hit tests).
func mapUIScale() float64 { return curUIScale.s * curUIScale.worldK() }

// mapScale is logical UI pixels -> map pixels (s x the render scale): the
// one conversion from a logical point or size to the camera's pixels.
func (a *App) mapScale() float64 { return a.scale.s * a.scale.worldK() }

// toLogical maps a physical (raw cursor) point to logical UI pixels.
func (a *App) toLogical(px, py int) (int, int) {
	s := a.scale.s
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
