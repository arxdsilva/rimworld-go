package ebiten

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Campfire presentation: a flickering flame drawn over the stone ring,
// a warm glow, and rising embers and smoke while it burns.

// fireHeight lifts the flame (tiles) to sit on the logs.
const fireHeight = 0.22

// drawFires draws the flame and glow of every burning campfire.
func (r *Renderer) drawFires(screen *ebiten.Image, w game.WorldView, tile float64) {
	for _, b := range w.Buildings {
		if !b.Burning {
			continue
		}
		t := float64(r.present) + float64(b.ID)*17
		f := 0.75 + 0.25*math.Sin(t*0.31)*math.Sin(t*0.13+2)
		cx, cy := float64(b.X)+0.5, float64(b.Y)+0.5-fireHeight
		sx, sy := r.worldToScreenF(cx, cy)
		// glow on the ground around the fire
		vector.FillCircle(screen, sx, sy+float32(tile*0.12), float32(tile*0.75), fade(color.RGBA{255, 150, 60, 255}, 0.10*f), true)
		vector.FillCircle(screen, sx, sy+float32(tile*0.12), float32(tile*0.42), fade(color.RGBA{255, 170, 80, 255}, 0.16*f), true)
		// flame: tongues that sway and shrink upward, red to yellow
		for i, c := range []color.RGBA{{205, 60, 30, 255}, {245, 130, 40, 255}, {255, 205, 90, 255}} {
			base := float32(tile * (0.17 - 0.04*float64(i)) * f)
			for k := 0; k < 4; k++ {
				h := float64(k) / 4
				sway := math.Sin(t*0.22+h*3+float64(i)) * 0.03 * h
				px, py := r.worldToScreenF(cx+sway, cy-h*0.32*f)
				vector.FillCircle(screen, px, py, base*float32(1-h*0.7), fade(c, 0.9), true)
			}
		}
	}
}

// emitFireParticles sends embers and smoke up from burning campfires.
func (r *Renderer) emitFireParticles(w game.WorldView) {
	for _, b := range w.Buildings {
		if !b.Burning {
			continue
		}
		seed := uint64(b.ID)*104729 + r.present
		x, y := float64(b.X)+0.5, float64(b.Y)+0.5-fireHeight-0.2
		if (r.present+uint64(b.ID))%9 == 0 {
			r.burst(partEmber, x, y, 1, seed)
		}
		if (r.present+uint64(b.ID))%14 == 0 {
			r.burst(partSmoke, x, y-0.15, 1, seed^0x5bd1e995)
		}
	}
}
