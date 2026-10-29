package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The renderer's side of the colony HUD (ARCH-005 wave 5.5): the overlay
// toggles and the Architect's forced overlays, the roof tool's ghost, the
// camera jumps, and why the game paused itself. The HUD's regions live in
// package hud and reach these through hudHost (hud_compat.go).

// planOverlayOn reports the Plan overlay is showing (toggle or tool held).
func (r *Renderer) planOverlayOn() bool { return r.overlays().Plan.Shown() }

// togglePowerOverlayIfAny tolerates a nil renderer (menus).
func (r *Renderer) togglePowerOverlayIfAny() {
	if r != nil {
		r.overlays().Power.Toggle()
	}
}

// toggleRoofOverlayIfAny tolerates a nil renderer (menus).
func (r *Renderer) toggleRoofOverlayIfAny() {
	if r != nil {
		r.overlays().Roof.Toggle()
	}
}

// homeShown reports the Home overlay is showing (toggle or tool held).
func (r *Renderer) homeShown() bool { return r.overlays().Home.On || r.overlays().Home.Forced }

// toggleHomeOverlayIfAny is the HUD Home button action; it tolerates a nil
// renderer (menus).
func (r *Renderer) toggleHomeOverlayIfAny() {
	if r != nil {
		r.overlays().Home.Toggle()
	}
}

// togglePlanOverlayIfAny is the HUD Plans button action; it tolerates a nil
// renderer (menus).
func (r *Renderer) togglePlanOverlayIfAny() {
	if r != nil {
		r.overlays().Plan.Toggle()
	}
}

// roofToolID reports id is one of the two roof Architect tools.
func roofToolID(id string) bool { return id == "roof" || id == "noroof" }

// roofForced is whether the overlay must be forced on: a roof tool is the
// selected tool of an open Architect.
func roofForced(archOpen bool, toolID string) bool { return archOpen && roofToolID(toolID) }

// syncRoofForce keeps renderer.overlays().Roof.Forced (and the Home overlay's
// forced flag) in step with the Architect.
func (a *App) syncRoofForce() {
	a.syncHomeForce()
	a.syncPlanForce()
	a.syncSnowClearForce() // snow_clear_draw.go (PLAY-015)
	a.syncPowerForce()     // power_overlay.go (PLAY-021)
	if a.renderer != nil {
		a.renderer.overlays().Roof.Forced = roofForced(a.arch.Open, a.arch.ToolID)
	}
}

// roofGhostUnsupported decides whether a cell under the Build roof
// rectangle gets the red "can't roof" mark: any non-wall cell lacking
// the Supported flag.
func roofGhostUnsupported(flags uint8, wall bool) bool {
	return !wall && flags&overlaypkg.RoofSupported == 0
}

// drawRoofToolGhost marks unsupported cells under the dragged rectangle.
func (a *App) drawRoofToolGhost(screen *ebiten.Image, x0, y0, x1, y1 int) {
	r := a.renderer
	w := a.shell.World()
	walls := map[[2]int]bool{}
	for _, b := range w.Buildings {
		if b.Kind == "wall" || b.Kind == "door" {
			walls[[2]int{b.X, b.Y}] = true
		}
	}
	tile := a.shell.TilePx()
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	ov, v := r.overlays(), r.liveView()
	for _, c := range architect.PlacementCells(architect.PlaceArea, x0, y0, x1, y1) {
		if !roofGhostUnsupported(overlaypkg.RoofCellAt(w, c[0], c[1]), walls[c]) {
			continue
		}
		sx, sy := a.shell.WorldToScreen(c[0], c[1])
		if sprite.Visible(sx, sy, float32(tile), float32(tile), sw, sh) {
			ov.DrawRoofUnsupported(v, screen, c[0], c[1], tile) // render/overlay roof_draw.go
		}
	}
}

// homeToolID reports id is one of the two Home Architect tools.
func homeToolID(id string) bool { return id == "home" || id == "unhome" }

// homeForced is whether the Home overlay must be forced on: a Home tool is
// in hand.
func homeForced(archOpen bool, toolID string) bool { return archOpen && homeToolID(toolID) }

// syncHomeForce keeps renderer.overlays().Home.Forced in step with the Architect.
func (a *App) syncHomeForce() {
	if a.renderer != nil {
		a.renderer.overlays().Home.Forced = homeForced(a.arch.Open, a.arch.ToolID)
	}
}

// planToolID reports id is one of the two Plan Architect tools.
func planToolID(id string) bool { return id == "plan" || id == "unplan" }

// planForced is whether the Plan overlay must be forced on: a Plan tool is
// in hand.
func planForced(archOpen bool, toolID string) bool { return archOpen && planToolID(toolID) }

// syncPlanForce keeps renderer.overlays().Plan.Forced in step with the Architect.
func (a *App) syncPlanForce() {
	if a.renderer != nil {
		a.renderer.overlays().Plan.Forced = planForced(a.arch.Open, a.arch.ToolID)
	}
}

// minimapOn reports the layout should reserve the minimap's square.
func (a *App) minimapOn() bool { return a.renderer.minimapOn() }

// The minimap (render/minimap): the renderer holds the Map by value (its
// zero value is a shown, empty minimap) and owns the camera a click moves.

// minimapOn reports the minimap is shown; it tolerates a nil renderer.
func (r *Renderer) minimapOn() bool { return r != nil && r.mini.On(r.game) }

// toggleMinimap shows or hides the minimap; it tolerates a nil renderer.
func (r *Renderer) toggleMinimap() {
	if r != nil {
		r.mini.Toggle()
	}
}

// minimapClick centres the camera on the tile under the logical point
// (sx, sy) when it is on the minimap (minimap.Map.Click returns the
// offsets; the camera, smoothing and clamp stay the renderer's).
func (r *Renderer) minimapClick(sx, sy int, ui, v camerapkg.Viewport) bool {
	ok, ox, oy := r.mini.Click(r.liveView(), r.game, sx, sy, ui, v)
	if !ok {
		return false
	}
	r.cam.OffsetX, r.cam.OffsetY = ox, oy
	r.smooth.Vx, r.smooth.Vy = 0, 0
	r.ClampCamera(v)
	return true
}

// drawMinimap draws the minimap in the UI viewport ui for the map
// viewport v.
func (r *Renderer) drawMinimap(screen *ebiten.Image, ui, v camerapkg.Viewport) {
	r.mini.Draw(r.liveView(), screen, r.game, ui, v)
}

// speedReason says why the game is paused when the game did it itself: the
// window went to the background, or a threat arrived. "" otherwise.
func (a *App) speedReason() string {
	if a.bg.paused {
		return bgPausedLabel
	}
	r := a.renderer
	if r == nil {
		return ""
	}
	r.speedRate() // drops a stale threat flag once the game runs again
	if r.threatPaused && r.speed == speedPaused {
		return "Paused (threat)"
	}
	return ""
}

// centerOn moves the camera so world point (wx, wy) is mid-screen.
// sw x sh is the screen in the map's pixels (captures run at scale 1).
func (r *Renderer) centerOn(wx, wy float64, sw, sh int) {
	r.centerOnView(wx, wy, camerapkg.Viewport{X0: 0, Y0: 0, X1: float64(sw), Y1: float64(sh)}, camerapkg.ColonyViewport(sw, sh))
}

// centerOnView centres the camera on a world point in the middle of the
// physical screen `screen`, clamped to the physical map viewport v.
func (r *Renderer) centerOnView(wx, wy float64, screen, v camerapkg.Viewport) {
	tile := r.cam.Tile()
	r.cam.OffsetX = wx*tile - (screen.X0+screen.X1)/2
	r.cam.OffsetY = wy*tile - (screen.Y0+screen.Y1)/2
	r.cam.Placed = true // an explicit jump; don't recentre on the map
	r.ClampCamera(v)
}

// centerOnMap is the UI's "jump to": the target goes to the middle of the
// physical map viewport.
func (a *App) centerOnMap(wx, wy float64) {
	v := a.mapViewport()
	a.renderer.centerOnView(wx, wy, v, v)
}
