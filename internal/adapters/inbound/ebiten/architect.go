package ebiten

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// rotateKeys is the live rotate keys, "Q/E" by default.
func (a *App) rotateKeys() string {
	return a.bindings().Label(input.RotateLeft) + "/" + a.bindings().Label(input.RotateRight)
}

// archMaxGhost caps the cells a placement preview judges.
const archMaxGhost = 400

func (a *App) selectedCrop() game.CropView {
	crops := zoneSpecies()
	for _, c := range crops {
		if c.Name == a.archCrop {
			return c
		}
	}
	for _, c := range crops {
		if c.Name == "potato" {
			return c
		}
	}
	return crops[0]
}

// archStuffPerRow is how many material buttons fit one row when a tool
// has more than four; carpets (five materials with counts) wrap to a
// second row, and long names ("Sandstone blocks") take fewer per row.
const archStuffPerRow = 3

// archStuffGrid lays out a buildable's material buttons inside the
// description box: per row, rows, and the button width.
func archStuffGrid(bv game.BuildableView) (perRow, rows int, bw float32) {
	n := len(bv.Stuffs)
	if n == 0 {
		return 0, 0, 0
	}
	perRow = n
	if n > 4 {
		perRow = archStuffPerRow
	}
	width := func(k int) float32 { return (archInnerW - archBtnGap*float32(k-1)) / float32(k) }
	need := 0
	if !bv.Carpet { // carpet labels carry counts and keep their three per row
		for _, s := range bv.Stuffs {
			need = max(need, int(textW(stuffLabel(s), tsBody))+16)
		}
	}
	for perRow > 1 && width(perRow) < float32(need) {
		perRow--
	}
	bw = min(max(84, float32(need)), width(perRow))
	return perRow, (n + perRow - 1) / perRow, bw
}

// architectRect is the Architect's area on the HUD: the description box
// above the category panel (zero while it is closed).
func (a *App) architectRect() button {
	if !a.archOpen {
		return button{}
	}
	l := a.hud()
	r := boundsOf([]rect{l.ArchCats, l.ArchDesc})
	return button{x: r.X, y: r.Y, w: r.W, h: r.H}
}

// architectButtons are the category buttons and, for the armed tool, its
// material and crop buttons. The tools themselves are command buttons in
// the grid (archToolCommands).
func (a *App) architectButtons() []button {
	if !a.archOpen {
		return nil
	}
	l := a.hud()
	if l.ArchCats.empty() {
		return nil
	}
	out := make([]button, 0, len(archCategories)+16)
	for i, c := range archCategories {
		col, row := i%2, i/2
		out = append(out, button{
			id: "arch.cat." + c.id, label: c.label,
			x: l.ArchCats.X + archPad + float32(col)*(archCatW+archCatGap),
			y: l.ArchCats.Y + archPad + float32(row)*(archCatH+archCatGap),
			w: archCatW, h: archCatH, selected: a.archCat == c.id,
		})
	}
	if !l.ArchDesc.empty() {
		out = append(out, a.archDescButtons(l.ArchDesc)...)
	}
	return out
}

// stuffLabel is a material button's text: "Wood", "Granite blocks",
// "Steel", or the carpet's material ("Cloth" for "cloth carpet").
func stuffLabel(s string) string {
	s = strings.TrimSuffix(s, " carpet")
	return strings.ToUpper(s[:1]) + s[1:]
}

// toolStuff is the material the tool will build with: the player's pick
// when the buildable offers it, else its default (first) material.
func (a *App) toolStuff(t archTool) string {
	bv, ok := buildable(t.build)
	if !ok || len(bv.Stuffs) == 0 {
		return "wood"
	}
	for _, s := range bv.Stuffs {
		if s == a.archStuff {
			return s
		}
	}
	return bv.Stuffs[0]
}

// stuffCatch is the percent of tracked dirt a carpet of material stuff
// wipes (0 when bv is not a carpet or the catalog lists no figure).
func stuffCatch(bv game.BuildableView, stuff string) int {
	for i, s := range bv.Stuffs {
		if s == stuff && i < len(bv.Catches) {
			return bv.Catches[i]
		}
	}
	return 0
}

// archTooltipLines is the hover card for tool t with material stuff:
// name, description, cost and work, a stock warning, and for carpets the
// dirt it catches plus where to lay it. wood is the colony's wood.
func archTooltipLines(t archTool, stuff string, wood, stock int) []string {
	lines := []string{t.label, t.desc}
	bv, ok := buildable(t.build)
	if !ok {
		return lines
	}
	if bv.Carpet {
		lines = append(lines, fmt.Sprintf("Cost: 1 %s · Work %d · catches %d%% of tracked dirt", stuff, bv.Work, stuffCatch(bv, stuff)))
		lines = append(lines, carpetStockLine(stuff, stock))
		return append(lines, "Tip: lay it on the tile just inside a door")
	}
	cost := fmt.Sprintf("Cost: %d %s", bv.Wood, stuff)
	if n := componentsOf(bv); n > 0 {
		cost += fmt.Sprintf(" + %d %s", n, componentWord(n))
	}
	lines = append(lines, fmt.Sprintf("%s   Work: %d", cost, bv.Work))
	if pl := archPowerLine(bv); pl != "" { // PLAY-021: draw, output or store
		lines = append(lines, pl)
	}
	if stuff == "wood" && bv.Wood > wood {
		lines = append(lines, fmt.Sprintf("Only %d wood in the colony", wood))
	}
	return lines
}

// archStockLine is the Architect's stock line: wood, plus the selected
// material's free stock ("Granite blocks 40") or, with no stone or steel
// material picked, the colony's stone blocks.
func archStockLine(w game.WorldView, wood int, stuff string) string {
	if stuff == "" || stuff == "wood" {
		return fmt.Sprintf("Wood %d   Stone %d", wood, w.Stone)
	}
	return fmt.Sprintf("Wood %d   %s %d", wood, stuffLabel(stuff), stockOf(w, stuff))
}

// stockOf counts the free (on the ground, not carried, not forbidden)
// items of kind on the map.
func stockOf(w game.WorldView, kind string) int {
	n := 0
	for _, it := range w.Items {
		if it.Kind == kind && !it.Carried && !it.Forbidden {
			n += it.Qty
		}
	}
	return n
}

// carpetStockLine is the Architect card's stock line for a carpet: the
// count, and when none is on hand where to sew one.
func carpetStockLine(carpet string, stock int) string {
	if stock > 0 {
		return fmt.Sprintf("In stock: %d %s", stock, carpet)
	}
	return fmt.Sprintf("In stock: 0 %s%s", carpet, sewHint(carpet))
}

// sewHint is ": sew one at a tailoring bench (8 straw)" from the recipe
// that makes item, or "" when no recipe does.
func sewHint(item string) string {
	for _, r := range game.Recipes() {
		if r.Output == item {
			return fmt.Sprintf(": sew one at a %s (%s)", r.Bench, r.Input)
		}
	}
	return ""
}

// cropLines is the crop picker's stat lines for c (crops and the tree).
func cropLines(c game.CropView) []string {
	if c.Name == "tree" {
		return []string{
			fmt.Sprintf("Tree: %.1f days, %d wood per tree", c.Days, c.Yield),
			fmt.Sprintf("rich %.1fd  gravel %.1fd  felled when grown", c.RichDays, c.GravelDay),
		}
	}
	first := fmt.Sprintf("%s: %.1f days, %d %s per plant", strings.ToUpper(c.Name[:1])+c.Name[1:], c.Days, c.Yield, c.Label)
	second := fmt.Sprintf("rich %.1fd  gravel %.1fd  food %d each", c.RichDays, c.GravelDay, c.Nutrition)
	if !c.Food {
		second = fmt.Sprintf("rich %.1fd  gravel %.1fd  %s, not food", c.RichDays, c.GravelDay, c.Label)
	}
	keeps := "keeps forever"
	if c.ShelfDays > 0 {
		keeps = fmt.Sprintf("keeps %.0f days", c.ShelfDays)
	}
	out := []string{first, second, fmt.Sprintf("frost kills below %s, %s", fmtTemp(c.FrostKill), keeps)}
	if c.ExtraQty > 0 {
		out = append(out, fmt.Sprintf("also +%d %s per harvest", c.ExtraQty, c.Extra))
	}
	return out
}

func (a *App) openArchitect() {
	a.archOpen = true
	a.workOpen = false
	a.closeFloat()
	if a.archCat == "" {
		a.archCat, a.hst.arch.dflt = "structure", "structure"
	}
}

func (a *App) closeArchitect() {
	a.archOpen = false
	a.archTool = ""
	a.archDrag = false
	a.syncRoofForce()
}

// selectArchCategory opens category id; the open category again collapses
// it (no tools shown, none armed). The category the Architect opened on by
// itself does not count as chosen: clicking it first confirms it.
func (a *App) selectArchCategory(id string) {
	defer a.syncRoofForce()
	dflt := a.hst.arch.dflt
	a.hst.arch.dflt = ""
	if a.archCat == id && dflt != id {
		a.archCat, a.archTool, a.archDrag = "", "", false
		return
	}
	a.archCat = id
	if t, ok := toolByID(a.archTool); !ok || t.category != id {
		a.archTool = ""
		a.archDrag = false
	}
}

func (a *App) selectArchTool(id string) {
	t, ok := toolByID(id)
	if !ok {
		return
	}
	if a.areaToolPick(id) { // area_menu.go: the allowed-area tools open a submenu
		return
	}
	a.hst.arch.dflt = "" // the player is working in the open category
	a.archCat = t.category
	defer a.syncRoofForce()
	if a.archTool == id {
		a.archTool = "" // clicking the selected card toggles it off
		return
	}
	a.archTool = id
	a.archDrag = false
	a.archStyle = styleDefault
}

func (a *App) icon(name string) *ebiten.Image {
	if a.art == nil {
		return nil
	}
	return a.art.image("ui_icon/"+name, color.NRGBA{})
}

func drawIcon(dst, img *ebiten.Image, x, y, size float64) { drawIconCS(dst, img, x, y, size, noCS) }

// drawIconCS is drawIcon multiplied by cs (a stone item's tint).
func drawIconCS(dst, img *ebiten.Image, x, y, size float64, cs colorScale) {
	if img == nil {
		return
	}
	s := size / float64(img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	if cs != noCS {
		op.ColorScale.Scale(cs[0], cs[1], cs[2], cs[3])
	}
	uiDrawImage(dst, img, op)
}

// drawArchitect draws the category panel and the description box; the
// tools are drawn by the command grid.
func (a *App) drawArchitect(screen *ebiten.Image) {
	if !a.archOpen {
		return
	}
	l := a.hud()
	if l.ArchCats.empty() {
		return
	}
	drawPanel(screen, l.ArchCats.X, l.ArchCats.Y, l.ArchCats.W, l.ArchCats.H)
	if d := l.ArchDesc; !d.empty() {
		drawPanel(screen, d.X, d.Y, d.W, d.H)
		for _, ln := range a.archPlan().lines {
			if ln.y+lineStep(ln.style) > d.H {
				continue // a clamped box on a small screen shows what fits
			}
			drawText(screen, ln.text, float64(d.X+archPad), float64(d.Y+ln.y), ln.style)
		}
	}
	mx, my := a.currentCursor()
	for _, b := range a.architectButtons() {
		drawButton(screen, b, b.contains(mx, my))
	}
}

// ---------------------------------------------------------- placement

func (a *App) rotateTool(dir int) {
	a.archFacing = rotateFacing(a.archFacing, dir)
}

// handleArchitectInput runs while a tool is selected: drag to place,
// right-click to stop (taking the click, so no colonist is ordered too),
// Q/E to rotate or change the draw style.
func (a *App) handleArchitectInput() {
	defer a.syncRoofForce() // right-click and Escape drop the tool; keep the overlay in step
	if !a.archOpen || a.archTool == "" || a.renderer == nil {
		return
	}
	f := &a.in
	if f.fired(input.RotateLeft) {
		a.rotateOrStyle(-1)
	}
	if f.fired(input.RotateRight) {
		a.rotateOrStyle(1)
	}
	a.archCtrl = f.ctrl()
	if f.clicked(mbRight) {
		f.take(mbRight)
		if a.archDrag {
			a.archDrag = false
		} else {
			a.archTool = ""
		}
		return
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	onMap := !a.colonyChromeHit(mx, my, sw, sh)
	pmx, pmy := a.mapCursor()
	tx, ty := a.renderer.TileAt(pmx, pmy)
	if onMap && f.clicked(mbLeft) {
		f.take(mbLeft)
		a.archDrag = true
		a.archX0, a.archY0 = tx, ty
		a.archX1, a.archY1 = tx, ty
	}
	if a.archDrag && f.down(mbLeft) && onMap {
		a.archX1, a.archY1 = tx, ty
	}
	if a.archDrag && f.released(mbLeft) {
		if onMap {
			a.archX1, a.archY1 = tx, ty
		}
		a.submitArchitect()
		a.archDrag = false
	}
}

type ghostCell struct {
	x, y   int
	ok     bool
	reason string
	// replaces names the wall or fence a door/gate here takes the place of,
	// or the swap label ("wood wall → steel") for an in-place material swap.
	replaces string
	// facing is the facing the sim would store here (door/gate auto-orient).
	facing string
	// same: the same piece in the same material is already here; the cell
	// is neutral, does nothing and is not submitted.
	same bool
}

// isSwapLabel reports a material-swap label; the "→" marks it.
func isSwapLabel(s string) bool { return strings.Contains(s, "→") }

// previewCells is what the current tool would affect right now: the
// dragged span, or the hovered cell when not dragging. An area tool in
// the rectangle style gets its two corners; in the oval style every cell.
func (a *App) previewCells() (archTool, []ghostCell) {
	t, ok := toolByID(a.archTool)
	if !ok || a.renderer == nil {
		return archTool{}, nil
	}
	x0, y0, x1, y1 := a.dragSpan()
	style := a.curStyle(t)
	if t.mode == placeArea {
		if style != styleOval {
			return t, []ghostCell{{x: x0, y: y0, ok: true}, {x: x1, y: y1, ok: true}}
		}
		var out []ghostCell
		for _, c := range shapeCells(style, [2]int{x0, y0}, [2]int{x1, y1}, false) {
			out = append(out, ghostCell{x: c[0], y: c[1], ok: true})
		}
		return t, out
	}
	cells := shapeCells(style, [2]int{x0, y0}, [2]int{x1, y1}, false)
	if len(cells) > archMaxGhost {
		cells = cells[:archMaxGhost]
	}
	out := make([]ghostCell, 0, len(cells))
	for _, c := range cells {
		fc := facingNames[a.archFacing]
		ok, reason := a.renderer.game.CanPlaceAt(t.build, a.toolStuff(t), fc, c[0], c[1])
		gc := ghostCell{x: c[0], y: c[1], ok: ok, reason: reason,
			facing: a.renderer.game.AutoFacingAt(t.build, fc, c[0], c[1])}
		if !ok && reason == game.ReasonSame {
			gc.same, gc.reason = true, ""
		}
		if ok {
			gc.replaces = a.renderer.game.ReplacesAt(t.build, a.toolStuff(t), c[0], c[1])
		}
		out = append(out, gc)
	}
	return t, out
}

// dragSpan is the drag's two corners (the hovered cell twice when not
// dragging), the end squared while Ctrl is held on a box style.
func (a *App) dragSpan() (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = a.archX0, a.archY0, a.archX1, a.archY1
	if !a.archDrag {
		x0, y0 = a.renderer.TileAt(a.mapCursor())
		x1, y1 = x0, y0
	}
	if t, ok := toolByID(a.archTool); ok && a.archCtrl && a.curStyle(t) != styleLine && len(toolStyles(t)) > 0 {
		x1, y1 = constrainEnd(x0, y0, x1, y1)
	}
	return x0, y0, x1, y1
}

// areaOval reports the area tool draws an oval.
func (a *App) areaOval(t archTool) bool { return t.mode == placeArea && a.curStyle(t) == styleOval }

func (a *App) submitArchitect() {
	if a.renderer == nil {
		return
	}
	t, cells := a.previewCells()
	if t.id == "" {
		return
	}
	if t.mode == placeArea {
		x0, y0, x1, y1 := a.archX0, a.archY0, a.archX1, a.archY1
		if a.archCtrl {
			x1, y1 = constrainEnd(x0, y0, x1, y1)
		}
		runs := []rowRun{{y: -1}}
		if a.areaOval(t) {
			cs := make([][2]int, 0, len(cells))
			for _, c := range cells {
				cs = append(cs, [2]int{c.x, c.y})
			}
			runs = rowRuns(cs)
		}
		if kind := zoneKindOf(t); kind != "" {
			if why, free := zoneDragRefusal(x0, y0, x1, y1, func(x, y int) string { return a.zoneRefusalAt(kind, x, y) }); !free {
				a.hint = zoneDragHint(why) // the command still goes out; the sim ignores it
			}
		}
		for _, run := range runs {
			c := command.Command{Kind: t.cmd, X: x0, Y: y0, X2: x1, Y2: y1, Enabled: t.enable}
			if t.cmd == command.PaintArea {
				c.Index = a.areaPaintIndex() // areas_ui.go: the area picked in the manager
			}
			if run.y >= 0 { // one oval row
				c.X, c.Y, c.X2, c.Y2 = run.x0, run.y, run.x1, run.y
			}
			if t.crop != "" {
				c.Build, _ = game.ZoneSpeciesID(a.selectedCrop().Name)
			}
			if t.variant != 0 {
				c.Build = t.variant // a dumping stockpile (PLAY-023)
			}
			a.renderer.enqueue(c)
		}
		return
	}
	id, ok := kindID(t.build)
	if !ok {
		return
	}
	stuff, _ := game.StuffID(a.toolStuff(t))
	for _, c := range cells {
		if c.ok && !c.same {
			a.renderer.enqueue(command.Command{Kind: command.DesignateBuild, Build: id, Facing: a.archFacing, Stuff: stuff, X: c.x, Y: c.y})
		}
	}
}

func (a *App) drawArchitectGhost(screen *ebiten.Image) {
	if a.renderer == nil || !a.archOpen || a.archTool == "" {
		return
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	if !a.archDrag && a.colonyChromeHit(mx, my, sw, sh) {
		return
	}
	t, cells := a.previewCells()
	if a.areaOval(t) {
		kind := zoneKindOf(t)
		for _, c := range cells {
			if kind != "" {
				a.drawZoneGhost(screen, kind, c.x, c.y, c.x, c.y)
			} else {
				a.renderer.drawGhostRect(screen, c.x, c.y, c.x, c.y)
			}
		}
		if kind != "" {
			a.drawZoneRefusalTip(screen, kind, mx, my)
		}
		return
	}
	if t.mode == placeArea {
		if kind := zoneKindOf(t); kind != "" {
			a.drawZoneGhost(screen, kind, cells[0].x, cells[0].y, cells[1].x, cells[1].y)
			a.drawZoneRefusalTip(screen, kind, mx, my)
			return
		}
		a.renderer.drawGhostRect(screen, cells[0].x, cells[0].y, cells[1].x, cells[1].y)
		if t.cmd == command.DesignateRoof {
			a.drawRoofToolGhost(screen, cells[0].x, cells[0].y, cells[1].x, cells[1].y)
		}
		return
	}
	a.renderer.drawPlacementGhost(screen, t.build, a.toolStuff(t), facingNames[a.archFacing], cells)
	a.drawPowerGhostLinks(screen, t, cells)    // power_overlay.go: the link a powered building would make
	if rad, ok := placementRing(t.build); ok { // turret_draw.go: the beacon's pickup radius
		for _, c := range cells {
			if c.ok && !c.same {
				a.renderer.drawRangeRing(screen, c.x, c.y, rad, tokenNRGBA(uiAccent))
			}
		}
	}
	valid, firstBad, replaces := 0, "", ""
	for _, c := range cells {
		if c.same {
			continue // nothing to do here: not counted, not an error
		}
		if c.ok {
			valid++
			if replaces == "" {
				replaces = c.replaces
			}
		} else if firstBad == "" {
			firstBad = c.reason
		}
	}
	bv, _ := buildable(t.build)
	msg := fmt.Sprintf("%s x%d  %d %s", t.label, valid, valid*bv.Wood, a.toolStuff(t))
	if replaces != "" {
		if isSwapLabel(replaces) {
			msg += "   " + replaces
		} else {
			msg += "   replaces " + replaces
		}
	}
	if firstBad != "" {
		msg += "   can't place: " + firstBad
	}
	mw := int(textW(msg, tsBody))
	lx := min(mx+16, sw-mw-18) // keep the label on screen
	uiFillRect(screen, float32(lx), float32(my+18), float32(mw+12), lineStep(tsBody)+4, uiPanel, false)
	drawLabel(screen, lx+6, my+20, msg)
}

// presetArchTool opens the Architect with the build tool for kind
// selected (never toggling it off), the material and facing preset, as
// the building panel's Copy does. False when no tool builds kind.
func (a *App) presetArchTool(kind, stuff, facing string) bool {
	for _, t := range allArchTools() {
		if t.build != kind {
			continue
		}
		a.openArchitect()
		a.archCat = t.category
		a.archTool = t.id
		a.archDrag = false
		a.archStyle = styleDefault
		if bv, ok := buildable(kind); ok && len(bv.Stuffs) > 1 {
			a.archStuff = bv.Stuffs[0]
			for _, s := range bv.Stuffs {
				if s == stuff {
					a.archStuff = s
				}
			}
		}
		for i, n := range facingNames {
			if n == facing {
				a.archFacing = uint8(i)
			}
		}
		a.syncRoofForce()
		return true
	}
	return false
}

// zoneKindOf maps a zone tool to the sim's zone kind ("" for other tools).
func zoneKindOf(t archTool) string {
	switch t.cmd {
	case command.DesignateStockpile:
		if t.variant == game.StockpileVariantDump {
			return game.ZoneDump
		}
		return game.ZoneStockpile
	case command.DesignateGrowZone:
		return game.ZoneGrow
	case command.DesignatePen:
		return game.ZonePen
	}
	return ""
}

func normRect(x0, y0, x1, y1 int) (int, int, int, int) {
	return min(x0, x1), min(y0, y1), max(x0, x1), max(y0, y1)
}

// anyZoneCell reports whether any cell of the rectangle is claimable.
func anyZoneCell(x0, y0, x1, y1 int, can func(x, y int) bool) bool {
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

// zoneGhostCells judges each cell of the dragged rectangle, clipped to the
// visible tile window clip (minX, minY, maxX, maxY) so a huge drag stays cheap.
func zoneGhostCells(x0, y0, x1, y1 int, clip [4]int, can func(x, y int) bool) []ghostCell {
	x0, y0, x1, y1 = normRect(x0, y0, x1, y1)
	x0, y0, x1, y1 = max(x0, clip[0]), max(y0, clip[1]), min(x1, clip[2]), min(y1, clip[3])
	var out []ghostCell
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			out = append(out, ghostCell{x: x, y: y, ok: can(x, y)})
		}
	}
	return out
}

// drawZoneGhost tints each dragged cell green where the zone can claim it
// and red where it cannot.
func (a *App) drawZoneGhost(screen *ebiten.Image, kind string, x0, y0, x1, y1 int) {
	r := a.renderer
	sw, sh := a.size()
	tile := r.tilePx()
	cx0, cy0 := r.TileAt(0, 0)
	cx1, cy1 := r.TileAt(int(float64(sw)*uiScale()), int(float64(sh)*uiScale()))
	clip := [4]int{cx0 - 1, cy0 - 1, cx1 + 1, cy1 + 1}
	for _, c := range zoneGhostCells(x0, y0, x1, y1, clip, func(x, y int) bool { return r.game.CanZoneAt(kind, x, y) }) {
		sx, sy := r.worldToScreen(c.x, c.y)
		clr := color.NRGBA{R: 90, G: 200, B: 90, A: 64}
		if !c.ok {
			clr = color.NRGBA{R: 220, G: 70, B: 60, A: 64}
		}
		vector.FillRect(screen, sx, sy, float32(tile), float32(tile), clr, false)
	}
}

// zoneRefuser is the one simulation behaviour the zone ghost needs beyond
// CanZoneAt (consumer-owned, PLAY-023): why a zone of kind can't claim a
// cell, "" when it can.
type zoneRefuser interface {
	ZoneRefusal(kind string, x, y int) string
}

// zoneNoReason stands for a refusal the simulation gives no reason for
// (a Simulation without ZoneRefusal).
const zoneNoReason = "?"

// zoneRefusalAt is why a zone of kind can't claim tile (x, y): "" when it
// can, the simulation's reason, else zoneNoReason.
func (a *App) zoneRefusalAt(kind string, x, y int) string {
	g := a.renderer.game
	if zr, ok := g.(zoneRefuser); ok {
		return zr.ZoneRefusal(kind, x, y)
	}
	if g.CanZoneAt(kind, x, y) {
		return ""
	}
	return zoneNoReason
}

// zoneDragRefusal judges a dragged rectangle: free is true when some cell
// can be zoned; otherwise why is the most common refusal over its cells
// (ties: the reason met first, row-major).
func zoneDragRefusal(x0, y0, x1, y1 int, refusal func(x, y int) string) (why string, free bool) {
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

// zoneDragHint is the hint for a drag nothing in which can be zoned.
func zoneDragHint(why string) string {
	if why == "" || why == zoneNoReason {
		return "No free tiles there"
	}
	return "Can't zone here: " + why
}

// zoneTipText is the cursor tip naming why the hovered cell is refused
// ("" when it is fine).
func zoneTipText(why string) string {
	switch why {
	case "":
		return ""
	case zoneNoReason:
		return "Can't zone here"
	}
	return "Can't zone: " + why
}

// drawZoneRefusalTip names, beside the cursor, why the hovered cell can't
// take the zone.
func (a *App) drawZoneRefusalTip(screen *ebiten.Image, kind string, mx, my int) {
	tx, ty := a.renderer.TileAt(a.mapCursor())
	msg := zoneTipText(a.zoneRefusalAt(kind, tx, ty))
	if msg == "" {
		return
	}
	sw, _ := a.size()
	mw := int(textW(msg, tsBody))
	lx := min(mx+16, sw-mw-18) // keep the label on screen
	uiFillRect(screen, float32(lx), float32(my+18), float32(mw+12), lineStep(tsBody)+4, uiPanel, false)
	drawLabel(screen, lx+6, my+20, msg)
}

// drawIconFallback is a plain tile with the tool's initial, for a button
// whose icon is missing from the art catalog.
func drawIconFallback(screen *ebiten.Image, t archTool, x, y float32) {
	uiFillRect(screen, x, y, 40, 40, uiBtnDisabled, false)
	uiStrokeRect(screen, x, y, 40, 40, 1, uiBtnBorder, false)
	if t.label != "" {
		drawLabel(screen, int(x)+16, int(y)+15, t.label[:1])
	}
}
