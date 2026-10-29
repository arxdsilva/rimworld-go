package ebiten

import (
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
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
	if a.in.Fired(input.Screenshot) {
		a.meter.Shot.Pending = true // taken after the next Draw (screenshot.go)
	}
	if a.in.Fired(input.HideUI) && a.nav.screen() == ports.ScreenColony {
		a.chrome.Hidden = !a.chrome.Hidden
	}
}

// reservedFired toasts the first reserved action that fired and reports
// whether any did (they do nothing else).
func (a *App) reservedFired() bool {
	for _, act := range a.in.Res.FiredList() {
		if d, ok := input.Lookup(act); ok && d.Reserved != "" {
			a.chrome.Toast(d.Label + ": " + d.Reserved)
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
	case f.Fired(input.Minimap):
		r.toggleMinimap()
	case f.Fired(input.Pause):
		r.togglePause()
	case f.Fired(input.Speed1):
		r.setSpeed(speedNormal)
	case f.Fired(input.Speed2):
		r.setSpeed(speedFast)
	case f.Fired(input.Speed3):
		r.setSpeed(speedVeryFast)
	case f.Fired(input.SpeedUp):
		r.stepSpeed(1)
	case f.Fired(input.SpeedDown):
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
	case f.Fired(input.TabArchitect):
		a.applyClick("tab.architect")
	case f.Fired(input.TabWork):
		a.applyClick("tab.work")
	case f.Fired(input.TabSchedule):
		a.applyClick("tab.schedule")
	case f.Fired(input.TabResearch):
		a.applyClick("tab.research")
	case f.Fired(input.TabAssign):
		a.win.Assign().Toggle()
	case f.Fired(input.TabWorld):
		a.win.World().Toggle() // world_ui.go
	case f.Fired(input.TabHistory):
		if a.uiModalTop() == windows.HistoryModalID {
			a.uiCloseTop()
		} else {
			a.win.History().Launch()
		}
	case f.Fired(input.TabAnimals):
		a.openAnimals(false)
	case f.Fired(input.TabWildlife):
		a.openAnimals(true)
	}
}

// openAnimals opens the Animals tab on its Colony or Wild list, or closes
// it when that list is already showing.
func (a *App) openAnimals(wild bool) {
	if a.win.Open && a.win.Mode == "animals" && a.win.Wild == wild {
		a.win.Open = false
		return
	}
	if !a.win.Open || a.win.Mode != "animals" {
		a.applyClick("tab.animals")
	}
	a.win.Wild = wild
}

// handleMapKeys are the colony-map actions.
func (a *App) handleMapKeys() {
	f := &a.in
	switch {
	case f.Fired(input.Draft):
		a.draftSelection()
	case f.Fired(input.Forbid):
		a.forbidSelection()
	case f.Fired(input.Power):
		a.insp.Building().PowerSelection() // cmds_power.go (PLAY-021)
	case f.Fired(input.RoofOverlay):
		a.renderer.overlays().Roof.Toggle()
	case f.Fired(input.RoomStats):
		a.chrome.Rooms = !a.chrome.Rooms
	case f.Fired(input.PowerOverlay):
		a.renderer.overlays().Power.Toggle() // power_overlay.go (PLAY-021)
	case f.Fired(input.Search):
		a.ord.OpenSearch() // search_ui.go
	case f.Fired(input.PrevColonist):
		a.cycleColonist(-1)
	case f.Fired(input.NextColonist):
		a.cycleColonist(1)
	case f.Fired(input.CycleCell):
		a.cycleCell()
	case f.Fired(input.ToolCancel):
		a.presetTool("cancel")
	case f.Fired(input.ToolDeconstruct):
		a.presetTool("deconstruct")
	}
	for i, g := range input.Gizmos {
		if f.Fired(g) {
			a.insp.Commands().PressCommandKey(i) // hud_commands.go
		}
	}
}

// presetTool opens the Architect with an Orders tool in hand.
func (a *App) presetTool(id string) {
	t, ok := toolByID(id)
	if !ok {
		return
	}
	a.arch.OpenPanel()
	a.arch.Cat = t.Category
	if a.arch.ToolID != id {
		a.arch.SelectTool(id)
	}
}

// draftSelection is R: the selected colonists (one or a group) are
// drafted, or undrafted when all already are.
func (a *App) draftSelection() {
	pawns := a.sel.SelectedPawns()
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
	items := a.sel.SelectedItems()
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
	if p, ok := a.insp.SelectedPawn(); ok {
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
	a.sel.Select(selectionpkg.Cand{Kind: selectionpkg.Pawn, ID: p.ID})
	a.centreOnPawn(p.ID)
}

// centreOnPawn moves the camera onto a colonist (where it is drawn) and
// stops any pan or zoom ease in flight, so it stays there.
func (a *App) centreOnPawn(id entity.EntityID) {
	for _, p := range a.renderer.snapshot() {
		if p.ID == id {
			a.renderer.smooth.Halt()
			wx, wy := a.renderer.interpWorld(p)
			a.centerOnMap(wx+0.5, wy+0.5)
			return
		}
	}
}

// selectedTile is where the single selection stands.
func (a *App) selectedTile() (int, int, bool) {
	if p, ok := a.insp.SelectedPawn(); ok {
		return p.X, p.Y, true
	}
	if an, ok := a.insp.SelectedAnimal(); ok {
		return an.X, an.Y, true
	}
	if it, ok := a.sel.SelectedItem(); ok {
		return it.X, it.Y, true
	}
	if b, ok := a.insp.SelectedBuilding(); ok {
		return b.X, b.Y, true
	}
	if pl, ok := a.sel.SelectedPlant(); ok {
		return pl.X, pl.Y, true
	}
	if fr, ok := a.insp.SelectedFrame(); ok {
		return fr.X, fr.Y, true
	}
	return 0, 0, false
}

// selectedCand is the single selection as a candidate.
func (a *App) selectedCand() (selectionpkg.Cand, bool) {
	switch {
	case a.renderer.HasSelect():
		return selectionpkg.Cand{Kind: selectionpkg.Pawn, ID: a.renderer.Selected()}, true
	case a.sel.HasAnimal:
		return selectionpkg.Cand{Kind: selectionpkg.Animal, ID: a.sel.SelAnimal}, true
	case a.sel.HasItem:
		return selectionpkg.Cand{Kind: selectionpkg.Item, ID: a.sel.SelItem}, true
	case a.sel.HasBuilding:
		return selectionpkg.Cand{Kind: selectionpkg.Building, ID: a.sel.SelBuilding}, true
	case a.sel.HasPlant:
		return selectionpkg.Cand{Kind: selectionpkg.Plant, ID: a.sel.SelPlant}, true
	case a.sel.HasFrame:
		return selectionpkg.Cand{Kind: selectionpkg.Frame, ID: a.sel.SelFrame}, true
	}
	return selectionpkg.Cand{}, false
}

// cycleCell is Numpad *: the next thing in the selected thing's cell, as
// clicking the cell again does.
func (a *App) cycleCell() {
	tx, ty, ok := a.selectedTile()
	if !ok {
		return
	}
	cands := a.sel.Selectables(a.renderer.world(), tx, ty)
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
	a.sel.Select(cands[i])
	a.sel.Cycle = selectionpkg.Cycle{Tile: [2]int{tx, ty}, Idx: i, N: len(cands), Ok: true}
}

// cameraInput gathers this frame's camera intent.
func (a *App) cameraInput(v camerapkg.Viewport) camInput {
	f := &a.in
	var c camInput
	if f.Held(input.CamLeft) {
		c.dx--
	}
	if f.Held(input.CamRight) {
		c.dx++
	}
	if f.Held(input.CamUp) {
		c.dy--
	}
	if f.Held(input.CamDown) {
		c.dy++
	}
	c.fast = f.Shift() && f.Top() != input.CtxText
	// v and the camera are physical: edge scroll, the wheel's anchor and
	// the middle drag read the physical cursor; UI hit tests the logical one.
	mx, my := a.mapCursor()
	if a.renderer.edgeScroll && f.Raw.Focused {
		ex, ey := edgePan(mx, my, v)
		c.dx, c.dy = c.dx+ex, c.dy+ey
	}
	if f.Fired(input.ZoomIn) {
		c.zoom++
	}
	if f.Fired(input.ZoomOut) {
		c.zoom--
	}
	if lx, ly := a.currentCursor(); f.Raw.WheelY != 0 && !a.renderer.wheelTaken && !a.uiSt().WheelOver(lx, ly) { // a scroll view under the cursor owns the wheel
		c.wheel, c.wx, c.wy = f.Raw.WheelY, float64(mx), float64(my)
	}
	switch {
	case f.Clicked(inputframe.Middle):
		f.Take(inputframe.Middle)
		a.midDrag = midDragState{on: true, x: mx, y: my}
	case a.midDrag.on && f.Down(inputframe.Middle):
		c.dragX, c.dragY = float64(mx-a.midDrag.x), float64(my-a.midDrag.y)
		a.midDrag.x, a.midDrag.y = mx, my
	default:
		a.midDrag.on = false
	}
	return c
}
