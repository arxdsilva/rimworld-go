package ebiten

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// The selected zone's Expand and Shrink tools (PLAY-020), modelled on Move
// mode (move_mode.go) and the Architect's drag designators: the tool's icon
// follows the cursor with a hint, a left drag draws a rectangle and one
// command goes out when it is released. The tool stays on until right-click
// or Escape. The Expand command button starts it, Shrink likewise
// (cmds_zone.go); handleZoneTool is the map mouse's gate.

// zoneToolKind is which tool is active.
type zoneToolKind uint8

const (
	zoneToolNone zoneToolKind = iota
	zoneToolExpand
	zoneToolShrink
)

// zoneState is the selected zone's presentation state: the active tool and
// its drag, and the last clicked cell (App.zone).
type zoneState struct {
	tool           zoneToolKind
	fresh          bool // the tool started this frame: ignore the click that started it
	drag           bool
	x0, y0, x1, y1 int // the drag's two corners, in tiles

	// cell is the last clicked cell: when the selected zone's id is gone
	// (split, merged, deleted) the selection follows the zone holding it.
	cell    [2]int
	hasCell bool

	plants []int32 // the pane's plant lookup buffer (zonePlantsLine)
}

// zoneSt is the App's zone state (made on first use).
func (a *App) zoneSt() *zoneState {
	if a.zone == nil {
		a.zone = &zoneState{}
	}
	return a.zone
}

// noteCell records the last clicked cell.
func (s *zoneState) noteCell(c [2]int) { s.cell, s.hasCell = c, true }

// String is the tool's name in its button and hint.
func (k zoneToolKind) String() string {
	switch k {
	case zoneToolExpand:
		return "Expand"
	case zoneToolShrink:
		return "Shrink"
	}
	return ""
}

// zoneToolHint is the text that follows the cursor.
func zoneToolHint(k zoneToolKind, z game.ZoneView) string {
	what := "drag to add tiles"
	if k == zoneToolShrink {
		what = "drag to remove tiles"
	}
	return k.String() + " " + zoneTitle(z) + ": " + what + " · right-click or Esc: done"
}

// startZoneTool arms tool k on the selected zone. A click on the armed
// tool's button turns it off.
func (a *App) startZoneTool(k zoneToolKind) {
	st := a.zoneSt()
	if st.tool == k {
		st.tool, st.drag = zoneToolNone, false
		return
	}
	st.tool, st.drag, st.fresh = k, false, true
	a.archTool = "" // one tool at a time
	a.archDrag = false
	a.bldgMove.on = false
	a.closeFloat()
}

// zoneToolOn reports tool k is active.
func (a *App) zoneToolOn(k zoneToolKind) bool {
	st := a.zone
	return st != nil && st.tool == k
}

// handleZoneTool feeds the active tool this frame's input; true while a
// tool is active (the map mouse is the tool's, so selection must not also
// act on the click). Escape is the Esc chain's (escapeZoneTool).
func (a *App) handleZoneTool() bool {
	st := a.zone
	if st == nil || st.tool == zoneToolNone || a.renderer == nil {
		return false
	}
	z, ok := a.selectedZone()
	if !ok || a.archOpen || a.workOpen {
		st.tool, st.drag = zoneToolNone, false // the zone is gone, or another panel took over
		return false
	}
	f := &a.in
	if st.fresh { // the click that started the tool is not a drag
		st.fresh = false
		f.take(mbLeft)
		f.take(mbRight)
		return true
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	pmx, pmy := a.mapCursor()
	tx, ty := a.renderer.TileAt(pmx, pmy)
	onMap := !a.colonyChromeHit(mx, my, sw, sh)
	cancel := f.clicked(mbRight)
	press := onMap && f.clicked(mbLeft)
	if cancel {
		f.take(mbRight)
	}
	if press {
		f.take(mbLeft)
	}
	a.zoneToolStep(z, tx, ty, onMap, press, f.down(mbLeft), f.released(mbLeft), cancel)
	return true
}

// zoneToolStep is one frame of the tool on tile (tx, ty): press starts a
// drag, holding it moves the far corner, release sends the command, and
// cancel (right-click) drops the drag or, with none, ends the tool.
func (a *App) zoneToolStep(z game.ZoneView, tx, ty int, onMap, press, down, release, cancel bool) {
	st := a.zoneSt()
	switch {
	case cancel:
		if st.drag {
			st.drag = false
		} else {
			st.tool = zoneToolNone
		}
		return
	case press:
		st.drag = true
		st.x0, st.y0, st.x1, st.y1 = tx, ty, tx, ty
	}
	if st.drag && down && onMap {
		st.x1, st.y1 = tx, ty
	}
	if st.drag && release {
		if onMap {
			st.x1, st.y1 = tx, ty
		}
		a.submitZoneTool(z)
		st.drag = false
	}
}

// escapeZoneTool ends an active tool; true when it took the Esc.
func (a *App) escapeZoneTool() bool {
	st := a.zone
	if st == nil || st.tool == zoneToolNone {
		return false
	}
	st.tool, st.drag = zoneToolNone, false
	return true
}

// zoneToolRect is the tool's rectangle: the drag's corners, or the tile
// under the pointer when not dragging (ok false off the map).
func (a *App) zoneToolRect() (x0, y0, x1, y1 int, ok bool) {
	st := a.zoneSt()
	if st.drag {
		x0, y0, x1, y1 = normRect(st.x0, st.y0, st.x1, st.y1)
		return x0, y0, x1, y1, true
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) {
		return 0, 0, 0, 0, false
	}
	tx, ty := a.renderer.TileAt(a.mapCursor())
	return tx, ty, tx, ty, true
}

// zoneToolCells judges the cells under the tool for zone z, inside the
// visible tiles: Expand marks each cell green where the zone can claim it
// and red where not (CanZoneAt, as painting a zone does); Shrink lists, in
// red, only z's own cells.
func (a *App) zoneToolCells(z game.ZoneView) []ghostCell {
	st := a.zone
	if st == nil || st.tool == zoneToolNone || a.renderer == nil {
		return nil
	}
	x0, y0, x1, y1, ok := a.zoneToolRect()
	if !ok {
		return nil
	}
	r := a.renderer
	sw, sh := a.size()
	cx0, cy0 := r.TileAt(0, 0)
	cx1, cy1 := r.TileAt(int(float64(sw)*uiScale()), int(float64(sh)*uiScale()))
	clip := [4]int{cx0 - 1, cy0 - 1, cx1 + 1, cy1 + 1}
	if st.tool == zoneToolExpand {
		return zoneGhostCells(x0, y0, x1, y1, clip, func(x, y int) bool { return r.game.CanZoneAt(z.Kind, x, y) })
	}
	own := r.zoneGeomOf(z).cells
	var out []ghostCell
	for _, c := range zoneGhostCells(x0, y0, x1, y1, clip, func(x, y int) bool { return own[[2]int{x, y}] }) {
		if c.ok {
			out = append(out, ghostCell{x: c.x, y: c.y})
		}
	}
	return out
}

// submitZoneTool sends the dragged rectangle: Expand adds the valid cells
// to the zone, Shrink removes the zone's own cells in it. A rectangle that
// would change nothing sends nothing.
func (a *App) submitZoneTool(z game.ZoneView) {
	st := a.zoneSt()
	x0, y0, x1, y1 := normRect(st.x0, st.y0, st.x1, st.y1)
	r := a.renderer
	switch st.tool {
	case zoneToolExpand:
		if why, free := zoneDragRefusal(x0, y0, x1, y1, func(x, y int) string { return a.zoneRefusalAt(z.Kind, x, y) }); !free {
			a.hint = "nothing to add there"
			if why != zoneNoReason && why != game.ZoneRefuseTaken {
				a.hint += ": " + strings.ToLower(why) // PLAY-023: the most common refusal
			}
			return
		}
		r.enqueue(command.Command{Kind: command.ExpandZone, Target: uint64(z.ID), X: x0, Y: y0, X2: x1, Y2: y1})
	case zoneToolShrink:
		own := r.zoneGeomOf(z).cells
		if !anyZoneCell(x0, y0, x1, y1, func(x, y int) bool { return own[[2]int{x, y}] }) {
			a.hint = "none of this zone's tiles there"
			return
		}
		r.enqueue(command.Command{Kind: command.RemoveZoneCells, Target: uint64(z.ID), X: x0, Y: y0, X2: x1, Y2: y1})
	}
}

// ---- drawing ----

var (
	zoneAddClr    = color.NRGBA{R: 90, G: 200, B: 90, A: 64}
	zoneDenyClr   = color.NRGBA{R: 220, G: 70, B: 60, A: 64}
	zoneRemoveClr = color.NRGBA{R: 220, G: 70, B: 60, A: 110}
)

// drawZoneTool draws the active tool: the ghost cells on the map, then the
// cursor's icon and hint.
func (a *App) drawZoneTool(screen *ebiten.Image, z game.ZoneView) {
	st := a.zone
	if st == nil || st.tool == zoneToolNone {
		return
	}
	a.drawZoneToolGhost(screen, a.zoneToolCells(z), st.tool)
	a.drawZoneToolHint(screen, z, st.tool)
}

// drawZoneToolGhost tints the cells under the tool, in physical pixels
// like the map.
func (a *App) drawZoneToolGhost(screen *ebiten.Image, cells []ghostCell, k zoneToolKind) {
	tile := float32(a.renderer.tilePx())
	for _, c := range cells {
		clr := zoneDenyClr
		switch {
		case k == zoneToolShrink:
			clr = zoneRemoveClr
		case c.ok:
			clr = zoneAddClr
		}
		sx, sy := a.renderer.worldToScreen(c.x, c.y)
		vector.FillRect(screen, sx, sy, tile, tile, clr, false)
	}
}

// zoneToolIcon is the tool's cursor icon name.
func (a *App) zoneToolIcon(k zoneToolKind) string {
	if k == zoneToolShrink {
		return a.zoneIcon("zone_shrink", "plan_clear")
	}
	return a.zoneIcon("zone_expand", "zone_grow")
}

// zoneToolIconSize is the cursor icon's size (logical px).
const zoneToolIconSize = 24

// drawZoneToolHint draws the tool's icon and its hint beside the cursor
// (nothing over the HUD), in logical pixels.
func (a *App) drawZoneToolHint(screen *ebiten.Image, z game.ZoneView, k zoneToolKind) {
	mx, my := a.currentCursor()
	sw, sh := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) {
		return
	}
	msg := zoneToolHint(k, z)
	boxW, boxH := textW(msg, tsBody)+16, lineStep(tsBody)+4
	icon := a.icon(a.zoneToolIcon(k))
	iconW := float32(0)
	if icon != nil {
		iconW = zoneToolIconSize + 4
	}
	x := min(float32(mx+16), float32(sw)-iconW-boxW-4)
	y := min(float32(my+18), float32(sh)-boxH-4)
	if icon != nil {
		drawIcon(screen, icon, float64(x), float64(y+(boxH-zoneToolIconSize)/2), zoneToolIconSize)
	}
	uiFillRect(screen, x+iconW, y, boxW, boxH, uiPanel, false)
	drawText(screen, msg, float64(x+iconW+8), float64(y+2), tsBody)
}
