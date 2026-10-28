package overlay

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Arrival effects (NG-1 §2.11, §7.3): each pod of WorldView.Arrivals falls
// as a streak until its landing tick, then throws a dust burst where its
// shell stands. Presentation only: it reads the stamps and the simulation
// tick, never the sim's state.

const (
	// ArrivalFallTicks is how long before landing the streak shows;
	// arrivalFallTiles how far above the landing tile it starts.
	ArrivalFallTicks = 40
	arrivalFallTiles = 9.0
	// ArrivalDustTicks is how long the dust burst lasts after landing.
	ArrivalDustTicks = 110
)

// ArrivalFx is one effect sprite to draw: key at the centre (x, y) of its
// tile (tiles), alpha and scale.
type ArrivalFx struct {
	Key   string
	X, Y  float64
	Alpha float64
	Scale float64
}

// ArrivalEffects lists the effect sprites of the pods at simulation tick
// now, streaks first. A pod not yet in its fall draws nothing; the streak
// drops onto its tile and brightens as it comes; the dust swells and
// fades. Reduced Motion draws no streak and a dust puff that only fades.
func ArrivalEffects(pods []game.ArrivalView, now uint64, reduced bool) []ArrivalFx {
	var out []ArrivalFx
	for _, p := range pods {
		cx, cy := float64(p.X)+0.5, float64(p.Y)+0.5
		switch {
		case now < p.Tick:
			left := p.Tick - now
			if reduced || left > ArrivalFallTicks {
				continue
			}
			k := float64(left) / ArrivalFallTicks // 1 at the start of the fall, 0 on landing
			out = append(out, ArrivalFx{Key: "effect/pod_fall", X: cx, Y: cy - k*arrivalFallTiles, Alpha: 1 - 0.5*k, Scale: 1})
		case now-p.Tick < ArrivalDustTicks:
			age := float64(now-p.Tick) / ArrivalDustTicks
			scale := 1.0
			if !reduced {
				scale = 0.45 + 0.9*age
			}
			out = append(out, ArrivalFx{Key: "effect/pod_dust", X: cx, Y: cy, Alpha: (1 - age) * 0.85, Scale: scale})
		}
	}
	return out
}

// DrawArrivals draws the pods' fall streaks and dust bursts, unlit like
// the other particles.
func (o *Set) DrawArrivals(v *render.View, screen *ebiten.Image, w game.WorldView, tile float64) {
	if len(w.Arrivals) == 0 || v.Art == nil {
		return
	}
	for _, fx := range ArrivalEffects(w.Arrivals, uint64(v.Tick), v.ReducedMotion) {
		sp := v.Art.Sprite(fx.Key)
		img := v.Art.Image(fx.Key, color.NRGBA{})
		if sp == nil || img == nil {
			continue
		}
		sx, sy := v.WorldToScreenF(fx.X, fx.Y)
		s := tile / art.TilePx
		geo := ebiten.GeoM{}
		geo.Translate(-float64(sp.Meta.Origin[0]), -float64(sp.Meta.Origin[1]))
		geo.Scale(s*fx.Scale, s*fx.Scale)
		geo.Translate(float64(sx), float64(sy))
		op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
		a := float32(fx.Alpha)
		op.ColorScale.Scale(a, a, a, a)
		screen.DrawImage(img, op)
	}
}
