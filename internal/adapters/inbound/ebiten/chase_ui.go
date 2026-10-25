package ebiten

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017: an animal chasing a colonist is shown, not just announced: a
// red mark over the animal and a red line to its target (both world draws,
// in draw_combat.go), and a camera that brings an off-screen chase into
// view when the warning pauses the game (here).

// chasedPawn is the pawn with id, if there is one.
func chasedPawn(pawns []game.PawnView, id entity.EntityID) (game.PawnView, bool) {
	for _, p := range pawns {
		if p.ID == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// onThreatPause runs when a new danger message (posted at tick) has just
// auto-paused the game. When that message is an animal starting to chase
// a colonist and the chase is not wholly on screen, the camera centres
// between the animal and the colonist. Any other danger leaves it alone.
func (r *Renderer) onThreatPause(w game.WorldView, tick uint64) {
	// the newest danger posted at tick: oldest first, so the last one
	var msg game.MessageView
	found := false
	for i := len(w.Messages) - 1; i >= 0 && !found; i-- {
		if m := w.Messages[i]; m.Danger && m.Tick == tick {
			msg, found = m, true
		}
	}
	if !found || msg.Focus == 0 {
		return
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
		return
	}
	p, ok := chasedPawn(r.game.Snapshot(), an.TargetID)
	if !ok {
		return
	}
	// The App sets the colony viewport (physical) each frame; unset (no
	// App yet) there is nothing to centre in.
	v := r.voice.view
	if v.w() <= 0 || v.h() <= 0 {
		return
	}
	if r.onScreen(float64(an.X)+0.5, float64(an.Y)+0.5, v) && r.onScreen(float64(p.X)+0.5, float64(p.Y)+0.5, v) {
		return
	}
	r.centerOnView(float64(an.X+p.X)/2+0.5, float64(an.Y+p.Y)/2+0.5, v, v)
}

// onScreen reports whether world point (wx, wy) (tiles) is inside the
// physical viewport v.
func (r *Renderer) onScreen(wx, wy float64, v viewport) bool {
	sx, sy := r.worldToScreenF(wx, wy)
	return float64(sx) >= v.x0 && float64(sx) <= v.x1 && float64(sy) >= v.y0 && float64(sy) <= v.y1
}
