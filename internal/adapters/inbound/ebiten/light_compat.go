package ebiten

import (
	lightpkg "rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
)

// Lighting and shadows live in render/light (ARCH-004). These aliases and
// one-line wrappers keep the root's call sites unchanged while the layers
// move out one package at a time; each mover repoints its own files to
// light and the last one deletes this file (SOLID_GO.md §11).

type (
	shadowState = light.Shadows[creature.FigurePose]

	creatureCaster = light.Creature[creature.FigurePose]
)

func litFirst[T any](s []T) *T { return light.First(s) }

// litLayer is the renderer's light layer with the fire light hooked in
// (a burning cell's reach and strength live with the fires,
// fire_world_draw.go).
func (r *Renderer) litLayer() *lightpkg.Lit {
	if r.lit.Blaze == nil {
		r.lit.Blaze = overlay.BlazeLight
	}
	return &r.lit
}

// lightView is the frame view the light layer reads, with the renderer's
// live art, presentation frame and, outside a bake, camera copied in:
// tests drive these entry points without Renderer.Draw filling the view,
// and a bake moves r.cam but never the view.
func (r *Renderer) lightView() *render.View {
	v := &r.view
	v.Art, v.Present = r.art, r.present
	if !r.lod.Baking {
		v.Cam = r.cam
	}
	return v
}

// shadowView is the visible rectangle (setView) as the shadow pass reads it.
func (r *Renderer) shadowView() light.Rect {
	v := r.env.View
	return light.Rect{X0: v.X0, Y0: v.Y0, X1: v.X1, Y1: v.Y1}
}
