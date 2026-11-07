package hud

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"strings"

	"rimworld-go/internal/application/game"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Cell info (PLAY-018: GLOBAL owns it; UIS-059): what is under the
// cursor, as plain text with no backing plate at the lower-left, CellX from
// the window's left edge, lines CellPitch apart, the last line's baseline
// CellBottom above the tab strip. It is not chrome (a click on it reaches
// the map) and shows only while nothing is selected and no Architect or
// window is open; it hides when the cursor is over the HUD.

// hovered is the map cell under the cursor: not over the HUD, on the
// map.
func (c CellInfo) hovered() (tx, ty int, ok bool) {
	if !c.live() {
		return 0, 0, false
	}
	mx, my := c.sh.Cursor()
	sw, sh := c.sh.Size()
	if c.sh.ChromeHit(mx, my, sw, sh) {
		return 0, 0, false
	}
	if m := c.sh.Layout().Minimap; !m.Empty() && m.Hit(mx, my) { // not chrome (own click path), but not the map either
		return 0, 0, false
	}
	tx, ty = c.sh.TileAt(c.sh.MapCursor()) // physical: the map's pixels
	mw, mh := c.sh.MapSize()
	if tx < 0 || ty < 0 || tx >= mw || ty >= mh {
		return 0, 0, false
	}
	return tx, ty, true
}

// Lines are the hovered cell's lines: the ground, its fertility,
// the temperature, and what stands or lies there. Nil when cell info is
// not showing.
func (c CellInfo) Lines() []string {
	e := c.sh.Env()
	if e.Game == nil || c.Hidden || e.PanelOpen || !e.NothingSelected {
		return nil
	}
	if !c.sh.Layout().Window.Empty() {
		return nil
	}
	tx, ty, ok := c.hovered()
	if !ok {
		return nil
	}
	w := c.sh.World()
	if fl := overlay.UnexploredLines(w, tx, ty); fl != nil { // fog_draw.go
		return fl
	}
	lines := SoilLines(w, tx, ty)
	if len(lines) == 0 {
		return nil
	}
	if _, indoors := tileTemp(w, tx, ty); indoors {
		lines = append(lines, "Temperature "+FmtTemp(e.Units, e.Game.TemperatureAt(tx, ty))+" (indoors)")
	} else {
		lines = append(lines, "Temperature "+FmtTemp(e.Units, e.Game.TemperatureAt(tx, ty)))
	}
	for _, lbl := range []string{world.RoomLabelAt(w, tx, ty), HomeLabelAt(w, tx, ty), PlanLabelAt(w, tx, ty), overlay.RoofLabelAt(w, tx, ty), overlay.BridgeLabelAt(w, tx, ty), plantLabelAt(w, tx, ty), env.PlantDamageLabelAt(w, tx, ty), GrassLabelAt(w, tx, ty), world.FenceLabelAt(w, tx, ty), itemLabelAt(w, tx, ty)} {
		if lbl != "" {
			lines = append(lines, lbl)
		}
	}
	if e.Alt { // Alt held: the detailed cell lines (rooms.go)
		lines = append(lines, DetailLines(w, tx, ty)...)
	}
	return lines
}

// Rect is where cell info draws: its lines' box (CellInfoBox). Empty while
// it is not showing.
func (c CellInfo) Rect() gfx.Rect {
	lines := c.Lines()
	if len(lines) == 0 {
		return gfx.Rect{}
	}
	return CellInfoBox(lines, c.sh.Layout().Tabs.Y)
}

// cellLineTop is the top of line i of n, the last one's baseline
// CellBottom above tabsY.
func cellLineTop(i, n int, tabsY float32) float32 {
	base := tabsY - hudlayout.CellBottom - float32(n-1-i)*hudlayout.CellPitch
	return base - float32(gfx.Body.Size*notoAscent)
}

// CellInfoBox is the lines' ink box: from the first line's top to the last
// line's descent, CellX from the left edge, as wide as the widest line.
func CellInfoBox(lines []string, tabsY float32) gfx.Rect {
	n := len(lines)
	top := cellLineTop(0, n, tabsY)
	bottom := cellLineTop(n-1, n, tabsY) + float32(gfx.LineH(gfx.Body))
	return gfx.Rect{X: hudlayout.CellX, Y: top, W: gfx.MaxTextW(lines, gfx.Body), H: bottom - top}
}

// Draw draws the hovered cell's lines, all in the body style.
func (c CellInfo) Draw(screen *ebiten.Image, sh int) {
	lines := c.Lines()
	if len(lines) == 0 {
		return
	}
	tabsY := c.sh.Layout().Tabs.Y
	for i, l := range lines {
		gfx.Text(screen, l, hudlayout.CellX, float64(cellLineTop(i, len(lines), tabsY)), gfx.Body)
	}
}

// SoilLines describes the ground on a tile: its type (with its stone or
// ore), fertility, whether crops can be grown there and whether it can be
// smoothed.
func SoilLines(w game.WorldView, x, y int) []string {
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
	if i < len(w.StoneType) && int(w.StoneType[i]) < len(sprite.StoneNames) {
		stone = sprite.StoneNames[w.StoneType[i]]
	}
	if i < len(w.Ore) && int(w.Ore[i]) < len(sprite.OreNames) {
		ore = sprite.OreNames[w.Ore[i]]
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

// GrassLabelAt is the hover line of a growing grass tile ("Grass
// (growing, 40%)"); empty for full, absent or cut grass.
func GrassLabelAt(w game.WorldView, x, y int) string {
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
