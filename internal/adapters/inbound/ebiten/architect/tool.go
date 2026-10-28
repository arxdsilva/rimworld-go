package architect

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// Tool is the Architect (ARCH-005): the panel, the tool in hand and its
// drag. Its parts each own one seam: Zones (the selected zone's pane and
// Expand/Shrink tools), Areas (the allowed-area pickers and manager),
// Mover (the building Move mode) and Marks (the overlays and outlines a
// held tool shows).
type Tool struct {
	sh Shell

	// The state other root files read until the sweep repoints them to
	// methods (ARCH-005 wave 5.3; the lead moved these from App).
	Open           bool
	Cat            string // the open category
	ToolID         string // the tool in hand ("" none)
	Facing         uint8
	Stuff          string // picked material ("wood", "granite blocks", "steel", a carpet)
	Crop           string // crop the growing-zone tool sows
	Drag           bool
	X0, Y0, X1, Y1 int  // the drag box, in tiles
	Ctrl           bool // Ctrl held: the drag box is square (drawstyle.go)
	// Style is the drag tool's draw style (StyleDefault: the tool's own).
	Style DrawStyle

	desc  descState // the description box, measured (panel.go)
	locks lockCache // the scenario's locked buildings (locks.go)

	Zones *Zones
	Areas *Areas
	Mover *Mover
	Marks *Marks
}

// New builds the feature over its shell.
func New(sh Shell) *Tool {
	t := &Tool{sh: sh}
	t.Zones = &Zones{Tool: t}
	t.Areas = &Areas{Tool: t}
	t.Mover = &Mover{Tool: t}
	t.Marks = &Marks{Tool: t}
	return t
}

// sim is the colony's simulation (nil off the colony).
func (t *Tool) sim() Sim { return t.sh.Env().Sim }

// frame is this Update's input.
func (t *Tool) frame() *inputframe.Frame { return t.sh.Kit().Host().Frame() }

// rotateKeys is the live rotate keys, "Q/E" by default.
func (t *Tool) rotateKeys() string {
	k := t.sh.Env().Keys
	return k.Label(input.RotateLeft) + "/" + k.Label(input.RotateRight)
}

// archMaxGhost caps the cells a placement preview judges.
const archMaxGhost = 400

func (t *Tool) selectedCrop() game.CropView {
	crops := ZoneSpecies()
	for _, c := range crops {
		if c.Name == t.Crop {
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
	width := func(k int) float32 { return (InnerW - archBtnGap*float32(k-1)) / float32(k) }
	need := 0
	if !bv.Carpet { // carpet labels carry counts and keep their three per row
		for _, s := range bv.Stuffs {
			need = max(need, int(gfx.TextW(StuffLabel(s), gfx.Body))+16)
		}
	}
	for perRow > 1 && width(perRow) < float32(need) {
		perRow--
	}
	bw = min(max(84, float32(need)), width(perRow))
	return perRow, (n + perRow - 1) / perRow, bw
}

// Rect is the Architect's area on the HUD: the description box above the
// category panel (zero while it is closed).
func (t *Tool) Rect() uikit.Button {
	if !t.Open {
		return uikit.Button{}
	}
	l := t.sh.Layout()
	r := hudlayout.BoundsOf([]gfx.Rect{l.ArchCats, l.ArchDesc})
	return uikit.Button{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

// Buttons are the category buttons and, for the armed tool, its material
// and crop buttons. The tools themselves are command buttons in the grid
// (Cards).
func (t *Tool) Buttons() []uikit.Button {
	if !t.Open {
		return nil
	}
	l := t.sh.Layout()
	if l.ArchCats.Empty() {
		return nil
	}
	out := make([]uikit.Button, 0, len(Categories)+16)
	for i, c := range Categories {
		col, row := i%2, i/2
		out = append(out, uikit.Button{
			ID: ids.PrefixArchCat + c.ID, Label: c.Label,
			X: l.ArchCats.X + archPad + float32(col)*(archCatW+archCatGap),
			Y: l.ArchCats.Y + archPad + float32(row)*(archCatH+archCatGap),
			W: archCatW, H: archCatH, Selected: t.Cat == c.ID,
		})
	}
	if !l.ArchDesc.Empty() {
		out = append(out, t.descButtons(l.ArchDesc)...)
	}
	return out
}

// StuffLabel is a material button's text: "Wood", "Granite blocks",
// "Steel", or the carpet's material ("Cloth" for "cloth carpet").
func StuffLabel(s string) string {
	s = strings.TrimSuffix(s, " carpet")
	return strings.ToUpper(s[:1]) + s[1:]
}

// ToolStuff is the material the tool will build with: the player's pick
// when the buildable offers it, else its default (first) material.
func (t *Tool) ToolStuff(d ToolDef) string {
	bv, ok := Buildable(d.Build)
	if !ok || len(bv.Stuffs) == 0 {
		return "wood"
	}
	for _, s := range bv.Stuffs {
		if s == t.Stuff {
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

// TooltipLines is the hover card for tool d with material stuff: name,
// description, cost and work, a stock warning, and for carpets the dirt
// it catches plus where to lay it. wood is the colony's wood.
func TooltipLines(d ToolDef, stuff string, wood, stock int) []string {
	lines := []string{d.Label, d.Desc}
	bv, ok := Buildable(d.Build)
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
	if pl := PowerLine(bv); pl != "" { // PLAY-021: draw, output or store
		lines = append(lines, pl)
	}
	if stuff == "wood" && bv.Wood > wood {
		lines = append(lines, fmt.Sprintf("Only %d wood in the colony", wood))
	}
	return lines
}

// StockLine is the Architect's stock line: wood, plus the selected
// material's free stock ("Granite blocks 40") or, with no stone or steel
// material picked, the colony's stone blocks.
func StockLine(w game.WorldView, wood int, stuff string) string {
	if stuff == "" || stuff == "wood" {
		return fmt.Sprintf("Wood %d   Stone %d", wood, w.Stone)
	}
	return fmt.Sprintf("Wood %d   %s %d", wood, StuffLabel(stuff), StockOf(w, stuff))
}

// StockOf counts the free (on the ground, not carried, not forbidden)
// items of kind on the map.
func StockOf(w game.WorldView, kind string) int {
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

// fmtTemp renders tenths of a degree C as whole degrees ("-3C"), as the
// temperature readouts do (climate_ui.go).
func fmtTemp(tenths int) string {
	if tenths < 0 {
		return fmt.Sprintf("%dC", (tenths-5)/10)
	}
	return fmt.Sprintf("%dC", (tenths+5)/10)
}

// CropLines is the crop picker's stat lines for c (crops and the tree).
func CropLines(c game.CropView) []string {
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

// OpenPanel opens the Architect (closing the main-tab window and the
// float menu) on the category it was left on, Structure the first time.
func (t *Tool) OpenPanel() {
	t.Open = true
	t.sh.Do(ports.CloseWindows{})
	t.sh.CloseFloat()
	if t.Cat == "" {
		t.Cat, t.desc.dflt = "structure", "structure"
	}
}

// ClosePanel closes the Architect and drops its tool.
func (t *Tool) ClosePanel() {
	t.Open = false
	t.ToolID = ""
	t.Drag = false
	t.sh.SyncOverlays()
}

// OpenedOn is the category the Architect opened on by itself, until the
// player picks a category or a tool ("" then).
func (t *Tool) OpenedOn() string { return t.desc.dflt }

// SelectCategory opens category id; the open category again collapses it
// (no tools shown, none armed). The category the Architect opened on by
// itself does not count as chosen: clicking it first confirms it.
func (t *Tool) SelectCategory(id string) {
	defer t.sh.SyncOverlays()
	dflt := t.desc.dflt
	t.desc.dflt = ""
	if t.Cat == id && dflt != id {
		t.Cat, t.ToolID, t.Drag = "", "", false
		return
	}
	t.Cat = id
	if d, ok := ToolByID(t.ToolID); !ok || d.Category != id {
		t.ToolID = ""
		t.Drag = false
	}
}

// SelectTool arms tool id (the armed card again disarms it); an
// allowed-area tool opens its area submenu instead.
func (t *Tool) SelectTool(id string) {
	d, ok := ToolByID(id)
	if !ok {
		return
	}
	if t.Areas.toolPick(id) { // area_menu.go: the allowed-area tools open a submenu
		return
	}
	t.desc.dflt = "" // the player is working in the open category
	t.Cat = d.Category
	defer t.sh.SyncOverlays()
	if t.ToolID == id {
		t.ToolID = "" // clicking the selected card toggles it off
		return
	}
	t.ToolID = id
	t.Drag = false
	t.Style = StyleDefault
}

// Icon is the ui_icon art named name (nil without art).
func (t *Tool) Icon(name string) *ebiten.Image {
	c := t.sh.Env().Art
	if c == nil {
		return nil
	}
	return c.Image("ui_icon/"+name, color.NRGBA{})
}

// DrawIconCS is gfx.Icon multiplied by cs (a stone item's tint).
func DrawIconCS(dst, img *ebiten.Image, x, y, size float64, cs sprite.ColorScale) {
	if img == nil {
		return
	}
	s := size / float64(img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	if cs != sprite.NoCS {
		op.ColorScale.Scale(cs[0], cs[1], cs[2], cs[3])
	}
	gfx.DrawImage(dst, img, op)
}

// Draw draws the category panel and the description box; the tools are
// drawn by the command grid.
func (t *Tool) Draw(screen *ebiten.Image) {
	if !t.Open {
		return
	}
	l := t.sh.Layout()
	if l.ArchCats.Empty() {
		return
	}
	uikit.DrawPanel(screen, l.ArchCats.X, l.ArchCats.Y, l.ArchCats.W, l.ArchCats.H)
	if d := l.ArchDesc; !d.Empty() {
		uikit.DrawPanel(screen, d.X, d.Y, d.W, d.H)
		for _, ln := range t.Plan().Lines {
			if ln.Y+gfx.LineStep(ln.Style) > d.H {
				continue // a clamped box on a small screen shows what fits
			}
			gfx.Text(screen, ln.Text, float64(d.X+archPad), float64(d.Y+ln.Y), ln.Style)
		}
	}
	mx, my := t.sh.Cursor()
	for _, b := range t.Buttons() {
		uikit.DrawButton(screen, b, b.Contains(mx, my))
	}
}

// ---------------------------------------------------------- placement

// RotateTool turns the facing a build tool places with.
func (t *Tool) RotateTool(dir int) {
	t.Facing = RotateFacing(t.Facing, dir)
}

// HandleInput runs while a tool is selected: drag to place, right-click
// to stop (taking the click, so no colonist is ordered too), Q/E to
// rotate or change the draw style.
func (t *Tool) HandleInput() {
	defer t.sh.SyncOverlays() // right-click and Escape drop the tool; keep the overlay in step
	if !t.Open || t.ToolID == "" || t.sim() == nil {
		return
	}
	f := t.frame()
	if f.Fired(input.RotateLeft) {
		t.rotateOrStyle(-1)
	}
	if f.Fired(input.RotateRight) {
		t.rotateOrStyle(1)
	}
	t.Ctrl = f.Ctrl()
	if f.Clicked(inputframe.Right) {
		f.Take(inputframe.Right)
		if t.Drag {
			t.Drag = false
		} else {
			t.ToolID = ""
		}
		return
	}
	mx, my := t.sh.Cursor()
	sw, sh := t.sh.Size()
	onMap := !t.sh.ChromeHit(mx, my, sw, sh)
	pmx, pmy := t.sh.MapCursor()
	tx, ty := t.sh.TileAt(pmx, pmy)
	if onMap && f.Clicked(inputframe.Left) {
		f.Take(inputframe.Left)
		t.Drag = true
		t.X0, t.Y0 = tx, ty
		t.X1, t.Y1 = tx, ty
	}
	if t.Drag && f.Down(inputframe.Left) && onMap {
		t.X1, t.Y1 = tx, ty
	}
	if t.Drag && f.Released(inputframe.Left) {
		if onMap {
			t.X1, t.Y1 = tx, ty
		}
		t.Submit()
		t.Drag = false
	}
}

// GhostCell is one cell of a tool's preview.
type GhostCell struct {
	X, Y   int
	OK     bool
	Reason string
	// Replaces names the wall or fence a door/gate here takes the place of,
	// or the swap label ("wood wall → steel") for an in-place material swap.
	Replaces string
	// Facing is the facing the sim would store here (door/gate auto-orient).
	Facing string
	// Same: the same piece in the same material is already here; the cell
	// is neutral, does nothing and is not submitted.
	Same bool
}

// isSwapLabel reports a material-swap label; the "→" marks it.
func isSwapLabel(s string) bool { return strings.Contains(s, "→") }

// PreviewCells is what the current tool would affect right now: the
// dragged span, or the hovered cell when not dragging. An area tool in
// the rectangle style gets its two corners; in the oval style every cell.
func (t *Tool) PreviewCells() (ToolDef, []GhostCell) {
	d, ok := ToolByID(t.ToolID)
	sim := t.sim()
	if !ok || sim == nil {
		return ToolDef{}, nil
	}
	x0, y0, x1, y1 := t.DragSpan()
	style := t.CurStyle(d)
	if d.Mode == PlaceArea {
		if style != StyleOval {
			return d, []GhostCell{{X: x0, Y: y0, OK: true}, {X: x1, Y: y1, OK: true}}
		}
		var out []GhostCell
		for _, c := range ShapeCells(style, [2]int{x0, y0}, [2]int{x1, y1}, false) {
			out = append(out, GhostCell{X: c[0], Y: c[1], OK: true})
		}
		return d, out
	}
	cells := ShapeCells(style, [2]int{x0, y0}, [2]int{x1, y1}, false)
	if len(cells) > archMaxGhost {
		cells = cells[:archMaxGhost]
	}
	out := make([]GhostCell, 0, len(cells))
	stuff := t.ToolStuff(d)
	for _, c := range cells {
		fc := FacingNames[t.Facing]
		ok, reason := sim.CanPlaceAt(d.Build, stuff, fc, c[0], c[1])
		gc := GhostCell{X: c[0], Y: c[1], OK: ok, Reason: reason,
			Facing: sim.AutoFacingAt(d.Build, fc, c[0], c[1])}
		if !ok && reason == game.ReasonSame {
			gc.Same, gc.Reason = true, ""
		}
		if ok {
			gc.Replaces = sim.ReplacesAt(d.Build, stuff, c[0], c[1])
		}
		out = append(out, gc)
	}
	return d, out
}

// DragSpan is the drag's two corners (the hovered cell twice when not
// dragging), the end squared while Ctrl is held on a box style.
func (t *Tool) DragSpan() (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = t.X0, t.Y0, t.X1, t.Y1
	if !t.Drag {
		x0, y0 = t.sh.TileAt(t.sh.MapCursor())
		x1, y1 = x0, y0
	}
	if d, ok := ToolByID(t.ToolID); ok && t.Ctrl && t.CurStyle(d) != StyleLine && len(ToolStyles(d)) > 0 {
		x1, y1 = constrainEnd(x0, y0, x1, y1)
	}
	return x0, y0, x1, y1
}

// areaOval reports the area tool draws an oval.
func (t *Tool) areaOval(d ToolDef) bool { return d.Mode == PlaceArea && t.CurStyle(d) == StyleOval }

// Submit sends the drag's commands: one rectangle (or one per oval row)
// for an area tool, one DesignateBuild per placeable cell otherwise.
func (t *Tool) Submit() {
	if t.sim() == nil {
		return
	}
	d, cells := t.PreviewCells()
	if d.ID == "" {
		return
	}
	if d.Mode == PlaceArea {
		x0, y0, x1, y1 := t.X0, t.Y0, t.X1, t.Y1
		if t.Ctrl {
			x1, y1 = constrainEnd(x0, y0, x1, y1)
		}
		runs := []RowRun{{Y: -1}}
		if t.areaOval(d) {
			cs := make([][2]int, 0, len(cells))
			for _, c := range cells {
				cs = append(cs, [2]int{c.X, c.Y})
			}
			runs = RowRuns(cs)
		}
		if kind := ZoneKindOf(d); kind != "" {
			if why, free := ZoneDragRefusal(x0, y0, x1, y1, func(x, y int) string { return t.Zones.RefusalAt(kind, x, y) }); !free {
				t.sh.Hint(ZoneDragHint(why)) // the command still goes out; the sim ignores it
			}
		}
		for _, run := range runs {
			c := command.Command{Kind: d.Cmd, X: x0, Y: y0, X2: x1, Y2: y1, Enabled: d.Enable}
			if d.Cmd == command.PaintArea {
				c.Index = t.Areas.PaintIndex() // area_manager.go: the area picked in the submenu
			}
			if run.Y >= 0 { // one oval row
				c.X, c.Y, c.X2, c.Y2 = run.X0, run.Y, run.X1, run.Y
			}
			if d.Crop != "" {
				c.Build, _ = game.ZoneSpeciesID(t.selectedCrop().Name)
			}
			if d.Variant != 0 {
				c.Build = d.Variant // a dumping stockpile (PLAY-023)
			}
			t.sh.Enqueue(c)
		}
		return
	}
	id, ok := KindID(d.Build)
	if !ok {
		return
	}
	stuff, _ := game.StuffID(t.ToolStuff(d))
	for _, c := range cells {
		if c.OK && !c.Same {
			t.sh.Enqueue(command.Command{Kind: command.DesignateBuild, Build: id, Facing: t.Facing, Stuff: stuff, X: c.X, Y: c.Y})
		}
	}
}

// DrawGhost previews the tool in hand on the map under the cursor.
func (t *Tool) DrawGhost(screen *ebiten.Image) {
	if t.sim() == nil || !t.Open || t.ToolID == "" {
		return
	}
	mx, my := t.sh.Cursor()
	sw, sh := t.sh.Size()
	if !t.Drag && t.sh.ChromeHit(mx, my, sw, sh) {
		return
	}
	d, cells := t.PreviewCells()
	if t.areaOval(d) {
		kind := ZoneKindOf(d)
		for _, c := range cells {
			if kind != "" {
				t.Zones.drawGhost(screen, kind, c.X, c.Y, c.X, c.Y)
			} else {
				t.sh.DrawAreaGhost(screen, c.X, c.Y, c.X, c.Y, false)
			}
		}
		if kind != "" {
			t.Zones.drawRefusalTip(screen, kind, mx, my)
		}
		return
	}
	if d.Mode == PlaceArea {
		if kind := ZoneKindOf(d); kind != "" {
			t.Zones.drawGhost(screen, kind, cells[0].X, cells[0].Y, cells[1].X, cells[1].Y)
			t.Zones.drawRefusalTip(screen, kind, mx, my)
			return
		}
		t.sh.DrawAreaGhost(screen, cells[0].X, cells[0].Y, cells[1].X, cells[1].Y, d.Cmd == command.DesignateRoof)
		return
	}
	stuff := t.ToolStuff(d)
	t.sh.DrawPlacementGhost(screen, d.Build, stuff, FacingNames[t.Facing], cells)
	t.Marks.drawPowerGhostLinks(screen, d, cells)      // power.go: the link a powered building would make
	if rad, ok := overlay.PlacementRing(d.Build); ok { // the beacon's pickup radius
		for _, c := range cells {
			if c.OK && !c.Same {
				t.sh.DrawRangeRing(screen, c.X, c.Y, rad, overlay.TokenNRGBA(uikit.Accent))
			}
		}
	}
	valid, firstBad, replaces := 0, "", ""
	for _, c := range cells {
		if c.Same {
			continue // nothing to do here: not counted, not an error
		}
		if c.OK {
			valid++
			if replaces == "" {
				replaces = c.Replaces
			}
		} else if firstBad == "" {
			firstBad = c.Reason
		}
	}
	bv, _ := Buildable(d.Build)
	msg := fmt.Sprintf("%s x%d  %d %s", d.Label, valid, valid*bv.Wood, stuff)
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
	cursorTip(screen, msg, mx, my, sw)
}

// cursorTip is a one-line label on a panel beside the cursor, kept on
// screen.
func cursorTip(screen *ebiten.Image, msg string, mx, my, sw int) {
	mw := int(gfx.TextW(msg, gfx.Body))
	lx := min(mx+16, sw-mw-18) // keep the label on screen
	gfx.FillRect(screen, float32(lx), float32(my+18), float32(mw+12), gfx.LineStep(gfx.Body)+4, uikit.Panel, false)
	uikit.DrawLabel(screen, lx+6, my+20, msg)
}

// Preset opens the Architect with the build tool for kind selected (never
// toggling it off), the material and facing preset, as the building
// panel's Copy does. False when no tool builds kind.
func (t *Tool) Preset(kind, stuff, facing string) bool {
	for _, d := range AllTools() {
		if d.Build != kind {
			continue
		}
		t.OpenPanel()
		t.Cat = d.Category
		t.ToolID = d.ID
		t.Drag = false
		t.Style = StyleDefault
		if bv, ok := Buildable(kind); ok && len(bv.Stuffs) > 1 {
			t.Stuff = bv.Stuffs[0]
			for _, s := range bv.Stuffs {
				if s == stuff {
					t.Stuff = s
				}
			}
		}
		for i, n := range FacingNames {
			if n == facing {
				t.Facing = uint8(i)
			}
		}
		t.sh.SyncOverlays()
		return true
	}
	return false
}

// ZoneKindOf maps a zone tool to the sim's zone kind ("" for other tools).
func ZoneKindOf(d ToolDef) string {
	switch d.Cmd {
	case command.DesignateStockpile:
		if d.Variant == game.StockpileVariantDump {
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

// DrawIconFallback is a plain tile with the tool's initial, for a button
// whose icon is missing from the art catalog.
func DrawIconFallback(screen *ebiten.Image, d ToolDef, x, y float32) {
	gfx.FillRect(screen, x, y, 40, 40, uikit.BtnDisabled, false)
	gfx.StrokeRect(screen, x, y, 40, 40, 1, uikit.BtnBorder, false)
	if d.Label != "" {
		uikit.DrawLabel(screen, int(x)+16, int(y)+15, d.Label[:1])
	}
}
