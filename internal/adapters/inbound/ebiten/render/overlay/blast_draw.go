package overlay

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Blast and shield effects (CBT-5 UI): the per-kind explosion sprites at a
// burst (fire, EMP flash, a smoke cloud that lingers) and the bubble round
// a pawn whose shield belt is up. Presentation only, read from
// WorldView.Bursts and the shield feed; a frame with neither draws nothing
// and none allocates.

// blastFx is how a blast kind looks: its sprite and how long it shows
// (simulation ticks), and whether it swells as it ages.
type blastFx struct {
	key   string
	life  uint64
	swell float64 // extra scale gained over the life
}

// blastLook is the effect of blast kind ("" for kinds drawn elsewhere).
func blastLook(kind string) (blastFx, bool) {
	switch kind {
	case "flame":
		return blastFx{"effect/fire_burst", 50, 0.25}, true
	case "EMP":
		return blastFx{"effect/emp_flash", 30, 0.35}, true
	case "smoke":
		return blastFx{"effect/smoke_cloud", 1200, 0.3}, true
	}
	return blastFx{}, false
}

// BlastAlpha is a blast's opacity at age ticks of life: full, then fading
// over the last third.
func BlastAlpha(age, life uint64) float64 {
	if age >= life {
		return 0
	}
	if fade := life / 3; age > life-fade && fade > 0 {
		return float64(life-age) / float64(fade)
	}
	return 1
}

// drawFx draws sprite key centred on its origin at tile (cx, cy), scaled
// so it spans tiles tiles across, at alpha.
func drawFx(v *render.View, screen *ebiten.Image, key string, cx, cy, tiles, alpha, tile float64) {
	sp := v.Art.Sprite(key)
	img := v.Art.Image(key, color.NRGBA{})
	if sp == nil || img == nil || alpha <= 0 {
		return
	}
	sx, sy := v.WorldToScreenF(cx, cy)
	k := tile / art.TilePx
	span := float64(img.Bounds().Dx()) / art.TilePx // the sprite's width in tiles
	sc := k * tiles / span
	geo := ebiten.GeoM{}
	geo.Translate(-float64(sp.Meta.Origin[0]), -float64(sp.Meta.Origin[1]))
	geo.Scale(sc, sc)
	geo.Translate(float64(sx), float64(sy))
	op := ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
	a := float32(alpha)
	op.ColorScale.Scale(a, a, a, a)
	screen.DrawImage(img, &op)
}

// DrawBlasts draws each recent weapon blast of w by its kind, sized to
// its radius (BurstView.Radius, tenths of a tile).
func (c *Combat) DrawBlasts(v *render.View, screen *ebiten.Image, w game.WorldView, tile float64) {
	if len(w.Bursts) == 0 || v.Art == nil {
		return
	}
	now := uint64(v.Tick)
	for i := range w.Bursts {
		b := &w.Bursts[i]
		fx, ok := blastLook(b.Blast)
		if !ok || now < b.Tick || now-b.Tick >= fx.life {
			continue
		}
		age := now - b.Tick
		span := 2 * max(float64(b.Radius)/10, 1)
		span *= 1 + fx.swell*float64(age)/float64(fx.life)
		drawFx(v, screen, fx.key, float64(b.X)+0.5, float64(b.Y)+0.5, span, BlastAlpha(age, fx.life), tile)
	}
}

// shieldBubbleTiles is the bubble's height round a body, in tiles.
const shieldBubbleTiles = 1.5

// DrawShields draws the shield bubble round each pawn of the Shields
// feed whose belt holds charge, fainter as the charge runs down.
func (c *Combat) DrawShields(v *render.View, screen *ebiten.Image, tile float64) {
	if len(c.Shields) == 0 || v.Art == nil {
		return
	}
	for i := range c.Shields {
		s := &c.Shields[i]
		if s.Pct <= 0 {
			continue
		}
		a := 0.35 + 0.45*float64(min(s.Pct, 100))/100
		drawFx(v, screen, "effect/shield_bubble", float64(s.X)+0.5, float64(s.Y)+0.4, shieldBubbleTiles*256/224, a, tile)
	}
}
