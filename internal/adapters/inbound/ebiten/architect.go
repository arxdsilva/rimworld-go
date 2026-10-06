package ebiten

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

const (
	archW        = 276
	archCardW    = 126
	archCardH    = 58
	archCardGap  = 8
	archTop      = 64 // cards start below header + category row
	archInfoH    = 84
	archMaxGhost = 400
)

// Crop picker (growing-zone tool): 4 crops per row, then a stats line.
const (
	archCropCols = 4
	archCropBtnW = 62
	archCropBtnH = 22
	archCropH    = 2*(archCropBtnH+4) + 56
)

// archCardRows is the card rows the current category needs (min 2).
func (a *App) archCardRows() int {
	rows := (len(toolsIn(a.archCat)) + 1) / 2
	if rows < 2 {
		rows = 2
	}
	return rows
}

// archCropOpen reports the selected tool sows a crop.
func (a *App) archCropOpen() bool {
	t, ok := toolByID(a.archTool)
	return ok && t.crop != ""
}

func (a *App) selectedCrop() game.CropView {
	crops := game.Crops()
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

func (a *App) architectRect() button {
	h := archTop + a.archCardRows()*(archCardH+archCardGap) + archInfoH
	if a.archCropOpen() {
		h += archCropH
	}
	return button{x: hudPad, y: hudTopH + hudPad, w: archW, h: float32(h)}
}

// archCropTop is the y of the crop picker block.
func (a *App) archCropTop(r button) float32 {
	return r.y + archTop + float32(a.archCardRows()*(archCardH+archCardGap))
}

func (a *App) architectButtons() []button {
	if !a.archOpen {
		return nil
	}
	r := a.architectRect()
	out := make([]button, 0, len(archCategories)+6)
	cx := r.x + 10
	for _, c := range archCategories {
		w := float32(len(c.label)*7 + 14)
		out = append(out, button{
			id: "arch.cat." + c.id, label: c.label, x: cx, y: r.y + 30, w: w, h: 22,
			selected: a.archCat == c.id,
		})
		cx += w + 4
	}
	for i, t := range toolsIn(a.archCat) {
		col, row := i%2, i/2
		out = append(out, button{
			id: "arch.tool." + t.id,
			x:  r.x + 10 + float32(col)*(archCardW+archCardGap),
			y:  r.y + archTop + float32(row)*(archCardH+archCardGap),
			w:  archCardW, h: archCardH,
			selected: a.archTool == t.id,
		})
	}
	if a.archCropOpen() {
		sel := a.selectedCrop().Name
		top := a.archCropTop(r)
		for i, c := range game.Crops() {
			col, row := i%archCropCols, i/archCropCols
			out = append(out, button{
				id: "arch.crop." + c.Name, label: strings.ToUpper(c.Name[:1]) + c.Name[1:],
				x: r.x + 10 + float32(col)*(archCropBtnW+2), y: top + float32(row)*(archCropBtnH+4),
				w: archCropBtnW, h: archCropBtnH, selected: c.Name == sel,
			})
		}
	}
	if t, ok := toolByID(a.archTool); ok {
		if bv, ok := buildable(t.build); ok && bv.StoneOK {
			y := r.y + r.h - 28
			for i, s := range []struct{ id, label string }{{"wood", "Wood"}, {"stone", "Stone"}} {
				out = append(out, button{
					id: "arch.stuff." + s.id, label: s.label, x: r.x + 10 + float32(i)*70, y: y, w: 64, h: 22,
					selected: a.toolStuff(t) == s.id,
				})
			}
		}
	}
	return out
}

// toolStuff is the material the tool will build with.
func (a *App) toolStuff(t archTool) string {
	if bv, ok := buildable(t.build); ok && bv.StoneOK && a.archStuff == "stone" {
		return "stone"
	}
	return "wood"
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
}

func (a *App) selectArchCategory(id string) {
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
	if a.archTool == id {
		a.archTool = "" // clicking the selected card toggles it off
		return
	}
	a.archTool = id
	a.archDrag = false
}

func (a *App) icon(name string) *ebiten.Image {
	if a.art == nil {
		return nil
	}
	return a.art.image("ui_icon/"+name, color.NRGBA{})
}

func drawIcon(dst, img *ebiten.Image, x, y, size float64) {
	if img == nil {
		return
	}
	s := size / float64(img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(img, op)
}

func (a *App) drawArchitect(screen *ebiten.Image) {
	if !a.archOpen {
		return
	}
	r := a.architectRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+8, "Architect")
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
		drawIcon(screen, a.icon(t.icon), float64(b.x)+6, float64(b.y)+8, 40)
		drawLabel(screen, int(b.x)+50, int(b.y)+10, t.label)
		if bv, ok := buildable(t.build); ok {
			drawLabel(screen, int(b.x)+50, int(b.y)+30, fmt.Sprintf("%d %s", bv.Wood, a.toolStuff(t)))
		} else {
			drawLabel(screen, int(b.x)+50, int(b.y)+30, map[placeMode]string{placeArea: "drag area"}[t.mode])
		}
		if b.contains(mx, my) {
			tt := t
			hover = &tt
		}
	}
	if a.archCropOpen() {
		c := a.selectedCrop()
		y := int(a.archCropTop(r)) + 2*(archCropBtnH+4) + 2
		drawLabel(screen, int(r.x)+12, y, fmt.Sprintf("%s: %.1f days, %d %s per plant", strings.ToUpper(c.Name[:1])+c.Name[1:], c.Days, c.Yield, c.Label))
		drawLabel(screen, int(r.x)+12, y+16, fmt.Sprintf("rich %.1fd  gravel %.1fd  food %d each", c.RichDays, c.GravelDay, c.Nutrition))
		keeps := "keeps forever"
		if c.ShelfDays > 0 {
			keeps = fmt.Sprintf("keeps %.0f days", c.ShelfDays)
		}
		drawLabel(screen, int(r.x)+12, y+32, fmt.Sprintf("frost kills below %s, %s", fmtTemp(c.FrostKill), keeps))
	}
	iy := int(r.y+r.h) - archInfoH + 6
	stone := 0
	if a.renderer != nil {
		stone = a.renderer.world().Stone
	}
	drawLabel(screen, int(r.x)+12, iy, fmt.Sprintf("Wood %d   Stone %d", wood, stone))
	if t, ok := toolByID(a.archTool); ok {
		hint := map[placeMode]string{
			placeSingle: "Click to place",
			placeLine:   "Drag for a line",
			placeFill:   "Drag to fill",
			placeArea:   "Drag an area",
		}[t.mode]
		if bv, ok := buildable(t.build); ok && bv.Rotatable {
			hint += "  Q/E rotate (" + facingNames[a.archFacing] + ")"
		}
		drawLabel(screen, int(r.x)+12, iy+16, hint)
		if bv, ok := buildable(t.build); !ok || !bv.StoneOK {
			drawLabel(screen, int(r.x)+12, iy+32, "Right-click: stop placing")
		}
	} else {
		drawLabel(screen, int(r.x)+12, iy+16, "Pick something to place")
	}
	if hover != nil {
		a.drawArchTooltip(screen, *hover, wood, int(r.x+r.w)+6, int(r.y)+archTop)
	}
}

func (a *App) drawArchTooltip(screen *ebiten.Image, t archTool, wood, x, y int) {
	lines := []string{t.label, t.desc}
	if bv, ok := buildable(t.build); ok {
		lines = append(lines, fmt.Sprintf("Cost: %d wood   Work: %d", bv.Wood, bv.Work))
		if bv.Wood > wood {
			lines = append(lines, fmt.Sprintf("Only %d wood in the colony", wood))
		}
	}
	w := 0
	for _, l := range lines {
		if len(l) > w {
			w = len(l)
		}
	}
	drawPanel(screen, float32(x), float32(y), float32(w*7+20), float32(len(lines)*16+12))
	for i, l := range lines {
		drawLabel(screen, x+10, y+6+i*16, l)
	}
}

// ---------------------------------------------------------- placement

func (a *App) rotateTool(dir int) {
	a.archFacing = rotateFacing(a.archFacing, dir)
}

// handleArchitectInput runs while a tool is selected: drag to place,
// right-click to stop, Q/E to rotate.
func (a *App) handleArchitectInput(chromeConsumed bool) {
	if !a.archOpen || a.archTool == "" || a.renderer == nil {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		a.rotateTool(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		a.rotateTool(1)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
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
	if !chromeConsumed && onMap && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		a.archDrag = true
		a.archX0, a.archY0 = tx, ty
		a.archX1, a.archY1 = tx, ty
	}
	if a.archDrag && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && onMap {
		a.archX1, a.archY1 = tx, ty
	}
	if a.archDrag && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
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
}

// previewCells is what the current tool would affect right now: the
// dragged span, or the hovered cell when not dragging.
func (a *App) previewCells() (archTool, []ghostCell) {
	t, ok := toolByID(a.archTool)
	if !ok || a.renderer == nil {
		return archTool{}, nil
	}
	x0, y0, x1, y1 := a.archX0, a.archY0, a.archX1, a.archY1
	if !a.archDrag {
		mx, my := a.currentCursor()
		x0, y0 = a.renderer.TileAt(mx, my)
		x1, y1 = x0, y0
	}
	if t.mode == placeArea {
		return t, []ghostCell{{x: x0, y: y0, ok: true}, {x: x1, y: y1, ok: true}}
	}
	cells := placementCells(t.mode, x0, y0, x1, y1)
	if len(cells) > archMaxGhost {
		cells = cells[:archMaxGhost]
	}
	out := make([]ghostCell, 0, len(cells))
	for _, c := range cells {
		ok, reason := a.renderer.game.CanPlaceAt(t.build, a.toolStuff(t), facingNames[a.archFacing], c[0], c[1])
		out = append(out, ghostCell{x: c[0], y: c[1], ok: ok, reason: reason})
	}
	return t, out
}

func (a *App) submitArchitect() {
	if a.renderer == nil {
		return
	}
	t, cells := a.previewCells()
	if t.id == "" {
		return
	}
	if t.mode == placeArea {
		c := command.Command{Kind: t.cmd, X: a.archX0, Y: a.archY0, X2: a.archX1, Y2: a.archY1}
		if t.crop != "" {
			c.Build, _ = game.CropID(a.selectedCrop().Name)
		}
		a.renderer.enqueue(c)
		return
	}
	id, ok := kindID(t.build)
	if !ok {
		return
	}
	stuff, _ := game.StuffID(a.toolStuff(t))
	for _, c := range cells {
		if c.ok {
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
	if t.mode == placeArea {
		a.renderer.drawGhostRect(screen, cells[0].x, cells[0].y, cells[1].x, cells[1].y)
		return
	}
	a.renderer.drawPlacementGhost(screen, t.build, a.toolStuff(t), facingNames[a.archFacing], cells)
	valid, firstBad := 0, ""
	for _, c := range cells {
		if c.ok {
			valid++
		} else if firstBad == "" {
			firstBad = c.reason
		}
	}
	bv, _ := buildable(t.build)
	msg := fmt.Sprintf("%s x%d  %d %s", t.label, valid, valid*bv.Wood, a.toolStuff(t))
	if firstBad != "" {
		msg += "   can't place: " + firstBad
	}
	vector.FillRect(screen, float32(mx+16), float32(my+18), float32(len(msg)*6+12), 18, uiPanel, false)
	drawLabel(screen, mx+22, my+19, msg)
}
