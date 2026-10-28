package creature

import (
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017: an animal chasing a colonist is shown, not just announced: a
// red mark over the animal and a red line to its target (both world draws,
// in traces.go), and a camera that brings an off-screen chase into
// view when the warning pauses the game (here).

// ChasedPawn is the pawn with id, if there is one.
func ChasedPawn(pawns []game.PawnView, id entity.EntityID) (game.PawnView, bool) {
	for _, p := range pawns {
		if p.ID == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// OnThreatPause runs when a new danger message (posted at tick) has just
// auto-paused the game. When that message is an animal starting to chase
// a colonist and the chase is not wholly on screen inside the physical
// viewport vp, it returns where the camera centres: between the animal
// and the colonist. Any other danger leaves the camera alone (false).
func (c *Set) OnThreatPause(v *render.View, w game.WorldView, pawns []game.PawnView, tick uint64, vp camera.Viewport) (x, y float64, ok bool) {
	// the newest danger posted at tick: oldest first, so the last one
	var msg game.MessageView
	found := false
	for i := len(w.Messages) - 1; i >= 0 && !found; i-- {
		if m := w.Messages[i]; m.Danger && m.Tick == tick {
			msg, found = m, true
		}
	}
	if !found || msg.Focus == 0 {
		return 0, 0, false
	}
	var an game.AnimalView
	found = false
	for _, a := range w.Animals {
		if a.ID == msg.Focus && a.TargetID != 0 {
			an, found = a, true
			break
		}
	}
	if !found {
		return 0, 0, false
	}
	p, ok := ChasedPawn(pawns, an.TargetID)
	if !ok {
		return 0, 0, false
	}
	// The App sets the colony viewport (physical) each frame; unset (no
	// App yet) there is nothing to centre in.
	if vp.W() <= 0 || vp.H() <= 0 {
		return 0, 0, false
	}
	if onScreen(v, float64(an.X)+0.5, float64(an.Y)+0.5, vp) && onScreen(v, float64(p.X)+0.5, float64(p.Y)+0.5, vp) {
		return 0, 0, false
	}
	return float64(an.X+p.X)/2 + 0.5, float64(an.Y+p.Y)/2 + 0.5, true
}

// onScreen reports whether world point (wx, wy) (tiles) is inside the
// physical viewport vp.
func onScreen(v *render.View, wx, wy float64, vp camera.Viewport) bool {
	sx, sy := v.WorldToScreenF(wx, wy)
	return float64(sx) >= vp.X0 && float64(sx) <= vp.X1 && float64(sy) >= vp.Y0 && float64(sy) <= vp.Y1
}
