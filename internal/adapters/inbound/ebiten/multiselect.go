package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Multi-selection (UI-016, MAIN_UI §9 Selection): Shift+click adds or
// removes, a left drag boxes colonists (else animals, else items), a
// double-click takes every visible thing of the same kind. A group of two
// or more replaces the single selections and gets its own panel; R, F and
// right-click orders act on every member. Presentation state only.

// selGroup is a multi-selection of one kind (colonists, animals or items).
type selGroup struct {
	kind selKind
	ids  []entity.EntityID
}

// pressState is the map's left press: where it went down (for the drag
// box) and the last click (for double clicks).
type pressState struct {
	on, box, shift bool
	x, y           int
	lastAt         uint64 // App.updates of the last map click
	lastX, lastY   int
}

const (
	boxMinPx          = 6  // drag this far before a press becomes a box
	doubleClickFrames = 18 // two clicks this close (Update frames) are a double click
	doubleClickPx     = 6
	groupListMax      = 14
)

// grouped reports a multi-selection is active.
func (a *App) grouped() bool { return len(a.group.ids) >= 2 }

// inGroup reports id is a member of a group of kind k.
func (a *App) inGroup(k selKind, id entity.EntityID) bool {
	if !a.grouped() || a.group.kind != k {
		return false
	}
	for _, g := range a.group.ids {
		if g == id {
			return true
		}
	}
	return false
}

// groupable kinds can form a group.
func groupable(k selKind) bool { return k == selPawn || k == selAnimal || k == selItem }

// setGroup selects ids of kind: nothing, one thing (the single panel) or
// a group.
func (a *App) setGroup(kind selKind, ids []entity.EntityID) {
	var clean []entity.EntityID
	seen := map[entity.EntityID]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			clean = append(clean, id)
		}
	}
	switch len(clean) {
	case 0:
		a.clearSelections(selNone)
	case 1:
		a.selectCand(selCand{kind, clean[0]})
	default:
		a.clearSelections(selNone)
		a.group = selGroup{kind: kind, ids: clean}
	}
}

// selectionSet is the current selection as a kind and ids (a group or the
// single selection of a groupable kind).
func (a *App) selectionSet() (selKind, []entity.EntityID) {
	if a.grouped() {
		return a.group.kind, append([]entity.EntityID(nil), a.group.ids...)
	}
	if c, ok := a.selectedCand(); ok && groupable(c.kind) {
		return c.kind, []entity.EntityID{c.id}
	}
	return selNone, nil
}

// handleMapMouse is the map's left button (select, Shift-toggle, box,
// double click) and then the right button (orders, formation.go).
func (a *App) handleMapMouse() {
	f := &a.in
	mx, my := a.currentCursor()
	switch {
	case f.clicked(mbLeft):
		f.take(mbLeft)
		p := a.press
		double := p.lastAt > 0 && a.updates-p.lastAt <= doubleClickFrames &&
			abs(mx-p.lastX) <= doubleClickPx && abs(my-p.lastY) <= doubleClickPx
		a.press = pressState{on: true, x: mx, y: my, shift: f.shift(), lastAt: a.updates, lastX: mx, lastY: my}
		switch {
		case double:
			a.press.lastAt = 0 // a third click starts over
			a.selectSameOnScreen()
		case f.shift():
			a.shiftClick()
		default:
			a.clickSelect()
		}
	case a.press.on && f.down(mbLeft):
		if !a.press.box && (abs(mx-a.press.x) > boxMinPx || abs(my-a.press.y) > boxMinPx) {
			a.press.box = true
		}
	case a.press.on:
		if a.press.box {
			a.boxSelect(a.press.x, a.press.y, mx, my, a.press.shift)
		}
		a.press.on, a.press.box = false, false
	}
	a.handleOrders()
}

// candUnderCursor is what a plain click would select first.
func (a *App) candUnderCursor() (selCand, bool) {
	mx, my := a.currentCursor()
	if id, ok := a.renderer.pawnUnderCursor(mx, my); ok {
		return selCand{selPawn, id}, true
	}
	tx, ty := a.renderer.TileAt(mx, my)
	if cands := selectablesAt(a.renderer.world(), a.renderer.snapshot(), tx, ty); len(cands) > 0 {
		return cands[0], true
	}
	return selCand{}, false
}

// shiftClick adds the thing under the cursor to the selection, or takes
// it out when it is already in. Other kinds start a new selection.
func (a *App) shiftClick() {
	c, ok := a.candUnderCursor()
	if !ok || !groupable(c.kind) {
		a.clickSelect()
		return
	}
	a.toggleInSet(c)
}

// toggleInSet adds c to the selection or takes it out (a different kind
// starts over with c).
func (a *App) toggleInSet(c selCand) {
	kind, ids := a.selectionSet()
	if kind != c.kind {
		a.setGroup(c.kind, []entity.EntityID{c.id})
		return
	}
	out := ids[:0]
	found := false
	for _, id := range ids {
		if id == c.id {
			found = true
			continue
		}
		out = append(out, id)
	}
	if !found {
		out = append(out, c.id)
	}
	a.setGroup(kind, out)
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

// thingsIn lists what lies inside a screen rectangle: colonists, else
// animals, else ground items (each in read-model order).
func (a *App) thingsIn(x0, y0, x1, y1 float32) (selKind, []entity.EntityID) {
	r := a.renderer
	var ids []entity.EntityID
	for _, p := range r.snapshot() {
		if wx, wy := r.interpWorld(p); !p.Embodied && r.inRect(wx, wy, x0, y0, x1, y1) {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) > 0 {
		return selPawn, ids
	}
	w := r.world()
	for _, an := range w.Animals {
		if wx, wy := r.interpAnimal(an); r.inRect(wx, wy, x0, y0, x1, y1) {
			ids = append(ids, an.ID)
		}
	}
	if len(ids) > 0 {
		return selAnimal, ids
	}
	for _, it := range w.Items {
		if !it.Carried && r.inRect(float64(it.X), float64(it.Y), x0, y0, x1, y1) {
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
	if kind == selNone {
		if !add {
			a.clearSelections(selNone)
		}
		return
	}
	if add {
		if k, cur := a.selectionSet(); k == kind {
			ids = append(cur, ids...)
		}
	}
	a.setGroup(kind, ids)
}

// selectSameOnScreen is the double click: every visible colonist, animal
// of the same species, or stack of the same item.
func (a *App) selectSameOnScreen() {
	c, ok := a.selectedCand()
	if !ok || !groupable(c.kind) {
		a.clickSelect()
		return
	}
	v := colonyViewport(a.size())
	x0, y0, x1, y1 := float32(v.x0), float32(v.y0), float32(v.x1), float32(v.y1)
	r := a.renderer
	w := r.world()
	var ids []entity.EntityID
	switch c.kind {
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
	a.setGroup(c.kind, append([]entity.EntityID{c.id}, ids...))
}

// selectedPawns is the selected colonists: the group's living members in
// selection order, or the single selected colonist.
func (a *App) selectedPawns() []game.PawnView {
	if a.renderer == nil {
		return nil
	}
	if !a.grouped() {
		if p, ok := a.selectedPawn(); ok {
			return []game.PawnView{p}
		}
		return nil
	}
	if a.group.kind != selPawn {
		return nil
	}
	byID := map[entity.EntityID]game.PawnView{}
	for _, p := range a.renderer.snapshot() {
		byID[p.ID] = p
	}
	var out []game.PawnView
	for _, id := range a.group.ids {
		if p, ok := byID[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

// selectedItems is the selected ground stacks (group or single).
func (a *App) selectedItems() []game.ItemView {
	if a.renderer == nil {
		return nil
	}
	if !a.grouped() {
		if it, ok := a.selectedItem(); ok {
			return []game.ItemView{it}
		}
		return nil
	}
	if a.group.kind != selItem {
		return nil
	}
	byID := map[entity.EntityID]game.ItemView{}
	for _, it := range a.renderer.world().Items {
		if !it.Carried {
			byID[it.ID] = it
		}
	}
	var out []game.ItemView
	for _, id := range a.group.ids {
		if it, ok := byID[id]; ok {
			out = append(out, it)
		}
	}
	return out
}

// selectedAnimals is the group's animals.
func (a *App) selectedAnimals() []game.AnimalView {
	if a.renderer == nil || !a.grouped() || a.group.kind != selAnimal {
		return nil
	}
	byID := map[entity.EntityID]game.AnimalView{}
	for _, an := range a.renderer.world().Animals {
		byID[an.ID] = an
	}
	var out []game.AnimalView
	for _, id := range a.group.ids {
		if an, ok := byID[id]; ok {
			out = append(out, an)
		}
	}
	return out
}

// groupRow is one member line of the group panel.
type groupRow struct {
	id    entity.EntityID
	label string
}

// groupRows lists the group's members (dropping ones that are gone).
func (a *App) groupRows() []groupRow {
	var out []groupRow
	switch a.group.kind {
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

// groupNoun is "3 colonists" / "2 animals" / "4 stacks".
func groupNoun(k selKind, n int) string {
	noun := map[selKind]string{selPawn: "colonist", selAnimal: "animal", selItem: "stack"}[k]
	if n != 1 {
		noun += "s"
	}
	return fmt.Sprintf("%d %s", n, noun)
}

func groupRect(rows int) button {
	r := inspectorRect()
	return button{x: r.x, y: r.y, w: r.w, h: float32(76 + min(rows, groupListMax)*18)}
}

// groupButtons are the group panel's buttons ("group.*").
func (a *App) groupButtons() []button {
	if !a.grouped() || a.workOpen || a.archOpen || a.renderer == nil {
		return nil
	}
	rows := a.groupRows()
	if len(rows) < 2 {
		return nil // pruneGroup falls back to one
	}
	r := groupRect(len(rows))
	out := []button{{id: "group.close", label: "X", x: r.x + r.w - 28, y: r.y + 6, w: 22, h: 22}}
	y := r.y + 34
	switch a.group.kind {
	case selPawn:
		out = append(out,
			button{id: "group.draft", label: "Draft all" + a.keyHint(input.Draft), x: r.x + 12, y: y, w: 140, h: 24},
			button{id: "group.undraft", label: "Undraft all", x: r.x + 160, y: y, w: 120, h: 24})
	case selItem:
		out = append(out,
			button{id: "group.forbid", label: "Forbid all" + a.keyHint(input.Forbid), x: r.x + 12, y: y, w: 140, h: 24},
			button{id: "group.allow", label: "Allow all", x: r.x + 160, y: y, w: 120, h: 24})
	}
	for i, row := range rows {
		if i == groupListMax {
			break
		}
		out = append(out, button{id: "group.pick." + strconv.FormatUint(uint64(row.id), 10), label: row.label,
			x: r.x + 12, y: r.y + 66 + float32(i)*18, w: r.w - 24, h: 16, tip: "Select only this one"})
	}
	return out
}

// pruneGroup drops members that are gone; a group of one becomes the
// single selection.
func (a *App) pruneGroup() {
	if a.grouped() {
		if rows := a.groupRows(); len(rows) < 2 {
			a.setGroup(a.group.kind, groupIDs(rows))
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

// applyGroupClick handles a "group.*" button.
func (a *App) applyGroupClick(id string) {
	if a.renderer == nil {
		return
	}
	switch id {
	case "group.close":
		a.clearSelections(selNone)
	case "group.draft", "group.undraft":
		on := id == "group.draft"
		for _, p := range a.selectedPawns() {
			if p.Drafted != on {
				a.renderer.enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: on})
			}
		}
	case "group.forbid", "group.allow":
		on := id == "group.forbid"
		for _, it := range a.selectedItems() {
			if it.Forbidden != on {
				a.renderer.enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(it.ID), Enabled: on})
			}
		}
	default:
		if n, err := strconv.ParseUint(strings.TrimPrefix(id, "group.pick."), 10, 64); err == nil {
			a.selectCand(selCand{a.group.kind, entity.EntityID(n)})
		}
	}
}

// drawGroupInspector draws the group panel and marks every member.
func (a *App) drawGroupInspector(screen *ebiten.Image) {
	bs := a.groupButtons()
	if len(bs) == 0 {
		return
	}
	rows := a.groupRows()
	r := groupRect(len(rows))
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+10, groupNoun(a.group.kind, len(rows))+" selected")
	if len(rows) > groupListMax {
		drawLabel(screen, int(r.x)+12, int(r.y+r.h)-18, fmt.Sprintf("... and %d more", len(rows)-groupListMax))
	}
	mx, my := a.currentCursor()
	for _, b := range bs {
		if strings.HasPrefix(b.id, "group.pick.") {
			if b.contains(mx, my) {
				vector.FillRect(screen, b.x, b.y, b.w, b.h, uiBtnHover, false)
			}
			drawLabel(screen, int(b.x)+4, int(b.y)+1, b.label)
			continue
		}
		drawButton(screen, b, b.contains(mx, my))
	}
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
	switch a.group.kind {
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
	if !a.press.on || !a.press.box {
		return
	}
	mx, my := a.currentCursor()
	x0, y0, x1, y1 := screenRect(a.press.x, a.press.y, mx, my)
	fill := uiAccent
	fill.A = 40
	vector.FillRect(screen, x0, y0, x1-x0, y1-y0, fill, false)
	vector.StrokeRect(screen, x0, y0, x1-x0, y1-y0, 1, uiAccent, false)
}
