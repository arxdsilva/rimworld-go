package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Cell info (PLAY-018: GLOBAL owns it): what is under the cursor, as plain
// text on a subtle translucent backing at the lower-left, built bottom-up
// from (8, Tabs.Y - 8). It is not chrome (a click on it reaches the map)
// and shows only while nothing is selected and no Architect or window is
// open; it hides when the cursor is over the HUD.

const (
	cellInfoPad     = 6
	cellInfoBacking = 0x70 // the backing's alpha over the map
)

// nothingSelected reports no pawn, animal, item, plant, building, frame,
// zone or group is selected.
func (a *App) nothingSelected() bool {
	return a.renderer != nil && !a.grouped() && !hasPawnSel(a) &&
		!a.sel.HasAnimal && !a.sel.HasItem && !a.sel.HasPlant && !a.sel.HasBuilding && !a.sel.HasFrame && !a.sel.HasZone
}

// hoveredCell is the map cell under the cursor: not over the HUD, on the
// map.
func (a *App) hoveredCell() (tx, ty int, ok bool) {
	if a.renderer == nil {
		return 0, 0, false
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) {
		return 0, 0, false
	}
	if m := a.hud().Minimap; !m.Empty() && m.Hit(mx, my) { // not chrome (own click path), but not the map either
		return 0, 0, false
	}
	tx, ty = a.shell.TileAt(a.mapCursor()) // physical: the map's pixels
	mw, mh := a.shell.MapSize()
	if tx < 0 || ty < 0 || tx >= mw || ty >= mh {
		return 0, 0, false
	}
	return tx, ty, true
}

// cellInfoLines are the hovered cell's lines: the ground, its fertility,
// the temperature, and what stands or lies there. Nil when cell info is
// not showing.
func (a *App) cellInfoLines() []string {
	if a.renderer == nil || a.hideUI || a.arch.Open || a.win.Open || !a.nothingSelected() {
		return nil
	}
	if !a.hud().Window.Empty() {
		return nil
	}
	tx, ty, ok := a.hoveredCell()
	if !ok {
		return nil
	}
	w := a.shell.World()
	if fl := unexploredLines(w, tx, ty); fl != nil { // fog_draw.go
		return fl
	}
	lines := soilLines(w, tx, ty)
	if len(lines) == 0 {
		return nil
	}
	if _, indoors := tileTemp(w, tx, ty); indoors {
		lines = append(lines, "Temperature "+fmtTemp(a.renderer.game.TemperatureAt(tx, ty))+" (indoors)")
	} else {
		lines = append(lines, "Temperature "+fmtTemp(a.renderer.game.TemperatureAt(tx, ty)))
	}
	for _, lbl := range []string{roomLabelAt(w, tx, ty), homeLabelAt(w, tx, ty), planLabelAt(w, tx, ty), roofLabelAt(w, tx, ty), bridgeLabelAt(w, tx, ty), plantLabelAt(w, tx, ty), plantDamageLabelAt(w, tx, ty), grassLabelAt(w, tx, ty), fenceLabelAt(w, tx, ty), itemLabelAt(w, tx, ty)} {
		if lbl != "" {
			lines = append(lines, lbl)
		}
	}
	if a.in.Alt() { // Alt held: the detailed cell lines (room_stats.go)
		lines = append(lines, detailLines(w, tx, ty)...)
	}
	return lines
}

// cellInfoRect is where cell info draws: its lines' box with the bottom-left
// corner at (8, Tabs.Y - 8). Empty while it is not showing.
func (a *App) cellInfoRect() rect {
	lines := a.cellInfoLines()
	if len(lines) == 0 {
		return rect{}
	}
	return cellInfoBox(lines, a.hud().Tabs.Y)
}

// cellInfoBox is the box of lines whose bottom edge sits hudMargin above
// tabsY.
func cellInfoBox(lines []string, tabsY float32) rect {
	w := maxTextW(lines, tsLabel) + 2*cellInfoPad
	h := float32(len(lines))*lineStep(tsBody) + 2*cellInfoPad
	return rect{X: hudMargin, Y: tabsY - hudMargin - h, W: w, H: h}
}

// drawCellInfo draws the hovered cell's lines.
func (a *App) drawCellInfo(screen *ebiten.Image, sh int) {
	lines := a.cellInfoLines()
	if len(lines) == 0 {
		return
	}
	r := cellInfoBox(lines, a.hud().Tabs.Y)
	back := uiDim
	back.A = cellInfoBacking
	uiFillRect(screen, r.X, r.Y, r.W, r.H, back, false)
	step := float64(lineStep(tsBody))
	for i, l := range lines {
		st := tsBody
		if i == 0 {
			st = withColor(tsLabel, uiHeading)
		}
		drawText(screen, l, float64(r.X+cellInfoPad), float64(r.Y+cellInfoPad)+float64(i)*step, st)
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

// hasPawnSel reports a pawn is selected.
func hasPawnSel(a *App) bool {
	_, ok := a.shell.Selected()
	return ok
}
