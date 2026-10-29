// The selection's map side (UI-016): the left-button gestures (select, Shift-toggle, the drag box, the double click), the marks drawn over what is selected on the map, and the icon helpers that read the renderer's sprites. The inspector itself lives in inspector/ (ARCH-005); these draw in map pixels with the renderer, so they stay with it.
package ebiten

import (
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// leftMouse adapts the input frame's left button to selection.Mouse.
type leftMouse struct{ f *inputframe.Frame }

func (m leftMouse) Clicked() bool { return m.f.Clicked(inputframe.Left) }

func (m leftMouse) Take() { m.f.Take(inputframe.Left) }

func (m leftMouse) Down() bool { return m.f.Down(inputframe.Left) }

func (m leftMouse) Shift() bool { return m.f.Shift() }

// handleMapMouse is the map's left button (select, Shift-toggle, box,
// double click) and then the right button (orders, formation.go).
func (a *App) handleMapMouse() {
	a.sel.HandleMapMouse(leftMouse{&a.in}, a.selectSameOnScreen, a.boxSelect)
}

// screenRect normalizes two screen corners.
func screenRect(x0, y0, x1, y1 int) (float32, float32, float32, float32) {
	return float32(min(x0, x1)), float32(min(y0, y1)), float32(max(x0, x1)), float32(max(y0, y1))
}

// inRect reports the centre of world cell (wx, wy) is drawn inside the
// screen rectangle.
func (r *Renderer) inRect(wx, wy float64, x0, y0, x1, y1 float32) bool {
	sx, sy := r.worldToScreenF(wx+0.5, wy+0.5)
	return sx >= x0 && sx <= x1 && sy >= y0 && sy <= y1
}

// bodyInRect reports the screen rectangle holding the centre of a tile
// an's drawn body covers (PLAY-022 C7: a box over a big animal's flank
// takes it, as a click there does). An animal whose cell is farther from
// the box than the largest body reaches is not measured.
func (r *Renderer) bodyInRect(an game.AnimalView, x0, y0, x1, y1 float32) bool {
	tile := r.cam.Tile()
	pad, padS := r.animalPads(tile)
	// the tile centres inside the box
	tx0 := int(math.Ceil((float64(x0)+r.cam.OffsetX)/tile - 0.5))
	ty0 := int(math.Ceil((float64(y0)+r.cam.OffsetY)/tile - 0.5))
	tx1 := int(math.Floor((float64(x1)+r.cam.OffsetX)/tile - 0.5))
	ty1 := int(math.Floor((float64(y1)+r.cam.OffsetY)/tile - 0.5))
	if tx1 < tx0 || ty1 < ty0 || an.X < tx0-pad || an.X > tx1+pad || an.Y < ty0-pad || an.Y > ty1+padS {
		return false
	}
	bx0, by0, bx1, by1, ok := r.animalBodyRect(an)
	if !ok {
		return false
	}
	// the tiles the body spans whose centres the box holds
	cx0, cy0 := int(math.Floor(bx0)), int(math.Floor(by0))
	cx1, cy1 := int(math.Floor(bx1)), int(math.Floor(by1))
	for ty := max(cy0, ty0); ty <= min(cy1, ty1); ty++ {
		for tx := max(cx0, tx0); tx <= min(cx1, tx1); tx++ {
			if coversTile(bx0, by0, bx1, by1, tx, ty) {
				return true
			}
		}
	}
	return false
}

// thingsIn lists what lies inside a screen rectangle: colonists, else
// animals, else ground items (each in read-model order).
func (a *App) thingsIn(x0, y0, x1, y1 float32) (selectionpkg.Kind, []entity.EntityID) {
	r := a.renderer
	var ids []entity.EntityID
	w := r.world()
	for _, p := range r.snapshot() {
		if wx, wy := r.interpWorld(p); !p.Embodied && !overlaypkg.WorldFogged(w, p.X, p.Y) && r.inRect(wx, wy, x0, y0, x1, y1) {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) > 0 {
		return selectionpkg.Pawn, ids
	}
	for i := range w.Animals {
		an := &w.Animals[i]
		if overlaypkg.WorldFogged(w, an.X, an.Y) {
			continue
		}
		if wx, wy := r.interpAnimal(*an); r.inRect(wx, wy, x0, y0, x1, y1) || r.bodyInRect(*an, x0, y0, x1, y1) {
			ids = append(ids, an.ID)
		}
	}
	if len(ids) > 0 {
		return selectionpkg.Animal, ids
	}
	for _, it := range w.Items {
		if !it.Carried && !overlaypkg.WorldFogged(w, it.X, it.Y) && r.inRect(float64(it.X), float64(it.Y), x0, y0, x1, y1) {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) > 0 {
		return selectionpkg.Item, ids
	}
	return selectionpkg.None, nil
}

// boxSelect selects what the dragged box covers; with add (Shift) it joins
// a selection of the same kind.
func (a *App) boxSelect(sx0, sy0, sx1, sy1 int, add bool) {
	kind, ids := a.thingsIn(screenRect(sx0, sy0, sx1, sy1))
	a.sel.ApplyBox(kind, ids, add)
}

// selectSameOnScreen is the double click: every visible colonist, animal
// of the same species, or stack of the same item.
func (a *App) selectSameOnScreen() {
	c, ok := a.selectedCand()
	if !ok || !selectionpkg.Groupable(c.Kind) {
		a.sel.ClickSelect()
		return
	}
	v := a.mapViewport() // physical, like worldToScreenF
	x0, y0, x1, y1 := float32(v.X0), float32(v.Y0), float32(v.X1), float32(v.Y1)
	r := a.renderer
	w := r.world()
	var ids []entity.EntityID
	switch c.Kind {
	case selectionpkg.Pawn:
		for _, p := range r.snapshot() {
			if wx, wy := r.interpWorld(p); !p.Embodied && r.inRect(wx, wy, x0, y0, x1, y1) {
				ids = append(ids, p.ID)
			}
		}
	case selectionpkg.Animal:
		an, _ := a.insp.SelectedAnimal()
		for _, o := range w.Animals {
			if wx, wy := r.interpAnimal(o); o.Species == an.Species && r.inRect(wx, wy, x0, y0, x1, y1) {
				ids = append(ids, o.ID)
			}
		}
	case selectionpkg.Item:
		it, _ := a.sel.SelectedItem()
		for _, o := range w.Items {
			if !o.Carried && o.Kind == it.Kind && r.inRect(float64(o.X), float64(o.Y), x0, y0, x1, y1) {
				ids = append(ids, o.ID)
			}
		}
	}
	a.sel.SetGroup(c.Kind, append([]entity.EntityID{c.ID}, ids...))
}

// drawGroupMarks outlines every group member on the map.
func (a *App) drawGroupMarks(screen *ebiten.Image) {
	if !a.sel.Grouped() || a.renderer == nil {
		return
	}
	r := a.renderer
	tile := float32(r.cam.Tile())
	mark := func(wx, wy float64) {
		sx, sy := r.worldToScreenF(wx, wy)
		vector.StrokeRect(screen, sx+1, sy+1, tile-2, tile-2, 2, r.selectClr, false)
	}
	switch a.sel.Group.Kind {
	case selectionpkg.Pawn:
		for _, p := range a.sel.SelectedPawns() {
			mark(r.interpWorld(p))
		}
	case selectionpkg.Animal:
		for _, an := range a.sel.SelectedAnimals() {
			mark(r.interpAnimal(an))
		}
	case selectionpkg.Item:
		for _, it := range a.sel.SelectedItems() {
			mark(float64(it.X), float64(it.Y))
		}
	}
}

// drawBoxSelect draws the drag box while the left button is held.
func (a *App) drawBoxSelect(screen *ebiten.Image) {
	if !a.sel.Press.On || !a.sel.Press.Box {
		return
	}
	// The press and cursor are physical (map pixels); the box is UI-canvas
	// drawing, so its corners go to logical.
	pmx, pmy := a.mapCursor()
	lx0, ly0 := a.mapToLogical(a.sel.Press.X, a.sel.Press.Y)
	lx1, ly1 := a.mapToLogical(pmx, pmy)
	x0, y0, x1, y1 := screenRect(lx0, ly0, lx1, ly1)
	fill := uikit.Accent
	fill.A = 40
	gfx.FillRect(screen, x0, y0, x1-x0, y1-y0, fill, false)
	gfx.StrokeRect(screen, x0, y0, x1-x0, y1-y0, 1, uikit.Accent, false)
}

// drawAnimalMark draws the selection mark on the animal's tile; the
// panel itself is the inspect pane (animalPane).
func (a *App) drawAnimalMark(screen *ebiten.Image) {
	an, ok := a.insp.SelectedAnimal()
	if !ok || a.win.Open || a.arch.Open {
		return
	}
	if a.renderer.art != nil {
		wx, wy := a.renderer.interpAnimal(an)
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, wx, wy, a.shell.TilePx(), a.pw, a.ph) // physical, like the map
	}
}

// drawFrameMark marks the selected blueprint's tiles on the map. The
// pane (cmds_build.go) is the blueprint's panel now.
func (a *App) drawFrameMark(screen *ebiten.Image) {
	f, ok := a.insp.SelectedFrame()
	if !ok || a.win.Open || a.arch.Open {
		return
	}
	if a.renderer.art != nil {
		bw, bh := max(f.W, 1), max(f.H, 1)
		for dy := 0; dy < bh; dy++ {
			for dx := 0; dx < bw; dx++ {
				a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, float64(f.X+dx), float64(f.Y+dy), a.shell.TilePx(), a.pw, a.ph) // physical, like the map
			}
		}
	}
}

// "cloth carpet" -> cloth_carpet

// drawItemMark draws the selection mark on the stack's tile; the
// panel itself is the inspect pane (itemPane).
func (a *App) drawItemMark(screen *ebiten.Image) {
	it, ok := a.sel.SelectedItem()
	if !ok || a.win.Open || a.arch.Open {
		return
	}
	if a.renderer.art != nil {
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, float64(it.X), float64(it.Y), a.shell.TilePx(), a.pw, a.ph) // physical, like the map
	}
}

// plantIconKey is the sprite key of the plant's icon.
func (a *App) plantIconKey(p game.PlantView) string {
	key, _ := worldpkg.PlantSprite(p)
	if a.renderer.art != nil {
		key = a.renderer.seasonKey(key, a.shell.World())
	}
	return key
}

// drawPlantMark draws the selection mark on the plant's tile; the
// panel itself is the inspect pane (plantPane).
func (a *App) drawPlantMark(screen *ebiten.Image) {
	p, ok := a.sel.SelectedPlant()
	if !ok || a.win.Open || a.arch.Open {
		return
	}
	if a.renderer.art != nil {
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, float64(p.X), float64(p.Y), a.shell.TilePx(), a.pw, a.ph) // physical, like the map
	}
}

// drawBuildingMark marks the selected building on the map. The pane
// (cmds_build.go) is the building's panel now.
func (a *App) drawBuildingMark(screen *ebiten.Image) {
	b, ok := a.insp.SelectedBuilding()
	if !ok || a.win.Open || a.arch.Open {
		return
	}
	a.drawBuildingSelection(screen, b)
}

func (a *App) drawBuildingSelection(screen *ebiten.Image, b game.BuildingView) {
	tile := a.shell.TilePx()
	sx, sy := a.shell.WorldToScreen(b.X, b.Y)
	w, h := float32(float64(max(b.W, 1))*tile), float32(float64(max(b.H, 1))*tile)
	vector.StrokeRect(screen, sx+1, sy+1, w-2, h-2, 2, uikit.Accent, false)
	if b.Kind == overlaypkg.SentryTurretKind { // turret_draw.go: its reach
		a.renderer.drawRangeRing(screen, b.X, b.Y, overlaypkg.TurretRangeTiles, overlaypkg.TokenNRGBA(uikit.Accent))
	}
	if b.HasSpot {
		px, py := a.shell.WorldToScreen(b.SpotX, b.SpotY)
		vector.StrokeRect(screen, px+tile32(tile)*0.3, py+tile32(tile)*0.3, tile32(tile)*0.4, tile32(tile)*0.4, 1, uikit.Accent, false)
	}
}

func tile32(t float64) float32 { return float32(t) }
