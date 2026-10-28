package overlay

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Campfire presentation: a flickering flame drawn over the stone ring,
// a warm glow, and rising embers and smoke while it burns.

// FireHeight lifts the flame (tiles) to sit on the logs.
const FireHeight = 0.22

// DrawFires draws the flame and glow of every burning campfire (the
// torch lamp burns its own flame, power_draw.go; the generator shows none).
func (o *Fires) DrawFires(v *render.View, screen *ebiten.Image, w game.WorldView, tile float64) {
	for _, b := range w.Buildings {
		if !b.Burning || OwnsFlame(b.Kind) {
			continue
		}
		// ground glow, then tongues that sway and shrink upward, red to yellow
		t := float64(v.Present) + float64(b.ID)*17
		o.drawFlame(v, screen, float64(b.X)+0.5, float64(b.Y)+0.5-FireHeight, t, tile, 1)
	}
}

// EmitFireParticles sends embers and smoke up from burning campfires.
func (o *Fires) EmitFireParticles(v *render.View, w game.WorldView) {
	for _, b := range w.Buildings {
		if !b.Burning || OwnsFlame(b.Kind) {
			continue
		}
		seed := uint64(b.ID)*104729 + v.Present
		x, y := float64(b.X)+0.5, float64(b.Y)+0.5-FireHeight-0.2
		if (v.Present+uint64(b.ID))%9 == 0 {
			o.Host.Burst(Ember, x, y, 1, seed)
		}
		if (v.Present+uint64(b.ID))%14 == 0 {
			o.Host.Burst(Smoke, x, y-0.15, 1, seed^0x5bd1e995)
		}
	}
}

// drawFlame draws the procedural flame at world (cx, cy) (Host.Flame).
func (o *base) drawFlame(v *render.View, dst *ebiten.Image, cx, cy, t, tile, k float64) {
	sx, sy := v.WorldToScreenF(cx, cy)
	o.Host.Flame(dst, sx, sy, t, tile, k)
}
