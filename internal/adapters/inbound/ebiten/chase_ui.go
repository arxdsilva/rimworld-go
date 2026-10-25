package ebiten

import "rimworld-go/internal/application/game"

// PLAY-017: an animal chasing a colonist is shown, not just announced.

// onThreatPause runs when a new danger message (posted at tick) has just
// auto-paused the game.
func (r *Renderer) onThreatPause(w game.WorldView, tick uint64) {}
