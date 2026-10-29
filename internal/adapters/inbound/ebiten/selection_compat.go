package ebiten

import (
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// What is selected lives in selection (ARCH-005). These aliases and
// one-line wrappers keep the root's call sites unchanged while its
// features move out one package at a time (SOLID_GO.md §11).

func (a *App) applyGroupClick(id string) {
	if a.renderer != nil {
		a.sel.ApplyGroupClick(id)
	}
}

// selHost is what the selection model needs of the creature layer, the
// zone tools and the order handling, all still root code.
type selHost struct{ a *App }

func (h selHost) AnimalsCovering(w game.WorldView, tx, ty int) []selectionpkg.Cand {
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
	return h.a.arch.Zones.SelectLabel(w, mx, my, tx, ty)
}

func (h selHost) NoteCell(tile [2]int) { h.a.arch.Zones.NoteCell(tile) }

func (h selHost) SetZone(id uint16, on bool) {
	if on {
		h.a.arch.Zones.Select(id)
		return
	}
	h.a.arch.Zones.Clear()
}

func (h selHost) PowerShown() bool { return h.a.shell.PowerShown() }
func (h selHost) HandleOrders()    { h.a.ord.HandleOrders() }

func (h selHost) Frame() (uint64, float64) {
	return h.a.updates, max(1, h.a.scale.S) * h.a.scale.WorldK()
}
