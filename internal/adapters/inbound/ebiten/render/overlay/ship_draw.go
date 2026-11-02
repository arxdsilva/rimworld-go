package overlay

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/ship"
)

// The escape ship's live parts (AX-8, AR4's starship art): the parts'
// indicator lights, a casket's glow while someone sleeps in it (the
// reactor's glow is power_draw.go's, lit once it is started), and the
// launch flare where the ship lifted off. Presentation only: it reads the
// building views, the launch letter and the simulation tick.

// shipLinkTint is the highlight over the selected part's ship.
var shipLinkTint = color.NRGBA{90, 200, 230, 70}

// IsShipKind reports one of the escape ship's six part kinds.
func IsShipKind(kind string) bool {
	switch kind {
	case "ship structural beam", "ship stasis casket", "ship computer core", "ship reactor", "ship engine", "ship sensor cluster":
		return true
	}
	return false
}

// ShipSelOf is Set.ShipSel for the selected building b (sel false: none):
// a ship part's ship, else 0.
func ShipSelOf(b game.BuildingView, sel bool) uint64 {
	if !sel || !IsShipKind(b.Kind) {
		return 0
	}
	return b.AX.ShipID
}

// DrawShipLinks highlights every part of the selected part's ship
// (Set.ShipSel): the connectivity overlay. A part left out (a casket with
// no beam beside it) stays plain.
func (o *Set) DrawShipLinks(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if o.ShipSel == 0 {
		return
	}
	x0, y0, x1, y1 := visibleTiles(v, sw, sh, tile)
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if b.AX.ShipID != o.ShipSel || b.X+b.W < x0 || b.X > x1 || b.Y+b.H < y0 || b.Y > y1 {
			continue
		}
		sx, sy := v.WorldToScreenF(float64(b.X), float64(b.Y))
		ex, ey := v.WorldToScreenF(float64(b.X+max(b.W, 1)), float64(b.Y+max(b.H, 1)))
		o.solidRect(screen, sx, sy, ex-sx, ey-sy, shipLinkTint)
	}
	v.LayerDrawn("ship links")
}

// ShipReactorKind is the ship reactor (its glow: poweredLook).
const ShipReactorKind = "ship reactor"

// LaunchFlareTicks is how long the launch flare burns after the launch.
const LaunchFlareTicks = 240

// DrawShipGlow draws a ship part's glow layer over its sprite (key, just
// drawn): the beam's, engine's, core's and sensor's lights always, a
// casket's while it is occupied (its key is then the occupied sprite).
func (o *Wiring) DrawShipGlow(v *render.View, ch *Chunks, dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	if ch.Baking || v.Art == nil {
		return
	}
	switch b.Kind {
	case "ship structural beam", "ship engine", "ship computer core", "ship sensor cluster":
	case "ship stasis casket":
		if b.AX.Sleeper == "" {
			return
		}
	default:
		return
	}
	if g := o.Parts.glowOf(key); v.Art.Has(g) {
		o.drawUnlitSprite(v, dst, g, float64(b.X), float64(b.Y), tile, sw, sh, sprite.NoCS)
	}
}

// LaunchFlare is the launch flare at simulation tick now: its centre
// (tiles), scale and alpha; ok false once it has burned out, or with no
// launch.
func LaunchFlare(w game.WorldView, now uint64, reduced bool) (x, y, scale, alpha float64, ok bool) {
	if !w.AX.ShipLaunched {
		return 0, 0, 0, 0, false
	}
	for i := len(w.Messages) - 1; i >= 0; i-- {
		m := w.Messages[i]
		if m.Title != ship.LaunchTitle || now < m.Tick || now-m.Tick >= LaunchFlareTicks {
			continue
		}
		age := float64(now-m.Tick) / LaunchFlareTicks
		scale = 3.0
		if !reduced {
			scale = 2.0 + 3.0*age
		}
		return float64(m.X) + 0.5, float64(m.Y) + 0.5, scale, 1 - age, true
	}
	return 0, 0, 0, 0, false
}

// DrawLaunch draws the launch flare, unlit like the other particles.
func (o *Set) DrawLaunch(v *render.View, screen *ebiten.Image, w game.WorldView, tile float64) {
	x, y, scale, alpha, ok := LaunchFlare(w, uint64(v.Tick), v.ReducedMotion)
	if !ok || v.Art == nil {
		return
	}
	const key = "effect/launch_flare"
	sp, img := v.Art.Sprite(key), v.Art.Image(key, color.NRGBA{})
	if sp == nil || img == nil {
		return
	}
	sx, sy := v.WorldToScreenF(x, y)
	s := tile / art.TilePx * scale
	geo := ebiten.GeoM{}
	geo.Translate(-float64(sp.Meta.Origin[0]), -float64(sp.Meta.Origin[1]))
	geo.Scale(s, s)
	geo.Translate(float64(sx), float64(sy))
	op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
	a := float32(alpha)
	op.ColorScale.Scale(a, a, a, a)
	screen.DrawImage(img, op)
}
