package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The lower-right global controls (PLAY-018: GLOBAL owns them). One panel,
// the layout's Global rect, holds a measured stack. Bottom-up from the
// tab strip:
//
//	overlay toggles   24 px icons in rows of 8, right-aligned
//	speed buttons     pause, normal, fast, superfast (52x24)
//	speed reason      only while the game paused itself (threat, background)
//	hour line         clock and the schedule's current activity
//	date line         the calendar date and the season
//	weather           the weather and the wind arrow
//	temperature       the hovered cell's air: "Indoors 18 °C"
//	conditions        one line per active condition (status_chips.go)
//	legends           the roof and power keys, while those overlays are on
//
// globalHeight() is that stack's measured height; the layout reserves it
// (hud.go a.hud()), so nothing here may call a.hud() while measuring.

const (
	globalPad     = 4  // inside the panel
	globalIconPx  = 24 // an overlay toggle
	globalIconGap = 4  // between toggles and between speed buttons
	globalPerRow  = 8  // toggles per row
	speedBtnW     = 52
	speedBtnH     = 24
	globalGap     = 4 // between the button rows and the text above
	globalInnerW  = rightW - 2*globalPad

	learnButtonID  = "hud.learn"
	barButtonID    = "hud.bar"
	rescatButtonID = "hud.rescat"
)

// globalToggle is one overlay toggle. The legacy five route through
// applyClick (app.go) as plain buttons; the settings toggles and the power
// grid toggle (run != nil) are widget-kit widgets, which applyClick
// answers through uiClick.
type globalToggle struct {
	id, icon, short string
	on              func(a *App) bool
	tip             func(a *App) string
	run             func(a *App)
}

// toggleGameplay flips one gameplay setting and saves it the way the
// Options screen does.
func (a *App) toggleGameplay(flip func(g *config.Gameplay)) {
	s := a.settings
	flip(&s.Gameplay)
	a.setSettings(s)
}

// globalToggleList is the toggles, left to right.
var globalToggleList = []globalToggle{
	{id: roofButtonID, icon: "overlay_roof", short: "Roof",
		on:  func(a *App) bool { return a.renderer.roofOverlayOn() },
		tip: func(a *App) string { return "Roof overlay" + a.keyHint(input.RoofOverlay) }},
	{id: homeButtonID, icon: "home", short: "Home",
		on:  func(a *App) bool { return a.renderer.homeShown() },
		tip: func(a *App) string { return "Home area overlay" }},
	{id: homeAutoButtonID, icon: "home_auto", short: "Auto",
		on: func(a *App) bool { return a.renderer.world().HomeAuto },
		tip: func(a *App) string {
			if a.renderer.world().HomeAuto {
				return "Home auto-expand: on (click to turn off)"
			}
			return "Home auto-expand: off (click to turn on)"
		}},
	{id: planButtonID, icon: "plan", short: "Plans",
		on:  func(a *App) bool { return a.renderer.planShown() },
		tip: func(a *App) string { return "Plans overlay" }},
	{id: roomsButtonID, icon: "overlay_rooms", short: "Rooms",
		on:  func(a *App) bool { return a.rooms },
		tip: func(a *App) string { return "Room stats overlay" + a.keyHint(input.RoomStats) }},
	{id: learnButtonID, icon: "learn", short: "Help",
		on: func(a *App) bool { return a.settings.Gameplay.ShowLearningHelper },
		tip: func(a *App) string {
			return "Learning helper" + stateSuffix(a.settings.Gameplay.ShowLearningHelper) + "\nShort lessons in the top-right corner. Click to show or hide them."
		},
		run: func(a *App) {
			a.toggleGameplay(func(g *config.Gameplay) { g.ShowLearningHelper = !g.ShowLearningHelper })
		}},
	{id: barButtonID, icon: "colonist", short: "Bar",
		on: func(a *App) bool { return a.settings.Gameplay.ShowColonistBar },
		tip: func(a *App) string {
			return "Colonist bar" + stateSuffix(a.settings.Gameplay.ShowColonistBar) + "\nThe portraits along the top. Click to show or hide them."
		},
		run: func(a *App) { a.toggleGameplay(func(g *config.Gameplay) { g.ShowColonistBar = !g.ShowColonistBar }) }},
	{id: rescatButtonID, icon: "zone_stockpile", short: "Res", // the list counts stockpiled goods
		on: func(a *App) bool { return a.settings.Gameplay.CategorizedResources },
		tip: func(a *App) string {
			return "Categorised resources" + stateSuffix(a.settings.Gameplay.CategorizedResources) + "\nGroup the resource list by category. Click to switch."
		},
		run: func(a *App) {
			a.toggleGameplay(func(g *config.Gameplay) { g.CategorizedResources = !g.CategorizedResources })
		}},
	// PLAY-021: the ninth, so it opens a second row (power_overlay.go).
	{id: powerButtonID, icon: "overlay_power", short: "Power",
		on:  func(a *App) bool { return a.renderer.powerShown() },
		tip: func(a *App) string { return a.powerToggleTip() },
		run: func(a *App) { a.renderer.togglePowerOverlayIfAny() }},
}

func stateSuffix(on bool) string {
	if on {
		return " (on)"
	}
	return " (off)"
}

// speedButtons are the four speed buttons, left to right.
var speedButtons = []struct {
	id, icon, short string
	speed           speed
	act             input.Action
	name            string
}{
	{"speed.0", "speed_pause", "||", speedPaused, input.Pause, "Pause / resume"},
	{"speed.1", "speed_normal", ">", speedNormal, input.Speed1, "Normal speed"},
	{"speed.2", "speed_fast", ">>", speedFast, input.Speed2, "Fast speed"},
	{"speed.3", "speed_superfast", ">>>", speedVeryFast, input.Speed3, "Superfast speed"},
}

// globalPlan is the stack's content, found without the layout.
type globalPlan struct {
	reason  string       // why the game is paused ("" none)
	weather bool         // a weather row
	chips   []statusChip // active conditions, one line each
	legend  []string     // the roof and power legends, wrapped (nil: none)
}

// textRowH is a text row of the stack: the body line, whose height already
// holds its leading.
func textRowH() float32 { return lineStep(tsBody) }

// globalPlan reads the stack's content from the world and the renderer.
func (a *App) globalPlan() globalPlan {
	if a.renderer == nil {
		return globalPlan{}
	}
	w := a.renderer.world()
	p := globalPlan{reason: a.speedReason(), weather: w.Weather != "", chips: statusChips(w)}
	if line := a.roofLegend(); line != "" {
		p.legend = wrapPx(line, tsBody, globalInnerW)
	}
	if line := a.powerLegend(); line != "" { // power_overlay.go
		p.legend = append(p.legend, wrapPx(line, tsBody, globalInnerW)...)
	}
	return p
}

// globalRowRects are the stack's rows. Text rows are the Global rect's
// inner width; a condition's rect is as wide as its label, flush right.
type globalRowRects struct {
	Toggles                           []rect
	Speed                             [4]rect
	Reason, Hour, Date, Weather, Temp rect
	Conds                             []rect
	Legend                            rect
	top                               float32 // where the stack ends, pad included
}

// layoutGlobal places the stack bottom-up from the bottom of g.
func layoutGlobal(g rect, p globalPlan, nToggles int) globalRowRects {
	var o globalRowRects
	text := textRowH()
	right := g.right() - globalPad
	y := g.bottom() - globalPad
	rows := (nToggles + globalPerRow - 1) / globalPerRow
	o.Toggles = make([]rect, nToggles)
	for row := rows - 1; row >= 0; row-- {
		y -= globalIconPx
		first, last := row*globalPerRow, min(nToggles, (row+1)*globalPerRow)
		x := right - float32(last-first)*globalIconPx - float32(last-first-1)*globalIconGap
		for i := first; i < last; i++ {
			o.Toggles[i] = rect{x, y, globalIconPx, globalIconPx}
			x += globalIconPx + globalIconGap
		}
		if row > 0 {
			y -= globalIconGap
		}
	}
	y -= globalGap + speedBtnH
	x := right - 4*speedBtnW - 3*globalIconGap
	for i := range o.Speed {
		o.Speed[i] = rect{x, y, speedBtnW, speedBtnH}
		x += speedBtnW + globalIconGap
	}
	y -= globalGap
	next := func() rect {
		y -= text
		return rect{g.X + globalPad, y, g.W - 2*globalPad, text}
	}
	if p.reason != "" {
		o.Reason = next()
	}
	o.Hour = next()
	o.Date = next()
	if p.weather {
		o.Weather = next()
	}
	o.Temp = next()
	o.Conds = make([]rect, len(p.chips))
	for i := len(p.chips) - 1; i >= 0; i-- {
		r := next()
		w := min(textW(p.chips[i].label, tsBody)+2*globalPad, g.W)
		o.Conds[i] = rect{g.right() - w, r.Y, w, r.H}
	}
	if n := len(p.legend); n > 0 {
		y -= float32(n) * text
		o.Legend = rect{g.X + globalPad, y, g.W - 2*globalPad, float32(n) * text}
	}
	o.top = y - globalPad
	return o
}

// globalStackHeight is the stack's measured height.
func globalStackHeight(p globalPlan, nToggles int) float32 {
	return -layoutGlobal(rect{0, 0, rightW, 0}, p, nToggles).top
}

// globalHeight is the lower-right controls' height the layout reserves
// (hud.go a.hud()); it never calls a.hud(). The minimap sits above Global,
// so the renderer's minimap learns the height here too.
func (a *App) globalHeight() float32 {
	if a.renderer == nil {
		return 0
	}
	h := globalStackHeight(a.globalPlan(), len(globalToggleList))
	a.renderer.mini.globalH = h
	return h
}

// globalRows is this frame's stack, placed in the layout's Global rect.
func (a *App) globalRows() globalRowRects {
	return layoutGlobal(a.hud().Global, a.globalPlan(), len(globalToggleList))
}

// minimapOn reports the layout should reserve the minimap's square.
func (a *App) minimapOn() bool { return a.renderer != nil && a.renderer.minimapOn() }

// globalToggleRect is the rect of toggle id.
func (a *App) globalToggleRect(id string) (rect, bool) {
	if a.renderer == nil {
		return rect{}, false
	}
	rows := a.globalRows()
	for i, tg := range globalToggleList {
		if tg.id == id {
			return rows.Toggles[i], true
		}
	}
	return rect{}, false
}

// toggleButton is toggle i as a button at r.
func (a *App) toggleButton(i int, r rect) button {
	tg := globalToggleList[i]
	return button{id: tg.id, label: tg.short, x: r.X, y: r.Y, w: r.W, h: r.H, selected: tg.on(a), tip: tg.tip(a)}
}

// globalButtons are the global controls' buttons: the legacy overlay
// toggles and the four speed buttons.
func (a *App) globalButtons(sw int) []button {
	if a.renderer == nil {
		return nil
	}
	rows := a.globalRows()
	var out []button
	for i, tg := range globalToggleList {
		if tg.run == nil {
			out = append(out, a.toggleButton(i, rows.Toggles[i]))
		}
	}
	for i, sb := range speedButtons {
		r := rows.Speed[i]
		out = append(out, button{id: sb.id, label: sb.short, x: r.X, y: r.Y, w: r.W, h: r.H,
			selected: a.renderer.speed == sb.speed, tip: sb.name + a.keyHint(sb.act)})
	}
	return out
}

// applySpeedClick handles a speed button ("speed.<n>"): the same intents
// the time keys use.
func (a *App) applySpeedClick(id string) {
	r := a.renderer
	if r == nil {
		return
	}
	for _, sb := range speedButtons {
		if sb.id != id {
			continue
		}
		if sb.speed == speedPaused {
			r.togglePause()
		} else {
			r.setSpeed(sb.speed)
		}
		return
	}
}

// globalUI declares the global controls' widget-kit parts: the settings
// toggles (clickable) and the hover cards of the text rows.
func (a *App) globalUI(sw, sh int) {
	if a.renderer == nil {
		return
	}
	rows := a.globalRows()
	u := a.uiSt()
	for i, tg := range globalToggleList {
		if tg.run == nil {
			continue
		}
		u.add(uiWidget{id: tg.id, label: tg.short, r: rows.Toggles[i], tip: tg.tip(a), click: func() { tg.run(a) }})
	}
	w := a.renderer.world()
	a.uiTip("hud.tip.hour", rows.Hour, "Time\n"+a.hudTimeText())
	a.uiTip("hud.tip.date", rows.Date, a.dateTip(w))
	if !rows.Weather.empty() {
		a.uiTip("hud.tip.weather", rows.Weather, "Weather\nThe sky and the wind (the arrow shows where it blows).")
	}
	a.uiTip("hud.tip.temp", rows.Temp, "Temperature\nThe air under the cursor: indoors inside a room, outdoors elsewhere. Over the HUD it shows the outdoor temperature.")
	if !rows.Reason.empty() {
		a.uiTip("hud.tip.reason", rows.Reason, "Paused\n"+a.speedReason())
	}
}

// ---------------------------------------------------------------- Text rows

// hudTimeText is the clock text with the speed: the date, the speed label
// (which says why the game paused itself) and the schedule's activity.
func (a *App) hudTimeText() string {
	return fmt.Sprintf("%s  %s  %s", a.renderer.date(), a.speedLabel(), a.renderer.activity())
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

// hourLine is the clock and the schedule's current activity.
func (a *App) hourLine() string {
	d := a.renderer.date()
	return fmt.Sprintf("%02d:%02d  %s · %s", d.Hour, d.Minute, d.Period(), a.renderer.activity())
}

// dateLine is the calendar date and the season.
func (a *App) dateLine() string {
	return dateSeasonLine(a.renderer.date(), a.renderer.world().Season, globalInnerW)
}

// dateSeasonLine is the date and the season in at most maxW px: the long
// form, else a compact one.
func dateSeasonLine(d clock.Date, season string, maxW float64) string {
	forms := []string{
		fmt.Sprintf("Day %d, Quadrum %d, Year %d", d.Day+1, d.Quadrum+1, d.Year+1),
		fmt.Sprintf("D%d Q%d Y%d", d.Day+1, d.Quadrum+1, d.Year+1),
	}
	var s string
	for _, f := range forms {
		s = f
		if season != "" {
			s += " · " + season
		}
		if w, _ := measure(s, tsBody); w <= maxW {
			return s
		}
	}
	return ellipsize(s, tsBody, maxW)
}

// dateTip is the date row's hover card: the biome and its climate, else
// the full date.
func (a *App) dateTip(w game.WorldView) string {
	lines := biomeTooltip(w)
	if len(lines) == 0 {
		return "Date\n" + a.renderer.date().String()
	}
	return seasonText(w) + "\n" + strings.Join(lines[1:], "\n")
}

// fmtDegrees is a temperature in tenths of a degree C as "18 °C",
// rounding half away from zero like fmtTemp.
func fmtDegrees(tenths int) string {
	if tenths < 0 {
		return fmt.Sprintf("%d °C", (tenths-5)/10)
	}
	return fmt.Sprintf("%d °C", (tenths+5)/10)
}

// temperatureLine is "Indoors 18 °C" or "Outdoors 9 °C" for the cell under
// the cursor; over the HUD (or off the map) it is the outdoor temperature.
func (a *App) temperatureLine() string {
	w := a.renderer.world()
	t, indoors := w.OutdoorTemp, false
	if tx, ty, ok := a.hoveredCell(); ok { // hud_cellinfo.go
		t = a.renderer.game.TemperatureAt(tx, ty)
		_, indoors = tileTemp(w, tx, ty)
	}
	where := "Outdoors"
	if indoors {
		where = "Indoors"
	}
	return where + " " + fmtDegrees(t)
}

// ---------------------------------------------------------------- Drawing

// rightText draws s right-aligned at x in the text row r.
func rightText(screen *ebiten.Image, s string, x float32, r rect, st textStyle) {
	st.Align = alignRight
	drawText(screen, s, float64(x), float64(r.Y), st)
}

// drawGlobalControls draws the Global panel and its stack.
func (a *App) drawGlobalControls(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	g := a.hud().Global
	if g.empty() {
		return
	}
	plan := a.globalPlan()
	rows := layoutGlobal(g, plan, len(globalToggleList))
	drawPanel(screen, g.X, g.Y, g.W, g.H)
	right := g.right() - globalPad
	w := a.renderer.world()
	mx, my := a.currentCursor()

	for i, l := range plan.legend {
		r := rect{rows.Legend.X, rows.Legend.Y + float32(i)*textRowH(), rows.Legend.W, textRowH()}
		rightText(screen, l, right, r, withColor(tsBody, uiTextDim))
	}
	drawConditionLines(screen, plan.chips, rows.Conds) // status_chips.go
	rightText(screen, a.temperatureLine(), right, rows.Temp, tsBody)
	if !rows.Weather.empty() {
		drawWeatherLabelRight(screen, int(right), int(rows.Weather.Y)+2, w) // environment.go: drawLabel lifts its text 2 px
	}
	rightText(screen, a.dateLine(), right, rows.Date, tsBody)
	rightText(screen, a.hourLine(), right, rows.Hour, tsBody)
	if !rows.Reason.empty() {
		rightText(screen, a.speedReason(), right, rows.Reason, withColor(tsBody, uiWarning))
	}
	for i, sb := range speedButtons {
		a.drawGlobalButton(screen, rows.Speed[i], sb.id, sb.icon, sb.short, a.renderer.speed == sb.speed, mx, my)
	}
	for i, tg := range globalToggleList {
		a.drawGlobalButton(screen, rows.Toggles[i], tg.id, tg.icon, tg.short, tg.on(a), mx, my)
	}
}

// drawGlobalButton draws a square or speed button: its icon, or its short
// label while the icon is not drawn yet.
func (a *App) drawGlobalButton(screen *ebiten.Image, r rect, id, icon, short string, on bool, mx, my int) {
	b := button{id: id, x: r.X, y: r.Y, w: r.W, h: r.H, selected: on}
	drawButton(screen, b, b.contains(mx, my))
	if img := a.icon(icon); img != nil && icon != "" {
		size := min(r.W, r.H) - 4
		drawIcon(screen, img, float64(r.X+(r.W-size)/2), float64(r.Y+(r.H-size)/2), float64(size))
		return
	}
	cx, cy := r.center()
	drawCentered(screen, short, float64(cx), float64(cy), tsSmall)
}

// ---------------------------------------------------------- Roof overlay (UI-010)

const (
	roofButtonID  = "hud.roof"
	roofLegendTxt = "Roof: blue built · slate natural · dashed planned · hatched removal"
)

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
	a.syncPowerForce()     // power_overlay.go (PLAY-021)
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

const homeButtonID = "hud.home"

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

const planButtonID = "hud.plan"

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
