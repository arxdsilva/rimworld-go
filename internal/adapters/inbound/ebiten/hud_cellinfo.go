package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Cell info: what is under the cursor (PLAY-018: GLOBAL owns it; it moves
// into the inspect pane when nothing is selected).

// drawCellInfo draws the hovered cell's info.
func (a *App) drawCellInfo(screen *ebiten.Image, sh int) { a.drawHoverRoom(screen, sh) }

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
	if fl := unexploredLines(w, tx, ty); fl != nil { // fog_draw.go
		drawHoverPanel(screen, sw, int(hudBarY(sh)), fl)
		return
	}
	lines := soilLines(w, tx, ty)
	if len(lines) == 0 {
		return
	}
	if t, indoors := tileTemp(w, tx, ty); indoors {
		lines = append(lines, "Temperature "+fmtTemp(t)+" (indoors)")
	} else {
		lines = append(lines, "Temperature "+fmtTemp(t))
	}
	for _, lbl := range []string{roomLabelAt(w, tx, ty), homeLabelAt(w, tx, ty), planLabelAt(w, tx, ty), roofLabelAt(w, tx, ty), bridgeLabelAt(w, tx, ty), plantLabelAt(w, tx, ty), plantDamageLabelAt(w, tx, ty), grassLabelAt(w, tx, ty), fenceLabelAt(w, tx, ty), itemLabelAt(w, tx, ty)} {
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
