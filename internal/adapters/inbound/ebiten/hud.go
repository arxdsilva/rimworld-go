package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
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
}{
	{"tab.architect", "Architect", true},
	{"tab.work", "Work", true},
	{"tab.schedule", "Schedule", true},
	{"tab.assign", "Assign", false},
	{"tab.animals", "Animals", true},
	{"tab.research", "Research", true},
	{"tab.world", "World", false},
	{"tab.history", "History", false},
	{"tab.factions", "Factions", false},
	{"tab.menu", "Menu", true},
}

func hudTabY(sh int) float32 { return float32(sh - hudTabH) }

func hudBarY(sh int) float32 { return float32(sh - hudTabH - hudBarH) }

func pawnLabel(p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("Colonist %d", p.ID)
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
	pawns := a.renderer.snapshot()
	y := hudBarY(sh)
	out := make([]button, 0, len(pawns))
	for i, p := range pawns {
		sel := a.renderer.HasSelect() && a.renderer.Selected() == p.ID
		out = append(out, button{
			id:       "bar.pawn." + strconv.FormatUint(uint64(p.ID), 10),
			label:    "",
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
	out := a.floatButtons() // topmost first: hit-testing takes the first match
	out = append(out, a.roofButtons(sw)...)
	out = append(out, a.hudTabButtons(sw, sh)...)
	out = append(out, a.colonistBarButtons(sw, sh)...)
	out = append(out, a.inspectorButtons()...)
	out = append(out, a.orderQueueButtons()...)
	out = append(out, a.buildingButtons()...)
	out = append(out, a.animalButtons()...)
	out = append(out, a.alertButtons(sw)...)
	out = append(out, a.architectButtons()...)
	out = append(out, a.workButtons()...)
	return out
}

func (a *App) drawColonyHUD(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	a.syncRoofForce()
	a.drawArchitectGhost(screen)
	drawPanel(screen, 0, 0, float32(sw), hudTopH)
	x := hudPad
	wld := a.renderer.world()
	drawLabel(screen, x, 10, fmt.Sprintf("Food %d", wld.Food))
	x += 90
	drawLabel(screen, x, 10, fmt.Sprintf("Wood %d", wld.Wood))
	x += 90
	drawLabel(screen, x, 10, fmt.Sprintf("Stone %d", wld.Stone))
	x += 90
	if wld.Season != "" {
		season := fmt.Sprintf("%s  %s outside", wld.Season, fmtTemp(wld.OutdoorTemp))
		drawLabel(screen, x, 10, season)
		x += (len(season) + 3) * 6
	}
	drawWeatherLabel(screen, x, 10, wld)
	time := a.hudTimeText()
	drawLabel(screen, sw-len(time)*7-hudPad, 10, time)
	a.drawRoofButton(screen, sw)

	drawPanel(screen, 0, hudBarY(sh), float32(sw), hudBarH)
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.hudTabButtons(sw, sh), mx, my)
	a.drawButtons(screen, a.colonistBarButtons(sw, sh), mx, my)
	a.drawColonistPortraits(screen, sh)
	a.drawInspector(screen)
	a.drawOrderQueue(screen)
	a.drawBuildingInspector(screen)
	a.drawAnimalInspector(screen)
	a.drawAlerts(screen, sw)
	a.drawArchitect(screen)
	if a.workOpen {
		a.drawWork(screen, sw, sh)
	}
	a.drawFloat(screen)
	a.drawMessages(screen)
	a.drawHoverRoom(screen, sh)
}

func (a *App) drawColonistPortraits(screen *ebiten.Image, sh int) {
	pawns := a.renderer.snapshot()
	y := float64(hudBarY(sh) + 6)
	for i, p := range pawns {
		x := float64(hudPad + i*barSlot + 6)
		a.drawPawnPortrait(screen, p, x, y, 40, 48)
		if p.Drafted {
			drawIcon(screen, a.icon("draft"), x+24, y-2, 18)
		}
		vector.FillRect(screen, float32(x), float32(y)+46, 40*float32(p.Mood)/100, 3, moodColor(p.Mood), false)
		drawLabel(screen, int(x), int(hudBarY(sh))+hudBarH-14, pawnLabel(p))
	}
}

func (a *App) drawPawnPortrait(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) {
	if a.renderer.art != nil && a.renderer.drawPawnPortraitArt(screen, p, x, y, w, h) {
		return
	}
	if a.renderer.pack == nil {
		return
	}
	ss := a.renderer.pack.colonist(p.ID)
	if ss == nil {
		return
	}
	img := animFrame(ss.idle, a.renderer.present, uint64(idleAnimPeriod))
	drawImage(screen, img, x, y, w, h, false)
}

func (a *App) colonyChromeHit(mx, my, sw, sh int) bool {
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
	tx, ty := a.renderer.TileAt(mx, my)
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
	for _, lbl := range []string{roomLabelAt(w, tx, ty), roofLabelAt(w, tx, ty), plantLabelAt(w, tx, ty), itemLabelAt(w, tx, ty)} {
		if lbl != "" {
			lines = append(lines, lbl)
		}
	}
	drawHoverPanel(screen, sw, int(hudBarY(sh)), lines)
}

// drawHoverPanel draws lines in a right-aligned panel whose bottom edge sits
// just above bottomY. The first line is the title.
func drawHoverPanel(screen *ebiten.Image, sw, bottomY int, lines []string) {
	const lineH, pad = 16, 8
	wid := 0
	for _, l := range lines {
		if n := len(l) * 6; n > wid {
			wid = n
		}
	}
	pw, ph := wid+pad*2, len(lines)*lineH+pad*2-4
	px, py := sw-pw-hudPad, bottomY-ph-hudPad
	drawPanel(screen, float32(px), float32(py), float32(pw), float32(ph))
	for i, l := range lines {
		drawLabel(screen, px+pad, py+pad-2+i*lineH, l)
	}
}

// soilLines describes the ground on a tile: its type, fertility and whether
// crops can be grown there.
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
	lines := []string{s.Label, fmt.Sprintf("Fertility %d%%", s.Fertility)}
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

// itemLabelAt describes the ground stack on a tile ("Potatoes x 240").
func itemLabelAt(w game.WorldView, x, y int) string {
	for _, it := range w.Items {
		if it.X == x && it.Y == y && !it.Carried {
			name := it.Kind
			if it.Dried {
				name = "dried " + name
			}
			s := fmt.Sprintf("%s%s x %d", strings.ToUpper(name[:1]), name[1:], it.Qty)
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
	x := float32(sw-len(a.hudTimeText())*7-hudPad) - roofButtonW - hudPad
	return []button{{id: roofButtonID, label: "Roof", x: x, y: 4, w: roofButtonW, h: hudTopH - 8, selected: a.renderer.roofOverlayOn()}}
}

func (a *App) drawRoofButton(screen *ebiten.Image, sw int) {
	mx, my := a.currentCursor()
	for _, b := range a.roofButtons(sw) {
		drawButton(screen, b, b.contains(mx, my))
		if img := a.icon("overlay_roof"); img != nil {
			// Icon replaces the text label: blank it with the button fill, then draw.
			vector.FillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtn, false)
			if b.selected {
				vector.FillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtnSelected, false)
			}
			drawIcon(screen, img, float64(b.x)+(float64(b.w)-float64(b.h)+4)/2, float64(b.y)+2, float64(b.h)-4)
		}
		if line := a.roofLegend(); line != "" { // a tab under the bar, clear of the readouts
			lw := float32(len(line)*6 + 2*hudPad)
			lx := min(b.x+b.w, float32(sw-hudPad)) - lw
			drawPanel(screen, lx, hudTopH+2, lw, 22)
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

// toggleRoofOverlay is the R key and HUD button action.
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

// syncRoofForce keeps renderer.roofOverlayForced in step with the Architect.
func (a *App) syncRoofForce() {
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
