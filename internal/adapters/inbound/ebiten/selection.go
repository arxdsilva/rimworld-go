package ebiten

import (
	"slices"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Map selection (UI-011): a left click picks what is on the tile, in
// precedence pawn → animal → ground item → building → plant → floor;
// clicking the same tile again cycles to the next thing there. Selecting
// one kind clears every other selection.

type selKind uint8

const (
	selNone selKind = iota
	selPawn
	selAnimal
	selItem
	selBuilding
	selPlant
	selFrame
	selZone // PLAY-020: a zone (zone_select.go; the id is a zone id)
)

// selCand is one selectable thing on a tile.
type selCand struct {
	kind selKind
	id   entity.EntityID
}

// selCycle remembers the last click, for cycling through a tile's stack.
type selCycle struct {
	tile [2]int
	idx  int
	n    int
	ok   bool
}

// selectablesAt lists what a click on (tx, ty) can select, in precedence.
// selectables is selectablesAt for a click on the map: a hidden conduit
// counts only while the power overlay shows, the only place it is drawn
// (PLAY-021).
func (a *App) selectables(w game.WorldView, tx, ty int) []selCand {
	cands := selectablesAt(w, a.renderer.snapshot(), tx, ty)
	if a.renderer.powerShown() {
		return cands
	}
	return slices.DeleteFunc(cands, func(c selCand) bool {
		if c.kind != selBuilding {
			return false
		}
		for i := range w.Buildings {
			if w.Buildings[i].ID == c.id {
				return w.Buildings[i].Kind == hiddenConduitKind
			}
		}
		return false
	})
}

func selectablesAt(w game.WorldView, pawns []game.PawnView, tx, ty int) []selCand {
	var out []selCand
	if worldFogged(w, tx, ty) {
		return nil // unexplored: nothing to pick
	}
	for _, p := range pawns {
		if p.X == tx && p.Y == ty && !p.Embodied {
			out = append(out, selCand{selPawn, p.ID})
		}
	}
	for _, an := range w.Animals {
		if an.X == tx && an.Y == ty {
			out = append(out, selCand{selAnimal, an.ID})
		}
	}
	for _, it := range w.Items {
		if it.X == tx && it.Y == ty && !it.Carried {
			out = append(out, selCand{selItem, it.ID})
		}
	}
	var floor []selCand
	var floorFrames []selCand
	for _, f := range w.Frames {
		fw, fh := max(f.W, 1), max(f.H, 1)
		if tx < f.X || ty < f.Y || tx >= f.X+fw || ty >= f.Y+fh {
			continue
		}
		if f.Floor {
			floorFrames = append(floorFrames, selCand{selFrame, f.ID})
		} else {
			out = append(out, selCand{selFrame, f.ID})
		}
	}
	for _, b := range w.Buildings {
		bw, bh := max(b.W, 1), max(b.H, 1)
		if tx < b.X || ty < b.Y || tx >= b.X+bw || ty >= b.Y+bh {
			continue
		}
		if b.Floor {
			floor = append(floor, selCand{selBuilding, b.ID})
		} else {
			out = append(out, selCand{selBuilding, b.ID})
		}
	}
	for _, p := range w.Plants {
		if p.X == tx && p.Y == ty {
			out = append(out, selCand{selPlant, p.ID})
		}
	}
	out = append(out, floor...)
	out = append(out, floorFrames...)
	if z, ok := zoneCellAt(w, tx, ty); ok { // PLAY-020: a zone comes last
		out = append(out, selCand{selZone, entity.EntityID(z.ID)})
	}
	return out
}

// pawnUnderCursor is the colonist drawn under physical pixel (mx, my), even
// mid-step onto a neighbouring tile.
func (r *Renderer) pawnUnderCursor(mx, my int) (entity.EntityID, bool) {
	wx, wy := r.screenToWorld(mx, my)
	for _, p := range r.snapshot() {
		px, py := r.interpWorld(p)
		if wx >= px && wx < px+1 && wy >= py && wy < py+1 {
			return p.ID, true
		}
	}
	return 0, false
}

// clickSelect selects what is under the cursor (a map left click), in
// physical pixels (mapCursor).
func (a *App) clickSelect() {
	mx, my := a.mapCursor()
	tx, ty := a.renderer.TileAt(mx, my)
	w := a.renderer.world()
	if a.selectZoneLabel(w, mx, my, tx, ty) { // zone_select.go: a click on a zone's name plate
		a.selCycle = selCycle{}
		return
	}
	cands := a.selectables(w, tx, ty)
	tile := [2]int{tx, ty}
	a.zoneSt().noteCell(tile)
	if c := a.selCycle; c.ok && c.tile == tile && c.n == len(cands) && len(cands) > 1 {
		i := (c.idx + 1) % len(cands)
		a.selectCand(cands[i])
		a.selCycle.idx = i
		return
	}
	if id, ok := a.renderer.pawnUnderCursor(mx, my); ok {
		a.selectCand(selCand{selPawn, id})
		a.selCycle = selCycle{tile: tile, n: len(cands), ok: true}
		for i, c := range cands {
			if c.kind == selPawn && c.id == id {
				a.selCycle.idx = i
			}
		}
		return
	}
	if len(cands) == 0 {
		a.clearSelections(selNone)
		a.selCycle = selCycle{}
		return
	}
	a.selectCand(cands[0])
	a.selCycle = selCycle{tile: tile, n: len(cands), ok: true}
}

// selectCand selects c and clears every other kind of selection.
func (a *App) selectCand(c selCand) {
	a.clearSelections(c.kind)
	switch c.kind {
	case selPawn:
		a.renderer.Select(c.id)
	case selAnimal:
		a.selAnimal, a.hasAnimal = c.id, true
	case selItem:
		a.selItem, a.hasItem = c.id, true
	case selBuilding:
		a.selBuilding, a.hasBuilding = c.id, true
	case selPlant:
		a.selPlant, a.hasPlant = c.id, true
	case selFrame:
		a.selFrame, a.hasFrame = c.id, true
	case selZone:
		a.selectZone(uint16(c.id)) // zone_select.go
	}
}

// clearSelections drops every selection but kind keep, and any group.
func (a *App) clearSelections(keep selKind) {
	a.group = selGroup{}
	if keep != selPawn && a.renderer != nil {
		a.renderer.ClearSelect()
	}
	if keep != selAnimal {
		a.hasAnimal = false
	}
	if keep != selItem {
		a.hasItem = false
	}
	if keep != selBuilding {
		a.hasBuilding = false
	}
	if keep != selPlant {
		a.hasPlant = false
	}
	if keep != selFrame {
		a.hasFrame = false
	}
	if keep != selZone {
		a.clearZone() // zone_select.go: also ends its Expand and Shrink tool
	}
}

// selectPlantAt selects the plant at (tx, ty).
func (a *App) selectPlantAt(tx, ty int) bool {
	for _, p := range a.renderer.world().Plants {
		if p.X == tx && p.Y == ty {
			a.selectCand(selCand{selPlant, p.ID})
			return true
		}
	}
	return false
}

// selectItemAt selects the ground item at (tx, ty).
func (a *App) selectItemAt(tx, ty int) bool {
	for _, it := range a.renderer.world().Items {
		if it.X == tx && it.Y == ty && !it.Carried {
			a.selectCand(selCand{selItem, it.ID})
			return true
		}
	}
	return false
}

func (a *App) clearPlant() { a.hasPlant = false }
func (a *App) clearItem()  { a.hasItem = false }

// selectedPlant is the selected plant, if it still exists.
func (a *App) selectedPlant() (game.PlantView, bool) {
	if !a.hasPlant || a.renderer == nil {
		return game.PlantView{}, false
	}
	for _, p := range a.renderer.world().Plants {
		if p.ID == a.selPlant {
			return p, true
		}
	}
	a.hasPlant = false // cut, harvested or eaten
	return game.PlantView{}, false
}

// selectedItem is the selected ground item, if it is still on the ground.
func (a *App) selectedItem() (game.ItemView, bool) {
	if !a.hasItem || a.renderer == nil {
		return game.ItemView{}, false
	}
	for _, it := range a.renderer.world().Items {
		if it.ID == a.selItem && !it.Carried {
			return it, true
		}
	}
	a.hasItem = false // picked up, used or rotted away
	return game.ItemView{}, false
}
