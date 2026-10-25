package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/input"
)

const (
	hudTopH = 32
	hudTabH = 32
	hudBarH = 56
	hudPad  = 8
	barSlot = 52
)

var hudTabSpecs = []struct {
	id, label string
	enabled   bool
	key       input.Action // its shortcut ("" none)
}{
	{"tab.architect", "Architect", true, input.TabArchitect},
	{"tab.work", "Work", true, input.TabWork},
	{"tab.schedule", "Schedule", true, input.TabSchedule},
	{"tab.assign", "Assign", true, input.TabAssign},
	{"tab.animals", "Animals", true, input.TabAnimals},
	{"tab.research", "Research", true, input.TabResearch},
	{"tab.world", "World", true, input.TabWorld},
	{"tab.history", "History", true, input.TabHistory},
	{"tab.factions", "Factions", true, ""},
	{"tab.menu", "Menu", true, input.Cancel},
}

func hudTabY(sh int) float32 { return float32(sh - hudTabH) }

func hudBarY(sh int) float32 { return float32(sh - hudTabH - hudBarH) }

func pawnLabel(p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("Colonist %d", p.ID)
}

// barTip is a colonist bar slot's hover line: the name, the age and the
// adulthood title ("Ada, 31, Convoy mechanic"); a legacy colonist has
// neither, so only the name.
func barTip(p game.PawnView) string {
	parts := []string{pawnLabel(p)}
	if p.AgeYears > 0 {
		parts = append(parts, strconv.Itoa(p.AgeYears))
	}
	if p.Title != "" && p.Title != "—" {
		parts = append(parts, p.Title)
	}
	return strings.Join(parts, ", ")
}

func (a *App) hudTabButtons(sw, sh int) []button {
	n := len(hudTabSpecs)
	if n < 1 || sw < 1 {
		return nil
	}
	w := float32(sw) / float32(n)
	y := hudTabY(sh)
	out := make([]button, 0, n)
	for i, t := range hudTabSpecs {
		b := button{
			id:    t.id,
			label: t.label,
			x:     float32(i) * w,
			y:     y,
			w:     w,
			h:     hudTabH,
		}
		if !t.enabled {
			b.disabled = true
			b.reason = "not implemented"
		}
		if t.key != "" {
			b.tip = t.label + a.keyHint(t.key)
		}
		if t.id == "tab.assign" && a.renderer != nil {
			b.selected = a.uiModalTop() == assignModalID
		}
		if t.id == "tab.history" && a.renderer != nil {
			b.selected = a.uiModalTop() == historyModalID
		}
		if t.id == "tab.factions" && a.renderer != nil {
			if len(a.renderer.game.Factions()) == 0 {
				b.disabled, b.reason = true, "no other peoples in this colony"
			} else {
				b.selected = a.uiModalTop() == colFacModalID
			}
		}
		if t.id == "tab.work" && a.workOpen && (a.panelMode == "work" || a.panelMode == "") {
			b.selected = true
		}
		if t.id == "tab.animals" && a.workOpen && a.panelMode == "animals" {
			b.selected = true
		}
		if t.id == "tab.research" && a.workOpen && a.panelMode == "research" {
			b.selected = true
		}
		if t.id == "tab.schedule" && a.workOpen && a.panelMode == "schedule" {
			b.selected = true
		}
		if t.id == "tab.architect" && a.archOpen {
			b.selected = true
		}
		out = append(out, b)
	}
	return out
}

func (a *App) colonistBarButtons(sw, sh int) []button {
	if a.renderer == nil {
		return nil
	}
	pawns := a.barPawns()
	y := hudBarY(sh)
	out := make([]button, 0, len(pawns))
	for i, p := range pawns {
		sel := a.renderer.HasSelect() && a.renderer.Selected() == p.ID || a.inGroup(selPawn, p.ID)
		out = append(out, button{
			id:       "bar.pawn." + strconv.FormatUint(uint64(p.ID), 10),
			label:    "",
			tip:      barTip(p),
			x:        float32(hudPad + i*barSlot),
			y:        y + 4,
			w:        barSlot - 4,
			h:        hudBarH - 8,
			selected: sel,
		})
	}
	return out
}

func (a *App) colonyButtons(sw, sh int) []button {
	if a.hideUI {
		return nil // F11: only the map
	}
	out := a.floatButtons() // topmost first: hit-testing takes the first match
	out = append(out, a.devButtons()...)
	out = append(out, a.searchButtons(sw)...)
	out = append(out, a.roofButtons(sw)...)
	out = append(out, a.homeButtons(sw)...)
	out = append(out, a.planButtons(sw)...)
	out = append(out, a.roomsButtons(sw)...)
	out = append(out, a.groupButtons()...)
	out = append(out, a.hudTabButtons(sw, sh)...)
	out = append(out, a.colonistBarButtons(sw, sh)...)
	out = append(out, a.inspectorButtons()...)
	out = append(out, a.orderQueueButtons()...)
	out = append(out, a.buildingButtons()...)
	out = append(out, a.animalButtons()...)
	out = append(out, a.plantButtons()...)
	out = append(out, a.itemButtons()...)
	out = append(out, a.frameButtons()...)
	out = append(out, a.alertButtons(sw)...)
	out = append(out, a.messageButtons(sw)...)
	out = append(out, a.architectButtons()...)
	out = append(out, a.workButtons()...)
	return out
}

func (a *App) drawColonyHUD(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	a.syncRoofForce()
	a.drawRoomOverlay(screen) // room_stats.go
	a.drawGroupMarks(screen)  // multiselect.go
	a.drawFormation(screen)   // formation.go
	a.drawBoxSelect(screen)
	a.drawArchitectGhost(screen)
	a.drawMoveMarkers(screen) // move_mode.go: pending moves
	a.drawMoveMode(screen)
	drawPanel(screen, 0, 0, float32(sw), hudTopH)
	x := hudPad
	wld := a.renderer.world()
	for _, l := range resourceReadouts(wld) { // Food, Wood, Stone, Steel (+ Silver, Gold)
		drawLabel(screen, x, 10, l)
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
	a.drawPlanButton(screen, sw)
	a.drawRoomsButton(screen, sw)

	drawPanel(screen, 0, hudBarY(sh), float32(sw), hudBarH)
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.hudTabButtons(sw, sh), mx, my)
	a.drawButtons(screen, a.colonistBarButtons(sw, sh), mx, my)
	a.drawColonistPortraits(screen, sh)
	a.drawInspector(screen)
	a.drawOrderQueue(screen)
	a.drawBuildingInspector(screen)
	a.drawAnimalInspector(screen)
	a.drawPlantInspector(screen)
	a.drawItemInspector(screen)
	a.drawFrameInspector(screen)
	a.drawGroupInspector(screen)
	a.drawGizmoKeys(screen) // gizmo.go
	a.drawAlerts(screen, sw)
	a.drawGatheringBanner(screen, sw) // gathering_ui.go (ENG-003)
	a.drawStatusChips(screen, sw)     // status_chips.go: heat wave, cold snap, conditions
	a.drawArchitect(screen)
	if a.workOpen {
		a.drawWork(screen, sw, sh)
	}
	a.drawFloat(screen)
	a.drawMessages(screen)
	a.drawHoverRoom(screen, sh)
	a.drawRoomCard(screen)
	a.drawDevPanels(screen, sw, sh) // devmode.go
	a.drawSearch(screen, sw)        // search_ui.go
}

// barPawns are the colonists the colonist bar shows: everyone but
// prisoners (who stay selectable on the map).
func (a *App) barPawns() []game.PawnView {
	pawns := a.renderer.snapshot()
	for i, p := range pawns {
		if p.Prisoner {
			out := append(make([]game.PawnView, 0, len(pawns)), pawns[:i]...)
			for _, q := range pawns[i+1:] {
				if !q.Prisoner {
					out = append(out, q)
				}
			}
			return out
		}
	}
	return pawns
}

func (a *App) drawColonistPortraits(screen *ebiten.Image, sh int) {
	pawns := a.barPawns()
	y := float64(hudBarY(sh) + 6)
	for i, p := range pawns {
		x := float64(hudPad + i*barSlot + 6)
		a.drawPawnPortrait(screen, p, x, y, 40, 48)
		if p.Drafted {
			drawIcon(screen, a.icon("draft"), x+24, y-2, 18)
		}
		uiFillRect(screen, float32(x), float32(y)+46, 40*float32(p.Mood)/100, 3, moodColor(p.Mood), false)
		drawText(screen, ellipsize(pawnLabel(p), tsSmall, barSlot-4), x, float64(hudBarY(sh))+hudBarH-lineH(tsSmall)-1, tsSmall)
	}
}

func (a *App) drawPawnPortrait(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) {
	if a.renderer.art != nil {
		a.renderer.drawPawnPortraitArt(screen, p, x, y, w, h)
	}
}

func (a *App) colonyChromeHit(mx, my, sw, sh int) bool {
	if a.hideUI {
		return false
	}
	if a.search.open && a.searchRect(sw).contains(mx, my) {
		return true
	}
	if a.dev.menu && a.devMenuRect().contains(mx, my) {
		return true
	}
	if a.grouped() && !a.workOpen && !a.archOpen && groupRect(len(a.groupRows())).contains(mx, my) {
		return true
	}
	if my < hudTopH {
		return true
	}
	if a.float.open && a.floatRect().contains(mx, my) {
		return true
	}
	if my >= int(hudBarY(sh)) {
		return true
	}
	if a.workOpen && workRect(sw, sh).contains(mx, my) {
		return true
	}
	if a.archOpen && a.architectRect().contains(mx, my) {
		return true
	}
	if a.renderer != nil && a.renderer.HasSelect() && !a.workOpen && inspectorRect().contains(mx, my) {
		return true
	}
	if p, ok := a.selectedPawn(); ok && len(p.QueuedOrders) > 0 && !a.workOpen && orderQueueRect(len(p.QueuedOrders)).contains(mx, my) {
		return true
	}
	if a.hasBuilding && !a.workOpen && !a.archOpen && buildingRect().contains(mx, my) {
		return true
	}
	if _, ok := a.selectedFrame(); ok && !a.workOpen && !a.archOpen && frameRect().contains(mx, my) {
		return true
	}
	return false
}

// drawHoverRoom shows what is under the cursor (soil, room, plant, items)
// as a panel in the bottom-right corner, above the colonist bar.
func (a *App) drawHoverRoom(screen *ebiten.Image, sh int) {
	if a.renderer == nil {
		return
	}
	mx, my := a.currentCursor()
	sw, _ := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) {
		return
	}
	tx, ty := a.renderer.TileAt(a.mapCursor()) // physical: the map's pixels
	w := a.renderer.world()
	lines := soilLines(w, tx, ty)
	if len(lines) == 0 {
		return
	}
	if t, indoors := tileTemp(w, tx, ty); indoors {
		lines = append(lines, "Temperature "+fmtTemp(t)+" (indoors)")
	} else {
		lines = append(lines, "Temperature "+fmtTemp(t))
	}
	for _, lbl := range []string{roomLabelAt(w, tx, ty), homeLabelAt(w, tx, ty), planLabelAt(w, tx, ty), roofLabelAt(w, tx, ty), plantLabelAt(w, tx, ty), grassLabelAt(w, tx, ty), fenceLabelAt(w, tx, ty), itemLabelAt(w, tx, ty)} {
		if lbl != "" {
			lines = append(lines, lbl)
		}
	}
	if a.in.alt() { // Alt held: the detailed cell lines (room_stats.go)
		lines = append(lines, detailLines(w, tx, ty)...)
	}
	drawHoverPanel(screen, sw, int(hudBarY(sh)), lines)
}

// drawHoverPanel draws lines in a right-aligned panel whose bottom edge sits
// just above bottomY. The first line is the title.
func drawHoverPanel(screen *ebiten.Image, sw, bottomY int, lines []string) {
	const pad = 8
	step := int(lineStep(tsBody))
	wid := int(maxTextW(lines, tsBody))
	pw, ph := wid+pad*2, len(lines)*step+pad*2-4
	px, py := sw-pw-hudPad, bottomY-ph-hudPad
	drawPanel(screen, float32(px), float32(py), float32(pw), float32(ph))
	for i, l := range lines {
		drawLabel(screen, px+pad, py+pad-2+i*step, l)
	}
}

// soilLines describes the ground on a tile: its type (with its stone or
// ore), fertility, whether crops can be grown there and whether it can be
// smoothed.
func soilLines(w game.WorldView, x, y int) []string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return nil
	}
	i := y*w.TerrainW + x
	if i >= len(w.Terrain) {
		return nil
	}
	soils := game.Soils()
	c := int(w.Terrain[i])
	if c >= len(soils) {
		return nil
	}
	s := soils[c]
	stone, ore := "", ""
	if i < len(w.StoneType) && int(w.StoneType[i]) < len(stoneNames) {
		stone = stoneNames[w.StoneType[i]]
	}
	if i < len(w.Ore) && int(w.Ore[i]) < len(oreNames) {
		ore = oreNames[w.Ore[i]]
	}
	label := game.SoilLabel(s.Name, stone, ore) // "Granite rock wall", "Compacted steel (in granite)"
	if label == "" {
		label = s.Label
	}
	lines := []string{label, fmt.Sprintf("Fertility %d%%", s.Fertility)}
	switch {
	case !s.Walkable:
		lines = append(lines, "Impassable, mine to clear")
	case !s.Farmable:
		lines = append(lines, "Not farmable")
	case s.Fertility > 100:
		lines = append(lines, fmt.Sprintf("Farmable, crops grow %d%% faster", s.Fertility-100))
	case s.Fertility < 100:
		lines = append(lines, fmt.Sprintf("Farmable, crops grow %d%% slower", 100-s.Fertility))
	default:
		lines = append(lines, "Farmable, normal growth")
	}
	if s.Smoothable && ore == "" {
		lines = append(lines, "Smoothable")
	}
	return lines
}

// plantLabelAt describes the plant on a tile ("Corn, 62% grown").
func plantLabelAt(w game.WorldView, x, y int) string {
	for _, p := range w.Plants {
		if p.X != x || p.Y != y {
			continue
		}
		name := strings.ReplaceAll(p.Species, "_", " ")
		name = strings.ToUpper(name[:1]) + name[1:]
		switch {
		case p.Harvestable:
			return name + ", ready to harvest"
		default:
			return fmt.Sprintf("%s, %d%% grown", name, p.Growth)
		}
	}
	return ""
}

// grassLabelAt is the hover line of a growing grass tile ("Grass
// (growing, 40%)"); empty for full, absent or cut grass.
func grassLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return ""
	}
	i := y*w.TerrainW + x
	if i >= len(w.Grass) || w.Grass[i] == 0 || i >= len(w.GrassGrowth) || w.GrassGrowth[i] >= 100 {
		return ""
	}
	return fmt.Sprintf("Grass (growing, %d%%)", w.GrassGrowth[i])
}

// itemLabelAt describes the ground stack on a tile ("Potatoes x 240").
func itemLabelAt(w game.WorldView, x, y int) string {
	for _, it := range w.Items {
		if it.X == x && it.Y == y && !it.Carried {
			name := it.Kind
			if it.Dried {
				name = "dried " + name
			}
			s := fmt.Sprintf("%s%s x %d", strings.ToUpper(name[:1]), name[1:], it.Qty)
			if it.Forbidden {
				s += ", forbidden"
			}
			if f := freshLabel(it); f != "" {
				s += ", " + f
			}
			return s
		}
	}
	return ""
}

// ---------------------------------------------------------- Roof overlay (UI-010)

const (
	roofButtonID  = "hud.roof"
	roofButtonW   = 48
	roofLegendTxt = "Roof: blue built · slate natural · dashed planned · hatched removal"
)

// hudTimeText is the right-aligned clock text of the top strip.
func (a *App) hudTimeText() string {
	return fmt.Sprintf("%s  %s  %s", a.renderer.date(), a.renderer.speedRate(), a.renderer.activity())
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
