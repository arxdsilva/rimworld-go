package ebiten

import (
	"fmt"
	"math"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Multi-selection (UI-016, MAIN_UI §9 Selection): Shift+click adds or
// removes, a left drag boxes colonists (else animals, else items), a
// double-click takes every visible thing of the same kind. A group of two
// or more replaces the single selections and gets its own panel; R, F and
// right-click orders act on every member. Presentation state only.

// leftMouse adapts the input frame's left button to selection.Mouse.
type leftMouse struct{ f *inputframe.Frame }

func (m leftMouse) Clicked() bool { return m.f.Clicked(mbLeft) }
func (m leftMouse) Take()         { m.f.Take(mbLeft) }
func (m leftMouse) Down() bool    { return m.f.Down(mbLeft) }
func (m leftMouse) Shift() bool   { return m.f.Shift() }

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
	tile := r.tilePx()
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
func (a *App) thingsIn(x0, y0, x1, y1 float32) (selKind, []entity.EntityID) {
	r := a.renderer
	var ids []entity.EntityID
	w := r.world()
	for _, p := range r.snapshot() {
		if wx, wy := r.interpWorld(p); !p.Embodied && !worldFogged(w, p.X, p.Y) && r.inRect(wx, wy, x0, y0, x1, y1) {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) > 0 {
		return selPawn, ids
	}
	for i := range w.Animals {
		an := &w.Animals[i]
		if worldFogged(w, an.X, an.Y) {
			continue
		}
		if wx, wy := r.interpAnimal(*an); r.inRect(wx, wy, x0, y0, x1, y1) || r.bodyInRect(*an, x0, y0, x1, y1) {
			ids = append(ids, an.ID)
		}
	}
	if len(ids) > 0 {
		return selAnimal, ids
	}
	for _, it := range w.Items {
		if !it.Carried && !worldFogged(w, it.X, it.Y) && r.inRect(float64(it.X), float64(it.Y), x0, y0, x1, y1) {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) > 0 {
		return selItem, ids
	}
	return selNone, nil
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
	if !ok || !groupable(c.Kind) {
		a.clickSelect()
		return
	}
	v := a.mapViewport() // physical, like worldToScreenF
	x0, y0, x1, y1 := float32(v.X0), float32(v.Y0), float32(v.X1), float32(v.Y1)
	r := a.renderer
	w := r.world()
	var ids []entity.EntityID
	switch c.Kind {
	case selPawn:
		for _, p := range r.snapshot() {
			if wx, wy := r.interpWorld(p); !p.Embodied && r.inRect(wx, wy, x0, y0, x1, y1) {
				ids = append(ids, p.ID)
			}
		}
	case selAnimal:
		an, _ := a.selectedAnimal()
		for _, o := range w.Animals {
			if wx, wy := r.interpAnimal(o); o.Species == an.Species && r.inRect(wx, wy, x0, y0, x1, y1) {
				ids = append(ids, o.ID)
			}
		}
	case selItem:
		it, _ := a.selectedItem()
		for _, o := range w.Items {
			if !o.Carried && o.Kind == it.Kind && r.inRect(float64(o.X), float64(o.Y), x0, y0, x1, y1) {
				ids = append(ids, o.ID)
			}
		}
	}
	a.setGroup(c.Kind, append([]entity.EntityID{c.ID}, ids...))
}

// groupRow is one member line of the group panel.
type groupRow struct {
	id    entity.EntityID
	label string
}

// groupRows lists the group's members (dropping ones that are gone).
func (a *App) groupRows() []groupRow {
	var out []groupRow
	switch a.sel.Group.Kind {
	case selPawn:
		for _, p := range a.selectedPawns() {
			s := pawnLabel(p)
			if p.Drafted {
				s += "  (drafted)"
			}
			if p.Status != "alive" {
				s += "  (" + p.Status + ")"
			}
			out = append(out, groupRow{p.ID, s})
		}
	case selAnimal:
		for _, an := range a.selectedAnimals() {
			s := titleCase(an.Label)
			if an.Name != "" {
				s = an.Name + ", " + an.Label
			}
			out = append(out, groupRow{an.ID, fmt.Sprintf("%s  %d%%", s, an.Health)})
		}
	case selItem:
		for _, it := range a.selectedItems() {
			s := fmt.Sprintf("%s x %d", titleCase(it.Kind), it.Qty)
			if it.Forbidden {
				s += ", forbidden"
			}
			out = append(out, groupRow{it.ID, s})
		}
	}
	return out
}

// groupNoun is "3 colonists" / "2 animals" / "4 items".
func groupNoun(k selKind, n int) string {
	noun := map[selKind]string{selPawn: "colonist", selAnimal: "animal", selItem: "item"}[k]
	if n != 1 {
		noun += "s"
	}
	return fmt.Sprintf("%d %s", n, noun)
}

// groupButtons are the group's command buttons in the grid (the pane's
// close and member rows are paneButtons).
func (a *App) groupButtons() []button {
	if !a.grouped() || a.workOpen || a.archOpen || a.renderer == nil {
		return nil
	}
	if len(a.groupRows()) < 2 {
		return nil // pruneGroup falls back to one
	}
	return a.cellButtons()
}

// groupPane is the pane for a group selection: its size, and the members
// as clickable rows.
func (a *App) groupPane() paneSpec {
	rows := a.groupRows()
	if len(rows) < 2 {
		return paneSpec{} // pruneGroup falls back to one
	}
	spec := paneSpec{
		Title:    groupNoun(a.sel.Group.Kind, len(rows)),
		Subtitle: "Shift+click adds or removes",
		Close:    "group.close",
	}
	for _, r := range rows {
		spec.Rows = append(spec.Rows, paneRow{ID: "group.pick." + strconv.FormatUint(uint64(r.id), 10), Label: r.label, Tip: "Select only this one"})
	}
	return spec
}

// groupOwnKeys are the commands a group provides itself (they act on every
// member at once through applyGroupClick); members' commands under these
// keys are not merged again.
var groupOwnKeys = map[string]bool{
	"draft": true, "pawn.draft": true, "undraft": true,
	"forbid": true, "item.forbid": true, "allow": true,
	"haulmark": true, "item.haulmark": true,
}

// groupOwnCommands are the group-wide commands that already exist
// (applyGroupClick), each pair as the old panel had it: Draft and Undraft
// for colonists; Forbid, Allow and Mark for hauling for items. Draft and
// Forbid show how many members already are (On: all, Mixed: some).
func (a *App) groupOwnCommands() []cmdButton {
	switch a.sel.Group.Kind {
	case selPawn:
		pawns := a.selectedPawns()
		on := 0
		for _, p := range pawns {
			if p.Drafted {
				on++
			}
		}
		draft := cmdButton{ID: "group.draft", Key: "draft", Label: "Draft", Icon: "draft", Hotkey: input.Draft,
			Desc: "Draft every selected colonist."}
		draft.On = len(pawns) > 0 && on == len(pawns)
		draft.Mixed = on > 0 && on < len(pawns)
		return []cmdButton{draft, {ID: "group.undraft", Key: "undraft", Label: "Undraft", Icon: "draft",
			Desc: "Stand every selected colonist down."}}
	case selItem:
		items := a.selectedItems()
		on := 0
		for _, it := range items {
			if it.Forbidden {
				on++
			}
		}
		forbid := cmdButton{ID: "group.forbid", Key: "forbid", Label: "Forbid", Icon: "order_forbid", Hotkey: input.Forbid,
			Desc: "Colonists will not touch forbidden stacks."}
		forbid.On = len(items) > 0 && on == len(items)
		forbid.Mixed = on > 0 && on < len(items)
		out := []cmdButton{forbid, {ID: "group.allow", Key: "allow", Label: "Allow", Icon: "order_allow",
			Desc: "Let colonists use these stacks again."}}
		if haulMarkable(items) {
			out = append(out, cmdButton{ID: "group.haulmark", Key: "haulmark", Label: "Mark for hauling", Icon: "order_haul",
				Desc: "Colonists will haul the stacks found on the map."})
		}
		return out
	}
	return nil
}

// groupCommands are a group's commands: its own group-wide ones, then its
// members' commands merged by key (mergeCommands).
func (a *App) groupCommands() []cmdButton {
	own := a.groupOwnCommands()
	return append(own, a.groupMerged()...)
}

// groupMerged are the members' commands merged by key, without the keys
// the group handles itself. It is remembered for one Update (and one
// selection): the layout asks many times a frame.
func (a *App) groupMerged() []cmdButton {
	m := &a.hst.cmds.memo
	if m.ok && m.updates == a.updates && m.kind == a.sel.Group.Kind && slicesEqualIDs(m.ids, a.sel.Group.Ids) {
		return m.out
	}
	kind := a.sel.Group.Kind
	ids := append([]entity.EntityID(nil), a.sel.Group.Ids...)
	sets := make([][]cmdButton, len(ids))
	for i, id := range ids {
		for _, c := range a.commandsFor(kind, id) {
			if !groupOwnKeys[c.mergeKey()] {
				sets[i] = append(sets[i], c)
			}
		}
	}
	out := mergeCommands(sets, func(a *App, member int, c cmdButton) { a.runMember(kind, ids[member], c) })
	*m = groupMemo{updates: a.updates, kind: kind, ids: ids, out: out, ok: true}
	return out
}

func slicesEqualIDs(a, b []entity.EntityID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// runMember runs member id's command c for one member of the group. A
// legacy command (an id, no Run or Cmds) acts on the single selection, so
// the member is selected for the call and the group put back after.
func (a *App) runMember(kind selKind, id entity.EntityID, c cmdButton) {
	if c.Run != nil || len(c.Cmds) > 0 || c.Options != nil {
		a.runCommand(c)
		return
	}
	saved := a.sel.Group
	a.selectCand(selCand{Kind: kind, ID: id})
	a.applyClick(c.ID)
	a.clearSelections(selNone)
	a.sel.Group = saved
}

// haulMarkable reports whether any stack in items was found on the map and
// is not marked for hauling yet.
func haulMarkable(items []game.ItemView) bool {
	for _, it := range items {
		if it.Unclaimed && !it.HaulMarked {
			return true
		}
	}
	return false
}

// pruneGroup drops members that are gone; a group of one becomes the
// single selection.
func (a *App) pruneGroup() {
	if a.grouped() {
		if rows := a.groupRows(); len(rows) < 2 {
			a.setGroup(a.sel.Group.Kind, groupIDs(rows))
		}
	}
}

func groupIDs(rows []groupRow) []entity.EntityID {
	out := make([]entity.EntityID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.id)
	}
	return out
}

// drawGroupMarks outlines every group member on the map.
func (a *App) drawGroupMarks(screen *ebiten.Image) {
	if !a.grouped() || a.renderer == nil {
		return
	}
	r := a.renderer
	tile := float32(r.tilePx())
	mark := func(wx, wy float64) {
		sx, sy := r.worldToScreenF(wx, wy)
		vector.StrokeRect(screen, sx+1, sy+1, tile-2, tile-2, 2, r.selectClr, false)
	}
	switch a.sel.Group.Kind {
	case selPawn:
		for _, p := range a.selectedPawns() {
			mark(r.interpWorld(p))
		}
	case selAnimal:
		for _, an := range a.selectedAnimals() {
			mark(r.interpAnimal(an))
		}
	case selItem:
		for _, it := range a.selectedItems() {
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
	fill := uiAccent
	fill.A = 40
	uiFillRect(screen, x0, y0, x1-x0, y1-y0, fill, false)
	uiStrokeRect(screen, x0, y0, x1-x0, y1-y0, 1, uiAccent, false)
}
