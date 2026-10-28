package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/orders"
	"rimworld-go/internal/application/game"
)

// drawOrderRoute draws the selected colonist's current order target and
// queued orders as a dotted route with numbered waypoints (the drawing is
// orders.DrawRoute; the renderer owns the selection and the interpolated
// position).
func (r *Renderer) drawOrderRoute(screen *ebiten.Image, tile float64) {
	if !r.hasSelect {
		return
	}
	var p game.PawnView
	found := false
	for _, q := range r.view.Pawns {
		if q.ID == r.selected {
			p, found = q, true
		}
	}
	if !found {
		return
	}
	wx, wy := r.interpWorld(p)
	orders.DrawRoute(screen, tile, p, wx, wy, r.worldToScreenF)
}
