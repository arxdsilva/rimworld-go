package selection

import (
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Shell is what the selection model reads and asks of the shell: the
// colony's read model, the single selected pawn, the camera's tile lookup,
// the pointer and the command queue. The root satisfies every role.
type Shell interface {
	ports.World
	ports.PawnSel
	ports.View
	ports.Sizer
	ports.Commands
}

// Host is what only the root knows while the creature layer, the zone
// tools and the order handling are still root code (ARCH-005): a body hit
// test, the drawn colonist under the pointer, the zone tools' side of a
// selection, the power overlay's state and the right button's orders.
type Host interface {
	// AnimalsCovering lists the animals whose drawn body covers the tile
	// from another cell, nearest first.
	AnimalsCovering(w game.WorldView, tx, ty int) []Cand
	// PawnUnderCursor is the colonist drawn under map pixel (mx, my).
	PawnUnderCursor(mx, my int) (entity.EntityID, bool)
	// SelectZoneLabel selects the zone whose name plate is under (mx, my).
	SelectZoneLabel(w game.WorldView, mx, my, tx, ty int) bool
	// NoteCell records the last clicked cell for the zone tools.
	NoteCell(tile [2]int)
	// SetZone selects zone id (on) or drops the zone selection and ends
	// its Expand and Shrink tool (!on).
	SetZone(id uint16, on bool)
	// PowerShown reports the power overlay shows hidden conduits.
	PowerShown() bool
	// HandleOrders is the map's right button (orders, formation).
	HandleOrders()
	// Frame is the Update counter and the map-pixel scale (screen scale x
	// world scale) that the press thresholds follow.
	Frame() (updates uint64, scale float64)
}
