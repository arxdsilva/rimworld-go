package architect

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// The selected zone's Expand and Shrink tools (PLAY-020), modelled on Move
// mode (move_mode.go) and the Architect's drag designators: the tool's icon
// follows the cursor with a hint, a left drag draws a rectangle and one
// command goes out when it is released. The tool stays on until right-click
// or Escape. The Expand command button starts it, Shrink likewise (the
// root's cmds_zone.go); Handle is the map mouse's gate.

// ZoneToolKind is which tool is active.
type ZoneToolKind uint8

// The zone tools.
const (
	ZoneToolNone ZoneToolKind = iota
	ZoneToolExpand
	ZoneToolShrink
)

// ZoneState is the selected zone's presentation state: the active tool
// and its drag, and the last clicked cell.
type ZoneState struct {
	Kind           ZoneToolKind
	Fresh          bool // the tool started this frame: ignore the click that started it
	Drag           bool
	X0, Y0, X1, Y1 int // the drag's two corners, in tiles

	// cell is the last clicked cell: when the selected zone's id is gone
	// (split, merged, deleted) the selection follows the zone holding it.
	cell    [2]int
	hasCell bool

	plants []int32 // the pane's plant lookup buffer (plantsLine)
}

// State is the zone tools' state (a capture poses a drag through it).
func (z *Zones) State() *ZoneState { return &z.st }

// NoteCell records the last clicked cell.
func (z *Zones) NoteCell(c [2]int) { z.st.cell, z.st.hasCell = c, true }

// String is the tool's name in its button and hint.
func (k ZoneToolKind) String() string {
	switch k {
	case ZoneToolExpand:
		return "Expand"
	case ZoneToolShrink:
		return "Shrink"
	}
	return ""
}

// ZoneToolHint is the text that follows the cursor.
func ZoneToolHint(k ZoneToolKind, z game.ZoneView) string {
	what := "drag to add tiles"
	if k == ZoneToolShrink {
		what = "drag to remove tiles"
	}
	return k.String() + " " + zoneTitle(z) + ": " + what + " · right-click or Esc: done"
}

// Start arms tool k on the selected zone. A click on the armed tool's
// button turns it off.
func (z *Zones) Start(k ZoneToolKind) {
	st := &z.st
	if st.Kind == k {
		st.Kind, st.Drag = ZoneToolNone, false
		return
	}
	st.Kind, st.Drag, st.Fresh = k, false, true
	z.ToolID = "" // one tool at a time
	z.Tool.Drag = false
	z.Mover.st.On = false
	z.sh.CloseFloat()
}

// On reports tool k is active.
func (z *Zones) On(k ZoneToolKind) bool { return z.st.Kind == k && k != ZoneToolNone }

// Handle feeds the active tool this frame's input; true while a tool is
// active (the map mouse is the tool's, so selection must not also act on
// the click). Escape is the Esc chain's (Escape).
func (z *Zones) Handle() bool {
	st := &z.st
	if st.Kind == ZoneToolNone || z.sim() == nil {
		return false
	}
	zv, ok := z.Selected()
	if !ok || z.Open || z.sh.Env().WinOpen {
		st.Kind, st.Drag = ZoneToolNone, false // the zone is gone, or another panel took over
		return false
	}
	f := z.frame()
	if st.Fresh { // the click that started the tool is not a drag
		st.Fresh = false
		f.Take(inputframe.Left)
		f.Take(inputframe.Right)
		return true
	}
	mx, my := z.sh.Cursor()
	sw, sh := z.sh.Size()
	pmx, pmy := z.sh.MapCursor()
	tx, ty := z.sh.TileAt(pmx, pmy)
	onMap := !z.sh.ChromeHit(mx, my, sw, sh)
	cancel := f.Clicked(inputframe.Right)
	press := onMap && f.Clicked(inputframe.Left)
	if cancel {
		f.Take(inputframe.Right)
	}
	if press {
		f.Take(inputframe.Left)
	}
	z.step(zv, tx, ty, onMap, press, f.Down(inputframe.Left), f.Released(inputframe.Left), cancel)
	return true
}

// step is one frame of the tool on tile (tx, ty): press starts a drag,
// holding it moves the far corner, release sends the command, and cancel
// (right-click) drops the drag or, with none, ends the tool.
func (z *Zones) step(zv game.ZoneView, tx, ty int, onMap, press, down, release, cancel bool) {
	st := &z.st
	switch {
	case cancel:
		if st.Drag {
			st.Drag = false
		} else {
			st.Kind = ZoneToolNone
		}
		return
	case press:
		st.Drag = true
		st.X0, st.Y0, st.X1, st.Y1 = tx, ty, tx, ty
	}
	if st.Drag && down && onMap {
		st.X1, st.Y1 = tx, ty
	}
	if st.Drag && release {
		if onMap {
			st.X1, st.Y1 = tx, ty
		}
		z.submit(zv)
		st.Drag = false
	}
}

// Escape ends an active tool; true when it took the Esc.
func (z *Zones) Escape() bool {
	st := &z.st
	if st.Kind == ZoneToolNone {
		return false
	}
	st.Kind, st.Drag = ZoneToolNone, false
	return true
}

// toolRect is the tool's rectangle: the drag's corners, or the tile under
// the pointer when not dragging (ok false off the map).
func (z *Zones) toolRect() (x0, y0, x1, y1 int, ok bool) {
	st := &z.st
	if st.Drag {
		x0, y0, x1, y1 = normRect(st.X0, st.Y0, st.X1, st.Y1)
		return x0, y0, x1, y1, true
	}
	mx, my := z.sh.Cursor()
	sw, sh := z.sh.Size()
	if z.sh.ChromeHit(mx, my, sw, sh) {
		return 0, 0, 0, 0, false
	}
	tx, ty := z.sh.TileAt(z.sh.MapCursor())
	return tx, ty, tx, ty, true
}

// visibleTiles is the tile window on screen, one tile of margin around
// it: (minX, minY, maxX, maxY).
func (z *Zones) visibleTiles() [4]int {
	sw, sh := z.sh.Size()
	cx0, cy0 := z.sh.TileAt(0, 0)
	cx1, cy1 := z.sh.TileAt(int(float64(sw)*z.sh.MapScale()), int(float64(sh)*z.sh.MapScale()))
	return [4]int{cx0 - 1, cy0 - 1, cx1 + 1, cy1 + 1}
}

// Cells judges the cells under the tool for zone zv, inside the visible
// tiles: Expand marks each cell green where the zone can claim it and red
// where not (CanZoneAt, as painting a zone does); Shrink lists, in red,
// only zv's own cells.
func (z *Zones) Cells(zv game.ZoneView) []GhostCell {
	st := &z.st
	sim := z.sim()
	if st.Kind == ZoneToolNone || sim == nil {
		return nil
	}
	x0, y0, x1, y1, ok := z.toolRect()
	if !ok {
		return nil
	}
	clip := z.visibleTiles()
	if st.Kind == ZoneToolExpand {
		return ZoneGhostCells(x0, y0, x1, y1, clip, func(x, y int) bool { return sim.CanZoneAt(zv.Kind, x, y) })
	}
	own := z.sh.Overlays().Zones.ZoneGeomOf(zv).Cells
	var out []GhostCell
	for _, c := range ZoneGhostCells(x0, y0, x1, y1, clip, func(x, y int) bool { return own[[2]int{x, y}] }) {
		if c.OK {
			out = append(out, GhostCell{X: c.X, Y: c.Y})
		}
	}
	return out
}

// submit sends the dragged rectangle: Expand adds the valid cells to the
// zone, Shrink removes the zone's own cells in it. A rectangle that would
// change nothing sends nothing.
func (z *Zones) submit(zv game.ZoneView) {
	st := &z.st
	x0, y0, x1, y1 := normRect(st.X0, st.Y0, st.X1, st.Y1)
	switch st.Kind {
	case ZoneToolExpand:
		if why, free := ZoneDragRefusal(x0, y0, x1, y1, func(x, y int) string { return z.RefusalAt(zv.Kind, x, y) }); !free {
			hint := "nothing to add there"
			if why != ZoneNoReason && why != game.ZoneRefuseTaken {
				hint += ": " + strings.ToLower(why) // PLAY-023: the most common refusal
			}
			z.sh.Hint(hint)
			return
		}
		z.sh.Enqueue(command.Command{Kind: command.ExpandZone, Target: uint64(zv.ID), X: x0, Y: y0, X2: x1, Y2: y1})
	case ZoneToolShrink:
		own := z.sh.Overlays().Zones.ZoneGeomOf(zv).Cells
		if !AnyZoneCell(x0, y0, x1, y1, func(x, y int) bool { return own[[2]int{x, y}] }) {
			z.sh.Hint("none of this zone's tiles there")
			return
		}
		z.sh.Enqueue(command.Command{Kind: command.RemoveZoneCells, Target: uint64(zv.ID), X: x0, Y: y0, X2: x1, Y2: y1})
	}
}

// ---- the Architect's zone ghost ----

// AnyZoneCell reports whether any cell of the rectangle is claimable.
func AnyZoneCell(x0, y0, x1, y1 int, can func(x, y int) bool) bool {
	x0, y0, x1, y1 = normRect(x0, y0, x1, y1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if can(x, y) {
				return true
			}
		}
	}
	return false
}

// ZoneGhostCells judges each cell of the dragged rectangle, clipped to the
// visible tile window clip (minX, minY, maxX, maxY) so a huge drag stays cheap.
func ZoneGhostCells(x0, y0, x1, y1 int, clip [4]int, can func(x, y int) bool) []GhostCell {
	x0, y0, x1, y1 = normRect(x0, y0, x1, y1)
	x0, y0, x1, y1 = max(x0, clip[0]), max(y0, clip[1]), min(x1, clip[2]), min(y1, clip[3])
	var out []GhostCell
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			out = append(out, GhostCell{X: x, Y: y, OK: can(x, y)})
		}
	}
	return out
}

// drawGhost tints each dragged cell green where the zone can claim it and
// red where it cannot.
func (z *Zones) drawGhost(screen *ebiten.Image, kind string, x0, y0, x1, y1 int) {
	sim := z.sim()
	tile := z.sh.TilePx()
	for _, c := range ZoneGhostCells(x0, y0, x1, y1, z.visibleTiles(), func(x, y int) bool { return sim.CanZoneAt(kind, x, y) }) {
		sx, sy := z.sh.WorldToScreen(c.X, c.Y)
		clr := color.NRGBA{R: 90, G: 200, B: 90, A: 64}
		if !c.OK {
			clr = color.NRGBA{R: 220, G: 70, B: 60, A: 64}
		}
		vector.FillRect(screen, sx, sy, float32(tile), float32(tile), clr, false)
	}
}

// ZoneNoReason stands for a refusal the simulation gives no reason for
// (a Simulation without ZoneRefusal).
const ZoneNoReason = "?"

// RefusalAt is why a zone of kind can't claim tile (x, y): "" when it
// can, the simulation's reason, else ZoneNoReason.
func (z *Zones) RefusalAt(kind string, x, y int) string {
	g := z.sim()
	if zr, ok := g.(ZoneRefuser); ok {
		return zr.ZoneRefusal(kind, x, y)
	}
	if g.CanZoneAt(kind, x, y) {
		return ""
	}
	return ZoneNoReason
}

// ZoneDragRefusal judges a dragged rectangle: free is true when some cell
// can be zoned; otherwise why is the most common refusal over its cells
// (ties: the reason met first, row-major).
func ZoneDragRefusal(x0, y0, x1, y1 int, refusal func(x, y int) string) (why string, free bool) {
	x0, y0, x1, y1 = normRect(x0, y0, x1, y1)
	count := map[string]int{}
	best := 0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			r := refusal(x, y)
			if r == "" {
				return "", true
			}
			count[r]++
			if count[r] > best {
				why, best = r, count[r]
			}
		}
	}
	return why, false
}

// ZoneDragHint is the hint for a drag nothing in which can be zoned.
func ZoneDragHint(why string) string {
	if why == "" || why == ZoneNoReason {
		return "No free tiles there"
	}
	return "Can't zone here: " + why
}

// ZoneTipText is the cursor tip naming why the hovered cell is refused
// ("" when it is fine).
func ZoneTipText(why string) string {
	switch why {
	case "":
		return ""
	case ZoneNoReason:
		return "Can't zone here"
	}
	return "Can't zone: " + why
}

// drawRefusalTip names, beside the cursor, why the hovered cell can't
// take the zone.
func (z *Zones) drawRefusalTip(screen *ebiten.Image, kind string, mx, my int) {
	tx, ty := z.sh.TileAt(z.sh.MapCursor())
	msg := ZoneTipText(z.RefusalAt(kind, tx, ty))
	if msg == "" {
		return
	}
	sw, _ := z.sh.Size()
	cursorTip(screen, msg, mx, my, sw)
}

// ---- drawing the tool ----

var (
	zoneAddClr    = color.NRGBA{R: 90, G: 200, B: 90, A: 64}
	zoneDenyClr   = color.NRGBA{R: 220, G: 70, B: 60, A: 64}
	zoneRemoveClr = color.NRGBA{R: 220, G: 70, B: 60, A: 110}
)

// drawTool draws the active tool: the ghost cells on the map, then the
// cursor's icon and hint.
func (z *Zones) drawTool(screen *ebiten.Image, zv game.ZoneView) {
	k := z.st.Kind
	if k == ZoneToolNone {
		return
	}
	z.drawToolGhost(screen, z.Cells(zv), k)
	z.drawToolHint(screen, zv, k)
}

// drawToolGhost tints the cells under the tool, in physical pixels like
// the map.
func (z *Zones) drawToolGhost(screen *ebiten.Image, cells []GhostCell, k ZoneToolKind) {
	tile := float32(z.sh.TilePx())
	for _, c := range cells {
		clr := zoneDenyClr
		switch {
		case k == ZoneToolShrink:
			clr = zoneRemoveClr
		case c.OK:
			clr = zoneAddClr
		}
		sx, sy := z.sh.WorldToScreen(c.X, c.Y)
		vector.FillRect(screen, sx, sy, tile, tile, clr, false)
	}
}

// IconName is the first of want and its fallbacks the art has. With no
// art loaded (tests, plain mode) it is want. When none exists it is want
// too, so the grid draws its plain initial tile.
func (t *Tool) IconName(want string, fallbacks ...string) string {
	c := t.sh.Env().Art
	if c == nil {
		return want
	}
	for _, n := range append([]string{want}, fallbacks...) {
		if c.Has("ui_icon/" + n) {
			return n
		}
	}
	return want
}

// toolIcon is the tool's cursor icon name.
func (z *Zones) toolIcon(k ZoneToolKind) string {
	if k == ZoneToolShrink {
		return z.IconName("zone_shrink", "plan_clear")
	}
	return z.IconName("zone_expand", "zone_grow")
}

// zoneToolIconSize is the cursor icon's size (logical px).
const zoneToolIconSize = 24

// drawToolHint draws the tool's icon and its hint beside the cursor
// (nothing over the HUD), in logical pixels.
func (z *Zones) drawToolHint(screen *ebiten.Image, zv game.ZoneView, k ZoneToolKind) {
	mx, my := z.sh.Cursor()
	sw, sh := z.sh.Size()
	if z.sh.ChromeHit(mx, my, sw, sh) {
		return
	}
	msg := ZoneToolHint(k, zv)
	boxW, boxH := gfx.TextW(msg, gfx.Body)+16, gfx.LineStep(gfx.Body)+4
	icon := z.Icon(z.toolIcon(k))
	iconW := float32(0)
	if icon != nil {
		iconW = zoneToolIconSize + 4
	}
	x := min(float32(mx+16), float32(sw)-iconW-boxW-4)
	y := min(float32(my+18), float32(sh)-boxH-4)
	if icon != nil {
		gfx.Icon(screen, icon, float64(x), float64(y+(boxH-zoneToolIconSize)/2), zoneToolIconSize)
	}
	gfx.FillRect(screen, x+iconW, y, boxW, boxH, uikit.Panel, false)
	gfx.Text(screen, msg, float64(x+iconW+8), float64(y+2), gfx.Body)
}
