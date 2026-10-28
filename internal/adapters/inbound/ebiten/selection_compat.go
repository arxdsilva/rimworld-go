package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// What is selected lives in selection (ARCH-005). These aliases and
// one-line wrappers keep the root's call sites unchanged while its
// features move out one package at a time (SOLID_GO.md §11).

type (
	selKind    = selection.Kind
	selCand    = selection.Cand
	selGroup   = selection.Group
	selCycle   = selection.Cycle
	pressState = selection.Press
)

const (
	selNone     = selection.None
	selPawn     = selection.Pawn
	selAnimal   = selection.Animal
	selItem     = selection.Item
	selBuilding = selection.Building
	selPlant    = selection.Plant
	selFrame    = selection.Frame
	selZone     = selection.Zone

	doubleClickFrames = selection.DoubleClickFrames
)

func groupable(k selKind) bool { return selection.Groupable(k) }

func selectablesAt(w game.WorldView, pawns []game.PawnView, tx, ty int) []selCand {
	return selection.SelectablesAt(w, pawns, tx, ty)
}

func (a *App) selectables(w game.WorldView, tx, ty int) []selCand {
	return a.sel.Selectables(w, tx, ty)
}
func (a *App) clickSelect()                               { a.sel.ClickSelect() }
func (a *App) selectCand(c selCand)                       { a.sel.Select(c) }
func (a *App) clearSelections(keep selKind)               { a.sel.Clear(keep) }
func (a *App) selectPlantAt(tx, ty int) bool              { return a.sel.SelectPlantAt(tx, ty) }
func (a *App) selectItemAt(tx, ty int) bool               { return a.sel.SelectItemAt(tx, ty) }
func (a *App) clearPlant()                                { a.sel.ClearPlant() }
func (a *App) clearItem()                                 { a.sel.ClearItem() }
func (a *App) selectedPlant() (game.PlantView, bool)      { return a.sel.SelectedPlant() }
func (a *App) selectedItem() (game.ItemView, bool)        { return a.sel.SelectedItem() }
func (a *App) grouped() bool                              { return a.sel.Grouped() }
func (a *App) inGroup(k selKind, id entity.EntityID) bool { return a.sel.InGroup(k, id) }
func (a *App) setGroup(k selKind, ids []entity.EntityID)  { a.sel.SetGroup(k, ids) }
func (a *App) selectionSet() (selKind, []entity.EntityID) { return a.sel.SelectionSet() }
func (a *App) candUnderCursor() (selCand, bool)           { return a.sel.CandUnderCursor() }
func (a *App) shiftClick()                                { a.sel.ShiftClick() }
func (a *App) toggleInSet(c selCand)                      { a.sel.ToggleInSet(c) }
func (a *App) selectedPawns() []game.PawnView             { return a.sel.SelectedPawns() }
func (a *App) selectedItems() []game.ItemView             { return a.sel.SelectedItems() }
func (a *App) selectedAnimals() []game.AnimalView         { return a.sel.SelectedAnimals() }
func (a *App) applyGroupClick(id string) {
	if a.renderer != nil {
		a.sel.ApplyGroupClick(id)
	}
}

// selHost is what the selection model needs of the creature layer, the
// zone tools and the order handling, all still root code.
type selHost struct{ a *App }

func (h selHost) AnimalsCovering(w game.WorldView, tx, ty int) []selCand {
	if h.a.renderer == nil {
		return nil
	}
	return h.a.renderer.animalsCovering(w, tx, ty)
}

func (h selHost) PawnUnderCursor(mx, my int) (entity.EntityID, bool) {
	if h.a.renderer == nil {
		return 0, false
	}
	return h.a.renderer.pawnUnderCursor(mx, my)
}

func (h selHost) SelectZoneLabel(w game.WorldView, mx, my, tx, ty int) bool {
	return h.a.selectZoneLabel(w, mx, my, tx, ty)
}

func (h selHost) NoteCell(tile [2]int) { h.a.arch.Zones.NoteCell(tile) }

func (h selHost) SetZone(id uint16, on bool) {
	if on {
		h.a.selectZone(id)
		return
	}
	h.a.clearZone()
}

func (h selHost) PowerShown() bool { return h.a.shell.PowerShown() }
func (h selHost) HandleOrders()    { h.a.handleOrders() }

func (h selHost) Frame() (uint64, float64) {
	return h.a.updates, max(1, h.a.scale.S) * h.a.scale.WorldK()
}

func selectablesSplit(w game.WorldView, pawns []game.PawnView, tx, ty int) ([]selCand, int) {
	return selection.SelectablesSplit(w, pawns, tx, ty)
}
