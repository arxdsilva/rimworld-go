package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// The world draw order, the zoom tiers and bakes, the object pass, the
// labels and the terrain, plant, item and building art keys live in
// render/world (ARCH-004 wave 4.4). These aliases and one-line wrappers
// keep the root's call sites unchanged until the sweep repoints them
// (SOLID_GO.md §11). The renderer keeps the zoom state (lod), the link
// grid and the stone cache as fields this wave; the pipeline holds
// pointers to them.

// pipeSource is the renderer as the pipeline's world.Source.
type pipeSource struct{ r *Renderer }

func (s pipeSource) World() game.WorldView                { return s.r.game.World() }
func (s pipeSource) Snapshot() []game.PawnView            { return s.r.game.Snapshot() }
func (s pipeSource) Date() clock.Date                     { return s.r.game.Date() }
func (s pipeSource) ChunkLayer(cx, cy int) game.ChunkView { return s.r.game.ChunkLayer(cx, cy) }
func (s pipeSource) PlantsIn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) ([]int32, bool) {
	g := s.r.game
	if g == nil {
		return buf, false
	}
	if cur := g.World(); litFirst(cur.Plants) == litFirst(plants) && len(cur.Plants) == len(plants) {
		return g.PlantsIn(x0, y0, x1, y1, buf), true
	}
	return buf, false
}
func (s pipeSource) DrawOrderRoute(screen *ebiten.Image, tile float64) {
	s.r.drawOrderRoute(screen, tile)
}

// pipeline is the world pipeline over the renderer's layers, built on
// first use, with every layer's catalog kept current (the App sets r.art
// after New; tests swap it), as the old per-call accessors did.
func (r *Renderer) pipeline() *world.Pipeline {
	lit, e, c, o, cb := r.litLayer(), r.envLayer(), r.creatures(), r.overlays(), r.cueBus()
	if r.wp == nil {
		r.wp = world.New(
			world.Layers{Lit: lit, Env: e, Creatures: c, Overlays: o, Cues: cb, Shadows: &r.env.Shadow, Motion: &r.mo},
			world.Hosts{Index: envHost{r}, Stamp: envHost{r}, Casters: shadowHost{r}},
			world.Shared{Cam: &r.cam, Cast: &r.cast, Blood: &r.figureBlood, LOD: &r.lod, Links: &r.links, Stone: &r.stoneArt},
			&r.view, pipeSource{r})
	}
	r.wp.SetArt(r.art)
	return r.wp
}

// pipe is pipeline for the primitives the layers call back per tile or
// per mask (drawTile, the bakes, the chunk lists): the painter's catalog
// kept current, the other layers left as they are.
func (r *Renderer) pipe() *world.Pipeline {
	if r.wp == nil {
		return r.pipeline()
	}
	r.wp.SetArt(r.art)
	return r.wp
}

// worldView fills what the pipeline reads of the frame context for an
// entry point called outside Renderer.Draw (tests, the HUD), as the old
// per-layer views did per call.
func (r *Renderer) worldView() {
	v := &r.view
	v.Cam, v.Art, v.Present = r.cam, r.art, r.present
	v.AnimT, v.Fx = r.animT(), r.fx
	if r.game != nil {
		v.MapW, v.MapH, v.Tick = r.game.Width(), r.game.Height(), r.game.Tick()
	}
	v.Selected, v.HasSelect = r.selected, r.hasSelect
	v.ReducedMotion, v.ReducedFlashing = r.reducedMotion, r.reducedFlashing
}

// lightPrep is worldView's part a lit primitive reads: the catalog and the
// presentation frame (the camera comes from r.cam).
func (r *Renderer) lightPrep() { r.view.Art, r.view.Present = r.art, r.present }

func (r *Renderer) drawPuff(screen *ebiten.Image, f int, sx, sy float32, tile, k float64, cs sprite.ColorScale) bool {
	return r.pipe().Lb.DrawPuff(screen, f, sx, sy, tile, k, cs)
}

func (r *Renderer) drawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs sprite.ColorScale) {
	r.lightPrep()
	r.pipe().DrawTile(screen, key, wx, wy, tile, sw, sh, cs)
}

func (r *Renderer) animalPads(tile float64) (pad, padS int) { return world.AnimalPadsOf(r.art, tile) }
