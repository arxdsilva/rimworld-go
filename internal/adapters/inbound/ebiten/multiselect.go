package ebiten

import (
	"fmt"
	"math"
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
	mx, my := a.mapCursor() // the press, the box and the double click live in map pixels
	// the thresholds scale like the screen, then into map pixels
	sc := max(1, a.scale.s) * a.scale.worldK()
	boxMin, dblPx := int(boxMinPx*sc), int(doubleClickPx*sc)
	switch {
	case f.clicked(mbLeft):
		f.take(mbLeft)
		p := a.press
		double := p.lastAt > 0 && a.updates-p.lastAt <= doubleClickFrames &&
			abs(mx-p.lastX) <= dblPx && abs(my-p.lastY) <= dblPx
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
		if !a.press.box && (abs(mx-a.press.x) > boxMin || abs(my-a.press.y) > boxMin) {
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
	mx, my := a.mapCursor() // physical
	if id, ok := a.renderer.pawnUnderCursor(mx, my); ok {
		return selCand{selPawn, id}, true
	}
	tx, ty := a.shell.TileAt(mx, my)
	if cands := a.selectables(a.shell.World(), tx, ty); len(cands) > 0 {
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

// bodyInRect reports the screen rectangle holding the centre of a tile
// an's drawn body covers (PLAY-022 C7: a box over a big animal's flank
// takes it, as a click there does). An animal whose cell is farther from
// the box than the largest body reaches is not measured.
func (r *Renderer) bodyInRect(an game.AnimalView, x0, y0, x1, y1 float32) bool {
	tile := r.tilePx()
	pad, padS := r.animalPads(tile)
	// the tile centres inside the box
	tx0 := int(math.Ceil((float64(x0)+r.cam.offsetX)/tile - 0.5))
	ty0 := int(math.Ceil((float64(y0)+r.cam.offsetY)/tile - 0.5))
	tx1 := int(math.Floor((float64(x1)+r.cam.offsetX)/tile - 0.5))
	ty1 := int(math.Floor((float64(y1)+r.cam.offsetY)/tile - 0.5))
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
	v := a.mapViewport() // physical, like worldToScreenF
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
	for _, p := range a.shell.Pawns() {
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
	for _, it := range a.shell.World().Items {
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
	for _, an := range a.shell.World().Animals {
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
		Title:    groupNoun(a.group.kind, len(rows)),
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
	switch a.group.kind {
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
	if m.ok && m.updates == a.updates && m.kind == a.group.kind && slicesEqualIDs(m.ids, a.group.ids) {
		return m.out
	}
	kind := a.group.kind
	ids := append([]entity.EntityID(nil), a.group.ids...)
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
	saved := a.group
	a.selectCand(selCand{kind, id})
	a.applyClick(c.ID)
	a.clearSelections(selNone)
	a.group = saved
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
				a.shell.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: on})
			}
		}
	case "group.forbid", "group.allow":
		on := id == "group.forbid"
		for _, it := range a.selectedItems() {
			if it.Forbidden != on {
				a.shell.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(it.ID), Enabled: on})
			}
		}
	case "group.haulmark":
		for _, it := range a.selectedItems() {
			if it.Unclaimed && !it.HaulMarked {
				a.shell.Enqueue(command.Command{Kind: command.SetHaulMark, Target: uint64(it.ID), Enabled: true})
			}
		}
	default:
		if n, err := strconv.ParseUint(strings.TrimPrefix(id, "group.pick."), 10, 64); err == nil {
			a.selectCand(selCand{a.group.kind, entity.EntityID(n)})
		}
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
	// The press and cursor are physical (map pixels); the box is UI-canvas
	// drawing, so its corners go to logical.
	pmx, pmy := a.mapCursor()
	lx0, ly0 := a.mapToLogical(a.press.x, a.press.y)
	lx1, ly1 := a.mapToLogical(pmx, pmy)
	x0, y0, x1, y1 := screenRect(lx0, ly0, lx1, ly1)
	fill := uiAccent
	fill.A = 40
	uiFillRect(screen, x0, y0, x1-x0, y1-y0, fill, false)
	uiStrokeRect(screen, x0, y0, x1-x0, y1-y0, 1, uiAccent, false)
}
