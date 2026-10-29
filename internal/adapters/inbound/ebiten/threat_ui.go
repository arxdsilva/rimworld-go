package ebiten

import (
	"rimworld-go/internal/application/game"
)

// Threat pause (presentation only); the floating damage numbers are
// render/creature's (floaters.go).

// watchThreats pauses the game when a new danger message arrives (if the
// player wants that). Messages already there when the session starts or
// a save loads never pause.
func (r *Renderer) watchThreats(w game.WorldView) {
	newest := r.lastDanger
	for _, m := range w.Messages {
		if m.Danger && m.Tick > newest {
			newest = m.Tick
		}
	}
	if !r.dangerInit {
		r.dangerInit, r.lastDanger = true, newest
		return
	}
	if newest == r.lastDanger {
		return
	}
	r.lastDanger = newest
	r.threatAt = r.present
	r.cueBus().Global("ui.threat")
	if r.pauseOnThreat && r.speed != speedPaused {
		r.speed = speedPaused
		r.threatPaused = true
		r.onThreatPause(w, newest) // show the player the chase
	}
}

// onThreatPause brings an off-screen chase into the colony viewport when
// its warning has just paused the game (render/creature chase.go finds it;
// the camera is the renderer's).
func (r *Renderer) onThreatPause(w game.WorldView, tick uint64) {
	vp := r.cueBus().Voice.View
	if x, y, ok := r.pipe().Creatures.OnThreatPause(r.liveView(), w, r.game.Snapshot(), tick, vp); ok {
		r.centerOnView(x, y, vp, vp)
	}
}

// speedRate is the HUD speed label, saying why the game paused itself.
func (r *Renderer) speedRate() string {
	if r.threatPaused && r.speed == speedPaused {
		return "paused (threat)"
	}
	r.threatPaused = false
	return r.speed.rate()
}
