package ebiten

import (
	"image/color"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
)

// The sprite painter, the linked-building grid and keys, the stone tints
// and the fence sprites live in render/sprite (ARCH-004 wave 4.2). These
// aliases and one-line wrappers keep the root's call sites unchanged until
// the world pipeline moves (wave 4.4) and repoints them (SOLID_GO.md §11).

type (

	// linkGrid classifies building kinds with the renderer's names
	// (overlay.LinkKinds, render/overlay/power_draw.go).
	linkGrid = sprite.LinkGrid[overlay.LinkKinds]
)

// painter is the renderer's sprite painter over its live catalog, light
// layer (fire light hooked in) and stone cache. The lead's New gives the
// renderer its own (sprite.NewPainter); until then it is built per call,
// on the stack.
func (r *Renderer) painter() sprite.Painter {
	return sprite.Painter{Art: r.art, Lit: r.litLayer(), Stone: &r.stoneArt.StoneCache}
}

// spritePass is the pass the renderer draws sprites in: the shadow
// pass's cast, a bake, a figure's blood.
func (r *Renderer) spritePass() sprite.Pass {
	return sprite.Pass{Cast: r.cast, Baking: r.lod.Baking, Blood: r.figureBlood}
}

// spriteView is the view a sprite draw reads (lightView) with the
// renderer's live camera, which a bake moves; the caller puts frame back
// after the draw, so the view keeps the frame's camera.
func (r *Renderer) spriteView() (v *render.View, frame camerapkg.Camera) {
	v = r.lightView()
	frame, v.Cam = v.Cam, r.cam
	return v, frame
}

func (r *Renderer) drawSprite(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int) {
	v, frame := r.spriteView()
	p := r.painter()
	p.Draw(v, r.spritePass(), screen, key, tint, wx, wy, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) drawSpriteCS(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs spritepkg.ColorScale) {
	v, frame := r.spriteView()
	p := r.painter()
	p.DrawCS(v, r.spritePass(), screen, key, tint, wx, wy, tile, sw, sh, cs)
	v.Cam = frame
}

// drawLinked draws a linked family sprite now; false when there is no art
// (sprite.Painter.DrawLinked).
func (r *Renderer) drawLinked(screen *ebiten.Image, prefix string, x, y, mask, corners int, tint color.NRGBA, cs spritepkg.ColorScale, tile float64, sw, sh int) bool {
	v, frame := r.spriteView()
	p := r.painter()
	ok := p.DrawLinked(v, r.spritePass(), screen, prefix, x, y, mask, corners, tint, cs, tile, sw, sh)
	v.Cam = frame
	return ok
}

func (r *Renderer) stuffTint(stuff string) color.NRGBA {
	p := r.painter()
	return p.StuffTint(stuff)
}
