package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/input"
)

// The lower-right global controls (PLAY-018: GLOBAL owns them): overlay
// toggles, speed buttons, date, time, weather, temperature and conditions.
// Until migrated they are the legacy top strip.

// drawGlobalControls draws the global controls (legacy: the top strip with
// season, weather, clock and the overlay toggles, and the status chips).
func (a *App) drawGlobalControls(screen *ebiten.Image, sw, sh int) {
	drawPanel(screen, 0, 0, float32(sw), hudTopH)
	wld := a.renderer.world()
	x := hudPad // legacy: after the resource readouts (hud_resources.go)
	for _, l := range resourceReadouts(wld) {
		x += max(90, int(textW(l, tsBody))+18)
	}
	if wld.Season != "" {
		season := seasonText(wld)
		drawLabel(screen, x, 10, season)
		seasonW := int(textW(season, tsBody))
		defer a.drawBiomeTooltip(screen, x, seasonW)
		x += seasonW + 18
	}
	drawWeatherLabel(screen, x, 10, wld)
	time := a.hudTimeText()
	drawLabel(screen, sw-int(textW(time, tsBody))-hudPad, 10, time)
	a.drawRoofButton(screen, sw)
	a.drawHomeButton(screen, sw)
	a.drawHomeAutoButton(screen, sw) // home_auto_ui.go
	a.drawPlanButton(screen, sw)
	a.drawRoomsButton(screen, sw)
	a.drawStatusChips(screen, sw) // status_chips.go: heat wave, cold snap, conditions
}

// globalButtons are the global controls' buttons (legacy: the overlay
// toggles in the top strip).
func (a *App) globalButtons(sw int) []button {
	out := a.roofButtons(sw)
	out = append(out, a.homeButtons(sw)...)
	out = append(out, a.homeAutoButtons(sw)...) // home_auto_ui.go
	out = append(out, a.planButtons(sw)...)
	return append(out, a.roomsButtons(sw)...)
}

// globalChrome is the global controls' area while drawn the legacy way.
func (a *App) globalChrome(sw int) []rect { return []rect{{0, 0, float32(sw), hudTopH}} }

// globalHeight is the lower-right controls' height the layout reserves
// (0 while drawn the legacy way).
func (a *App) globalHeight() float32 { return 0 }

// minimapOn reports the layout should reserve the minimap's square.
func (a *App) minimapOn() bool { return false }

// applySpeedClick handles a speed button ("speed.<n>").
func (a *App) applySpeedClick(id string) {}

// globalUI declares the global controls' widget-kit parts (stub).
func (a *App) globalUI(sw, sh int) {}

// ---------------------------------------------------------- Roof overlay (UI-010)

const (
	roofButtonID  = "hud.roof"
	roofButtonW   = 48
	roofLegendTxt = "Roof: blue built · slate natural · dashed planned · hatched removal"
)

// hudTimeText is the right-aligned clock text of the top strip.
func (a *App) hudTimeText() string {
	return fmt.Sprintf("%s  %s  %s", a.renderer.date(), a.speedLabel(), a.renderer.activity())
}

// roofButtons is the HUD Roof toggle: top strip, left of the clock
// (clicks route through applyClick).
func (a *App) roofButtons(sw int) []button {
	if a.renderer == nil {
		return nil
	}
	x := float32(sw-int(textW(a.hudTimeText(), tsBody))-hudPad) - roofButtonW - hudPad
	return []button{{id: roofButtonID, label: "Roof", x: x, y: 4, w: roofButtonW, h: hudTopH - 8, selected: a.renderer.roofOverlayOn(),
		tip: "Roof overlay" + a.keyHint(input.RoofOverlay)}}
}

func (a *App) drawRoofButton(screen *ebiten.Image, sw int) {
	mx, my := a.currentCursor()
	for _, b := range a.roofButtons(sw) {
		drawButton(screen, b, b.contains(mx, my))
		if img := a.icon("overlay_roof"); img != nil {
			// Icon replaces the text label: blank it with the button fill, then draw.
			uiFillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtn, false)
			if b.selected {
				uiFillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtnSelected, false)
			}
			drawIcon(screen, img, float64(b.x)+(float64(b.w)-float64(b.h)+4)/2, float64(b.y)+2, float64(b.h)-4)
		}
		if line := a.roofLegend(); line != "" { // a tab under the bar, clear of the readouts
			lw := textW(line, tsBody) + 2*hudPad
			lx := min(b.x+b.w, float32(sw-hudPad)) - lw
			drawPanel(screen, lx, hudTopH+2, lw, lineStep(tsBody)+6)
			drawLabel(screen, int(lx)+hudPad, int(hudTopH)+7, line)
		}
	}
}

// roofLegend is the one-line key shown while the Roof overlay is on.
func (a *App) roofLegend() string {
	if a.renderer == nil || !a.renderer.roofOverlayOn() {
		return ""
	}
	return roofLegendTxt
}

// toggleRoofOverlay is the Shift+R key and HUD button action.
func (r *Renderer) toggleRoofOverlay() { r.roofOverlay = !r.roofOverlay }

// toggleRoofOverlayIfAny tolerates a nil renderer (menus).
func (r *Renderer) toggleRoofOverlayIfAny() {
	if r != nil {
		r.toggleRoofOverlay()
	}
}

// roofToolID reports id is one of the two roof Architect tools.
func roofToolID(id string) bool { return id == "roof" || id == "noroof" }

// roofForced is whether the overlay must be forced on: a roof tool is the
// selected tool of an open Architect.
func roofForced(archOpen bool, toolID string) bool { return archOpen && roofToolID(toolID) }

// syncRoofForce keeps renderer.roofOverlayForced (and the Home overlay's
// forced flag) in step with the Architect.
func (a *App) syncRoofForce() {
	a.syncHomeForce()
	a.syncPlanForce()
	a.syncSnowClearForce() // snow_clear_draw.go (PLAY-015)
	if a.renderer != nil {
		a.renderer.roofOverlayForced = roofForced(a.archOpen, a.archTool)
	}
}

// roofGhostUnsupported decides whether a cell under the Build roof
// rectangle gets the red "can't roof" mark: any non-wall cell lacking
// the Supported flag.
func roofGhostUnsupported(flags uint8, wall bool) bool {
	return !wall && flags&roofSupported == 0
}

// drawRoofToolGhost marks unsupported cells under the dragged rectangle.
func (a *App) drawRoofToolGhost(screen *ebiten.Image, x0, y0, x1, y1 int) {
	r := a.renderer
	w := r.world()
	walls := map[[2]int]bool{}
	for _, b := range w.Buildings {
		if b.Kind == "wall" || b.Kind == "door" {
			walls[[2]int{b.X, b.Y}] = true
		}
	}
	tile := r.tilePx()
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	for _, c := range placementCells(placeArea, x0, y0, x1, y1) {
		if !roofGhostUnsupported(roofCellAt(w, c[0], c[1]), walls[c]) {
			continue
		}
		sx, sy := r.worldToScreen(c[0], c[1])
		if visible(sx, sy, float32(tile), float32(tile), sw, sh) {
			r.drawRoofUnsupported(screen, c[0], c[1], tile) // roof_draw.go
		}
	}
}

// ---------------------------------------------------------- Home overlay (SIM-021)

const (
	homeButtonID = "hud.home"
	homeButtonW  = 48
)

// homeToolID reports id is one of the two Home Architect tools.
func homeToolID(id string) bool { return id == "home" || id == "unhome" }

// homeForced is whether the Home overlay must be forced on: a Home tool is
// in hand.
func homeForced(archOpen bool, toolID string) bool { return archOpen && homeToolID(toolID) }

// syncHomeForce keeps renderer.homeOverlayForced in step with the Architect.
func (a *App) syncHomeForce() {
	if a.renderer != nil {
		a.renderer.homeOverlayForced = homeForced(a.archOpen, a.archTool)
	}
}

// homeShown reports the Home overlay is showing (toggle or tool held).
func (r *Renderer) homeShown() bool { return r.homeOverlay || r.homeOverlayForced }

// toggleHomeOverlayIfAny is the HUD Home button action; it tolerates a nil
// renderer (menus).
func (r *Renderer) toggleHomeOverlayIfAny() {
	if r != nil {
		r.homeOverlay = !r.homeOverlay
	}
}

// homeButtons is the HUD Home toggle, immediately left of the Roof button.
// Clicks route through applyClick (id homeButtonID).
func (a *App) homeButtons(sw int) []button {
	rb := a.roofButtons(sw)
	if len(rb) == 0 {
		return nil
	}
	return []button{{id: homeButtonID, label: "Home", x: rb[0].x - homeButtonW - hudPad/2, y: 4, w: homeButtonW, h: hudTopH - 8, selected: a.renderer.homeShown()}}
}

func (a *App) drawHomeButton(screen *ebiten.Image, sw int) {
	mx, my := a.currentCursor()
	for _, b := range a.homeButtons(sw) {
		drawButton(screen, b, b.contains(mx, my))
		if img := a.icon("home"); img != nil {
			uiFillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtn, false)
			if b.selected {
				uiFillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtnSelected, false)
			}
			drawIcon(screen, img, float64(b.x)+(float64(b.w)-float64(b.h)+4)/2, float64(b.y)+2, float64(b.h)-4)
		}
	}
}

// homeLabelAt is "Home area" when the tile is in the Home area.
func homeLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return ""
	}
	if i := y*w.TerrainW + x; i < len(w.Home) && w.Home[i] != 0 {
		return "Home area"
	}
	return ""
}

// ---------------------------------------------------------- Plan overlay (PLAY-012)

const (
	planButtonID = "hud.plan"
	planButtonW  = 48
)

// planToolID reports id is one of the two Plan Architect tools.
func planToolID(id string) bool { return id == "plan" || id == "unplan" }

// planForced is whether the Plan overlay must be forced on: a Plan tool is
// in hand.
func planForced(archOpen bool, toolID string) bool { return archOpen && planToolID(toolID) }

// syncPlanForce keeps renderer.planForced in step with the Architect.
func (a *App) syncPlanForce() {
	if a.renderer != nil {
		a.renderer.planForced = planForced(a.archOpen, a.archTool)
	}
}

// planShown reports the Plan overlay is showing (default on, tool held).
func (r *Renderer) planShown() bool { return r.planOverlayOn() }

// togglePlanOverlayIfAny is the HUD Plans button action; it tolerates a nil
// renderer (menus).
func (r *Renderer) togglePlanOverlayIfAny() {
	if r != nil {
		r.planHidden = !r.planHidden
	}
}

// planButtons is the HUD Plans toggle, immediately left of the Home button.
func (a *App) planButtons(sw int) []button {
	hb := a.homeButtons(sw)
	if len(hb) == 0 {
		return nil
	}
	return []button{{id: planButtonID, label: "Plans", x: hb[0].x - planButtonW - hudPad/2, y: 4, w: planButtonW, h: hudTopH - 8, selected: a.renderer.planShown()}}
}

func (a *App) drawPlanButton(screen *ebiten.Image, sw int) {
	mx, my := a.currentCursor()
	for _, b := range a.planButtons(sw) {
		drawButton(screen, b, b.contains(mx, my))
		if img := a.icon("plan"); img != nil {
			uiFillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtn, false)
			if b.selected {
				uiFillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtnSelected, false)
			}
			drawIcon(screen, img, float64(b.x)+(float64(b.w)-float64(b.h)+4)/2, float64(b.y)+2, float64(b.h)-4)
		}
	}
}

// planLabelAt is "Planned" when the tile carries a plan mark.
func planLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return ""
	}
	if i := y*w.TerrainW + x; i < len(w.Plan) && w.Plan[i] != 0 {
		return "Planned"
	}
	return ""
}
