package ebiten

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Arrival effects (NG-1 §2.11, §7.3): each pod of WorldView.Arrivals falls
// as a streak until its landing tick, then throws a dust burst where its
// shell stands. Presentation only: it reads the stamps and the simulation
// tick, never the sim's state.

const (
	// arrivalFallTicks is how long before landing the streak shows;
	// arrivalFallTiles how far above the landing tile it starts.
	arrivalFallTicks = 40
	arrivalFallTiles = 9.0
	// arrivalDustTicks is how long the dust burst lasts after landing.
	arrivalDustTicks = 110
)

// arrivalFx is one effect sprite to draw: key at the centre (x, y) of its
// tile (tiles), alpha and scale.
type arrivalFx struct {
	key   string
	x, y  float64
	alpha float64
	scale float64
}

// arrivalEffects lists the effect sprites of the pods at simulation tick
// now, streaks first. A pod not yet in its fall draws nothing; the streak
// drops onto its tile and brightens as it comes; the dust swells and
// fades. Reduced Motion draws no streak and a dust puff that only fades.
func arrivalEffects(pods []game.ArrivalView, now uint64, reduced bool) []arrivalFx {
	var out []arrivalFx
	for _, p := range pods {
		cx, cy := float64(p.X)+0.5, float64(p.Y)+0.5
		switch {
		case now < p.Tick:
			left := p.Tick - now
			if reduced || left > arrivalFallTicks {
				continue
			}
			k := float64(left) / arrivalFallTicks // 1 at the start of the fall, 0 on landing
			out = append(out, arrivalFx{key: "effect/pod_fall", x: cx, y: cy - k*arrivalFallTiles, alpha: 1 - 0.5*k, scale: 1})
		case now-p.Tick < arrivalDustTicks:
			age := float64(now-p.Tick) / arrivalDustTicks
			scale := 1.0
			if !reduced {
				scale = 0.45 + 0.9*age
			}
			out = append(out, arrivalFx{key: "effect/pod_dust", x: cx, y: cy, alpha: (1 - age) * 0.85, scale: scale})
		}
	}
	return out
}

// drawArrivals draws the pods' fall streaks and dust bursts, unlit like
// the other particles.
func (r *Renderer) drawArrivals(screen *ebiten.Image, w game.WorldView, tile float64) {
	if len(w.Arrivals) == 0 || r.art == nil {
		return
	}
	for _, fx := range arrivalEffects(w.Arrivals, uint64(r.game.Tick()), r.reducedMotion) {
		sp := r.art.Sprite(fx.key)
		img := r.art.Image(fx.key, color.NRGBA{})
		if sp == nil || img == nil {
			continue
		}
		sx, sy := r.worldToScreenF(fx.x, fx.y)
		s := tile / artTilePx
		geo := ebiten.GeoM{}
		geo.Translate(-float64(sp.Meta.Origin[0]), -float64(sp.Meta.Origin[1]))
		geo.Scale(s*fx.scale, s*fx.scale)
		geo.Translate(float64(sx), float64(sy))
		op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
		a := float32(fx.alpha)
		op.ColorScale.Scale(a, a, a, a)
		screen.DrawImage(img, op)
	}
}
