package ebiten

import (
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Hotkeys (UI-016): what each resolved action does in the colony. The
// keys themselves live in package input; this file only maps actions to
// the same calls the buttons make.

// midDragState is the middle-mouse camera drag.
type midDragState struct {
	on   bool
	x, y int
}

// handleViewKeys runs on every screen: screenshot and hide UI.
func (a *App) handleViewKeys() {
	if a.in.fired(input.Screenshot) {
		a.shot.pending = true // taken after the next Draw (screenshot.go)
	}
	if a.in.fired(input.HideUI) && a.nav.screen() == screenColony {
		a.hideUI = !a.hideUI
	}
}

// reservedFired toasts the first reserved action that fired and reports
// whether any did (they do nothing else).
func (a *App) reservedFired() bool {
	for _, act := range a.in.res.FiredList() {
		if d, ok := input.Lookup(act); ok && d.Reserved != "" {
			a.toast(d.Label + ": " + d.Reserved)
			return true
		}
	}
	return false
}

// handleColonyKeys applies the fired actions on the live colony.
func (a *App) handleColonyKeys() {
	f := &a.in
	r := a.renderer
	if a.reservedFired() {
		return
	}
	switch {
	case f.fired(input.Pause):
		r.togglePause()
	case f.fired(input.Speed1):
		r.setSpeed(speedNormal)
	case f.fired(input.Speed2):
		r.setSpeed(speedFast)
	case f.fired(input.Speed3):
		r.setSpeed(speedVeryFast)
	case f.fired(input.SpeedUp):
		r.stepSpeed(1)
	case f.fired(input.SpeedDown):
		r.stepSpeed(-1)
	}
	a.handleTabKeys()
	a.handleMapKeys()
	a.handleDevKeys() // devmode.go
}

// handleTabKeys opens the main tabs (the same as clicking them).
func (a *App) handleTabKeys() {
	f := &a.in
	switch {
	case f.fired(input.TabArchitect):
		a.applyClick("tab.architect")
	case f.fired(input.TabWork):
		a.applyClick("tab.work")
	case f.fired(input.TabSchedule):
		a.applyClick("tab.schedule")
	case f.fired(input.TabResearch):
		a.applyClick("tab.research")
	case f.fired(input.TabAnimals):
		a.openAnimals(false)
	case f.fired(input.TabWildlife):
		a.openAnimals(true)
	}
}

// openAnimals opens the Animals tab on its Colony or Wild list, or closes
// it when that list is already showing.
func (a *App) openAnimals(wild bool) {
	if a.workOpen && a.panelMode == "animals" && a.animalsWild == wild {
		a.workOpen = false
		return
	}
	if !a.workOpen || a.panelMode != "animals" {
		a.applyClick("tab.animals")
	}
	a.animalsWild = wild
}

// handleMapKeys are the colony-map actions.
func (a *App) handleMapKeys() {
	f := &a.in
	switch {
	case f.fired(input.Draft):
		a.draftSelection()
	case f.fired(input.Forbid):
		a.forbidSelection()
	case f.fired(input.RoofOverlay):
		a.renderer.toggleRoofOverlay()
	case f.fired(input.RoomStats):
		a.rooms = !a.rooms
	case f.fired(input.Search):
		a.openSearch() // search_ui.go
	case f.fired(input.PrevColonist):
		a.cycleColonist(-1)
	case f.fired(input.NextColonist):
		a.cycleColonist(1)
	case f.fired(input.CycleCell):
		a.cycleCell()
	case f.fired(input.ToolCancel):
		a.presetTool("cancel")
	case f.fired(input.ToolDeconstruct):
		a.presetTool("deconstruct")
	}
	for i, g := range input.Gizmos {
		if f.fired(g) {
			a.pressGizmo(i) // gizmo.go
		}
	}
}

// presetTool opens the Architect with an Orders tool in hand.
func (a *App) presetTool(id string) {
	t, ok := toolByID(id)
	if !ok {
		return
	}
	a.openArchitect()
	a.archCat = t.category
	if a.archTool != id {
		a.selectArchTool(id)
	}
}

// draftSelection is R: the selected colonists (one or a group) are
// drafted, or undrafted when all already are.
func (a *App) draftSelection() {
	pawns := a.selectedPawns()
	if len(pawns) == 0 {
		return
	}
	all := true
	for _, p := range pawns {
		all = all && p.Drafted
	}
	for _, p := range pawns {
		if p.Drafted == !all {
			continue
		}
		a.renderer.enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: !all})
	}
}

// forbidSelection is F: the selected item stacks are forbidden, or allowed
// when all already are.
func (a *App) forbidSelection() {
	items := a.selectedItems()
	if len(items) == 0 {
		return
	}
	all := true
	for _, it := range items {
		all = all && it.Forbidden
	}
	for _, it := range items {
		if it.Forbidden != all {
			continue
		}
		a.renderer.enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(it.ID), Enabled: !all})
	}
}

// cycleColonist selects the next (d > 0) or previous colonist in bar
// order and centres the camera on them.
func (a *App) cycleColonist(d int) {
	pawns := a.renderer.snapshot()
	if len(pawns) == 0 {
		return
	}
	i := -1
	if p, ok := a.selectedPawn(); ok {
		for j, q := range pawns {
			if q.ID == p.ID {
				i = j
			}
		}
	}
	switch {
	case i < 0 && d > 0:
		i = 0
	case i < 0:
		i = len(pawns) - 1
	default:
		i = (i + d + len(pawns)) % len(pawns)
	}
	p := pawns[i]
	a.selectCand(selCand{selPawn, p.ID})
	a.centreOnPawn(p.ID)
}

// centreOnPawn moves the camera onto a colonist (where it is drawn).
func (a *App) centreOnPawn(id entity.EntityID) {
	for _, p := range a.renderer.snapshot() {
		if p.ID == id {
			wx, wy := a.renderer.interpWorld(p)
			a.renderer.centerOn(wx+0.5, wy+0.5, a.sw, a.sh)
			return
		}
	}
}

// selectedTile is where the single selection stands.
func (a *App) selectedTile() (int, int, bool) {
	if p, ok := a.selectedPawn(); ok {
		return p.X, p.Y, true
	}
	if an, ok := a.selectedAnimal(); ok {
		return an.X, an.Y, true
	}
	if it, ok := a.selectedItem(); ok {
		return it.X, it.Y, true
	}
	if b, ok := a.selectedBuilding(); ok {
		return b.X, b.Y, true
	}
	if pl, ok := a.selectedPlant(); ok {
		return pl.X, pl.Y, true
	}
	if fr, ok := a.selectedFrame(); ok {
		return fr.X, fr.Y, true
	}
	return 0, 0, false
}

// selectedCand is the single selection as a candidate.
func (a *App) selectedCand() (selCand, bool) {
	switch {
	case a.renderer.HasSelect():
		return selCand{selPawn, a.renderer.Selected()}, true
	case a.hasAnimal:
		return selCand{selAnimal, a.selAnimal}, true
	case a.hasItem:
		return selCand{selItem, a.selItem}, true
	case a.hasBuilding:
		return selCand{selBuilding, a.selBuilding}, true
	case a.hasPlant:
		return selCand{selPlant, a.selPlant}, true
	case a.hasFrame:
		return selCand{selFrame, a.selFrame}, true
	}
	return selCand{}, false
}

// cycleCell is Numpad *: the next thing in the selected thing's cell, as
// clicking the cell again does.
func (a *App) cycleCell() {
	tx, ty, ok := a.selectedTile()
	if !ok {
		return
	}
	cands := selectablesAt(a.renderer.world(), a.renderer.snapshot(), tx, ty)
	if len(cands) < 2 {
		return
	}
	cur, _ := a.selectedCand()
	i := 0
	for j, c := range cands {
		if c == cur {
			i = j
		}
	}
	i = (i + 1) % len(cands)
	a.selectCand(cands[i])
	a.selCycle = selCycle{tile: [2]int{tx, ty}, idx: i, n: len(cands), ok: true}
}

// cameraInput gathers this frame's camera intent.
func (a *App) cameraInput(v viewport) camInput {
	f := &a.in
	var c camInput
	if f.held(input.CamLeft) {
		c.dx--
	}
	if f.held(input.CamRight) {
		c.dx++
	}
	if f.held(input.CamUp) {
		c.dy--
	}
	if f.held(input.CamDown) {
		c.dy++
	}
	c.fast = f.shift() && f.top() != input.CtxText
	mx, my := a.currentCursor()
	if a.renderer.edgeScroll && f.raw.focused {
		ex, ey := edgePan(mx, my, v)
		c.dx, c.dy = c.dx+ex, c.dy+ey
	}
	if f.fired(input.ZoomIn) {
		c.zoom++
	}
	if f.fired(input.ZoomOut) {
		c.zoom--
	}
	if f.raw.wheelY != 0 && !a.renderer.wheelTaken && !a.uiWheelOver(mx, my) { // a scroll view under the cursor owns the wheel
		c.wheel, c.wx, c.wy = f.raw.wheelY, float64(mx), float64(my)
	}
	switch {
	case f.clicked(mbMiddle):
		f.take(mbMiddle)
		a.midDrag = midDragState{on: true, x: mx, y: my}
	case a.midDrag.on && f.down(mbMiddle):
		c.dragX, c.dragY = float64(mx-a.midDrag.x), float64(my-a.midDrag.y)
		a.midDrag.x, a.midDrag.y = mx, my
	default:
		a.midDrag.on = false
	}
	return c
}
