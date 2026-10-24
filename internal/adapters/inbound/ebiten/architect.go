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

const (
	archW        = 276
	archCardW    = 126
	archCardH    = 58
	archCardGap  = 8
	archTop      = 80 // cards start below the header and two category rows
	archInfoH    = 70
	archMaxGhost = 400
)

// Crop picker (growing-zone tool): 4 crops per row, then a stats line.
const (
	archCropCols = 4
	archCropBtnW = 62
	archCropBtnH = 22
	archCropRows = 3 // eight crops and the tree entry
	archCropH    = archCropRows*(archCropBtnH+4) + 72
)

// archCardRows is the card rows the current category needs (min 2).
func (a *App) archCardRows() int {
	rows := (len(toolsIn(a.archCat)) + 1) / 2
	if rows < 2 {
		rows = 2
	}
	return rows
}

// archStride is the distance between card rows: the card and its gap,
// with the gap squeezed (to 2 px) when a long category (furniture) would
// run the panel into the bottom bar.
func (a *App) archStride() float32 {
	stride := float32(archCardH + archCardGap)
	_, sh := a.size()
	avail := hudBarY(sh) - float32(hudTopH+2*hudPad) - archTop - archInfoH - float32(a.archStuffExtra())
	if a.archCropOpen() {
		avail -= archCropH
	}
	rows := float32(a.archCardRows())
	if need := stride * rows; need > avail {
		stride = max(float32(archCardH+2), avail/rows)
	}
	return stride
}

// archCropOpen reports the selected tool sows a crop.
func (a *App) archCropOpen() bool {
	t, ok := toolByID(a.archTool)
	return ok && t.crop != ""
}

func (a *App) selectedCrop() game.CropView {
	crops := game.ZoneSpecies()
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

// archStuffGrid lays out a buildable's material buttons: per row, rows,
// and the button width.
func archStuffGrid(bv game.BuildableView) (perRow, rows int, bw float32) {
	n := len(bv.Stuffs)
	if n == 0 {
		return 0, 0, 0
	}
	perRow = n
	if n > 4 {
		perRow = archStuffPerRow
	}
	width := func(k int) float32 { return (archW - 20 - 6*float32(k-1)) / float32(k) }
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

// archStuffExtra is the extra panel height the material rows need.
func (a *App) archStuffExtra() int {
	if t, ok := toolByID(a.archTool); ok {
		if bv, ok := buildable(t.build); ok && len(bv.Stuffs) > 1 {
			_, rows, _ := archStuffGrid(bv)
			return max(rows-1, 0) * 26
		}
	}
	return 0
}

func (a *App) architectRect() button {
	h := archTop + float32(a.archCardRows())*a.archStride() + archInfoH + float32(a.archStuffExtra())
	if a.archCropOpen() {
		h += archCropH
	}
	return button{x: hudPad, y: hudTopH + hudPad, w: archW, h: h}
}

// archCropTop is the y of the crop picker block.
func (a *App) archCropTop(r button) float32 {
	return r.y + archTop + float32(a.archCardRows())*a.archStride()
}

func (a *App) architectButtons() []button {
	if !a.archOpen {
		return nil
	}
	r := a.architectRect()
	out := make([]button, 0, len(archCategories)+6)
	cx, cy := r.x+10, r.y+30
	for _, c := range archCategories {
		w := textW(c.label, tsBody) + 20
		if cx+w > r.x+r.w-10 { // wrap to a second row
			cx, cy = r.x+10, cy+24
		}
		out = append(out, button{
			id: "arch.cat." + c.id, label: c.label, x: cx, y: cy, w: w, h: 22,
			selected: a.archCat == c.id,
		})
		cx += w + 4
	}
	for i, t := range toolsIn(a.archCat) {
		col, row := i%2, i/2
		b := button{
			id: "arch.tool." + t.id,
			x:  r.x + 10 + float32(col)*(archCardW+archCardGap),
			y:  r.y + archTop + float32(row)*a.archStride(),
			w:  archCardW, h: archCardH,
			selected: a.archTool == t.id,
		}
		if t.build != "" {
			if why := a.toolLock(t); why != "" {
				b.disabled, b.reason, b.selected, b.quiet = true, why, false, true
			}
		}
		out = append(out, b)
	}
	if a.archCropOpen() {
		sel := a.selectedCrop().Name
		top := a.archCropTop(r)
		for i, c := range game.ZoneSpecies() {
			col, row := i%archCropCols, i/archCropCols
			b := button{
				id: "arch.crop." + c.Name, label: strings.ToUpper(c.Name[:1]) + c.Name[1:],
				x: r.x + 10 + float32(col)*(archCropBtnW+2), y: top + float32(row)*(archCropBtnH+4),
				w: archCropBtnW, h: archCropBtnH, selected: c.Name == sel,
			}
			if p := a.lockedBy("crop:" + c.Name); p != "" {
				b.disabled, b.reason, b.selected, b.quiet = true, "Needs research: "+p, false, true
			}
			out = append(out, b)
		}
	}
	if t, ok := toolByID(a.archTool); ok {
		if bv, ok := buildable(t.build); ok && len(bv.Stuffs) > 1 {
			// One button per material, default first, shrunk to fit the
			// panel (a carpet shows one per carpet material).
			perRow, rows, bw := archStuffGrid(bv)
			for i, s := range bv.Stuffs {
				row, col := i/perRow, i%perRow
				label := stuffLabel(s)
				if bv.Carpet && a.renderer != nil {
					label = fmt.Sprintf("%s (%d)", label, stockOf(a.renderer.world(), s))
				}
				out = append(out, button{
					id: "arch.stuff." + s, label: label, x: r.x + 10 + float32(col)*(bw+6),
					y: r.y + r.h - 28 - float32(rows-1-row)*26, w: bw, h: 22,
					selected: a.toolStuff(t) == s,
				})
			}
		}
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
		a.archCat = "structure"
	}
}

func (a *App) closeArchitect() {
	a.archOpen = false
	a.archTool = ""
	a.archDrag = false
	a.syncRoofForce()
}

func (a *App) selectArchCategory(id string) {
	defer a.syncRoofForce()
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
	dst.DrawImage(img, op)
}

func (a *App) drawArchitect(screen *ebiten.Image) {
	if !a.archOpen {
		return
	}
	r := a.architectRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawText(screen, "Architect", float64(r.x)+12, float64(r.y)+6, tsLabel)
	mx, my := a.currentCursor()
	wood := 0
	if a.renderer != nil {
		wood = a.renderer.world().Wood
	}
	var hover *archTool
	for _, b := range a.architectButtons() {
		if !strings.HasPrefix(b.id, "arch.tool.") {
			drawButton(screen, b, b.contains(mx, my))
			continue
		}
		t, _ := toolByID(strings.TrimPrefix(b.id, "arch.tool."))
		drawButton(screen, b, b.contains(mx, my))
		if img := a.icon(t.icon); img != nil {
			drawIcon(screen, img, float64(b.x)+6, float64(b.y)+8, 40)
		} else {
			drawIconFallback(screen, t, b.x+6, b.y+8)
		}
		tw := float64(b.w) - 54
		drawFitSmall(screen, t.label, float64(b.x)+50, float64(b.y)+8, tw, tsBody)
		sub := ""
		if b.disabled {
			sub = lockWord(b.reason)
		} else if bv, ok := buildable(t.build); ok && bv.Carpet {
			sub = "1 carpet"
		} else if ok {
			sub = fmt.Sprintf("%d %s", bv.Wood, strings.TrimSuffix(a.toolStuff(t), " blocks"))
		} else {
			sub = map[placeMode]string{placeArea: "drag area"}[t.mode]
		}
		drawText(screen, ellipsize(sub, tsSmall, tw), float64(b.x)+50, float64(b.y)+31, withColor(tsSmall, uiTextDim))
		if b.contains(mx, my) {
			tt := t
			hover = &tt
		}
	}
	if a.archCropOpen() {
		y := int(a.archCropTop(r)) + archCropRows*(archCropBtnH+4) + 2
		for i, ln := range cropLines(a.selectedCrop()) {
			drawLabel(screen, int(r.x)+12, y+i*int(lineStep(tsBody)), ln)
		}
	}
	step := 18
	iy := int(r.y+r.h) - archInfoH - a.archStuffExtra() + 6
	stuff := ""
	if t, ok := toolByID(a.archTool); ok {
		if bv, ok := buildable(t.build); ok && !bv.Carpet {
			stuff = a.toolStuff(t)
		}
	}
	if a.renderer != nil {
		drawLabel(screen, int(r.x)+12, iy, archStockLine(a.renderer.world(), wood, stuff))
	}
	if t, ok := toolByID(a.archTool); ok {
		hint := map[placeMode]string{
			placeSingle: "Click to place",
			placeLine:   "Drag for a line",
			placeFill:   "Drag to fill",
			placeArea:   "Drag an area",
		}[t.mode]
		if t.build == "door" || t.build == "fence gate" {
			hint += "  faces along the wall or fence; " + a.rotateKeys() + " sets a free-standing one (" + facingNames[a.archFacing] + ")"
		} else if bv, ok := buildable(t.build); ok && bv.Rotatable {
			hint += "  " + a.rotateKeys() + " rotate (" + facingNames[a.archFacing] + ")"
		}
		if len(toolStyles(t)) > 1 {
			hint = styleNames[a.curStyle(t)] + "  " + a.rotateKeys() + " shape, Ctrl square"
		}
		drawLabel(screen, int(r.x)+12, iy+step, hint)
		if bv, ok := buildable(t.build); !ok || len(bv.Stuffs) < 2 {
			drawLabel(screen, int(r.x)+12, iy+2*step, "Right-click: stop placing")
		}
	} else {
		drawLabel(screen, int(r.x)+12, iy+step, "Pick something to place")
	}
	if hover != nil {
		a.drawArchTooltip(screen, *hover, wood, int(r.x+r.w)+6, int(r.y)+archTop)
	}
}

func (a *App) drawArchTooltip(screen *ebiten.Image, t archTool, wood, x, y int) {
	stock := 0
	if a.renderer != nil {
		stock = stockOf(a.renderer.world(), a.toolStuff(t))
	}
	lines := archTooltipLines(t, a.toolStuff(t), wood, stock)
	if why := a.toolLock(t); why != "" {
		lines = append(lines, "Locked: "+why)
	}
	step := int(lineStep(tsBody))
	w := int(maxTextW(lines, tsBody))
	drawPanel(screen, float32(x), float32(y), float32(w+20), float32(len(lines)*step+12))
	for i, l := range lines {
		drawLabel(screen, x+10, y+6+i*step, l)
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
	tx, ty := a.renderer.TileAt(mx, my)
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
		mx, my := a.currentCursor()
		x0, y0 = a.renderer.TileAt(mx, my)
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
		if kind := zoneKindOf(t); kind != "" && !anyZoneCell(x0, y0, x1, y1, func(x, y int) bool {
			return a.renderer.game.CanZoneAt(kind, x, y)
		}) {
			a.hint = "No free tiles there" // the command still goes out; the sim ignores it
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
		return
	}
	if t.mode == placeArea {
		if kind := zoneKindOf(t); kind != "" {
			a.drawZoneGhost(screen, kind, cells[0].x, cells[0].y, cells[1].x, cells[1].y)
			return
		}
		a.renderer.drawGhostRect(screen, cells[0].x, cells[0].y, cells[1].x, cells[1].y)
		if t.cmd == command.DesignateRoof {
			a.drawRoofToolGhost(screen, cells[0].x, cells[0].y, cells[1].x, cells[1].y)
		}
		return
	}
	a.renderer.drawPlacementGhost(screen, t.build, a.toolStuff(t), facingNames[a.archFacing], cells)
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
	vector.FillRect(screen, float32(lx), float32(my+18), float32(mw+12), lineStep(tsBody)+4, uiPanel, false)
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
	cx1, cy1 := r.TileAt(sw, sh)
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

// drawIconFallback is a plain tile with the tool's initial, for a card
// whose icon is missing from the art catalog.
func drawIconFallback(screen *ebiten.Image, t archTool, x, y float32) {
	vector.FillRect(screen, x, y, 40, 40, uiBtnDisabled, false)
	vector.StrokeRect(screen, x, y, 40, 40, 1, uiBtnBorder, false)
	if t.label != "" {
		drawLabel(screen, int(x)+16, int(y)+15, t.label[:1])
	}
}
