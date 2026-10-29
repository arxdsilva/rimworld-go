package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The ground cover and weather live in render/env (ARCH-004). The renderer
// steps the layer once a frame (stepEnvironment) and is its Chunks, Stamp
// and Movers (envHost). The layer calls the host inside a frame the
// renderer has prepared (Draw, or an entry point's liveView), from the
// pipeline it belongs to, so the host reads the pipeline and the frame
// view as they are.

// stepEnvironment advances springs and weather particles once per frame
// (env.Layer.Step), drops the autumn leaves of the trees brushed and puffs
// the roof collapses' dust.
func (r *Renderer) stepEnvironment(w game.WorldView, pawns []game.PawnView) {
	p, v := r.pipe(), r.liveView()
	p.Env.Step(v, w, pawns, r.mo.Stepped, envHost{r}, envHost{r})
	for _, t := range p.Env.Leaves {
		dropLeaves(p.Creatures, v, t[0], t[1])
	}
	r.env.stepCollapses(p.Creatures, v, w)
}

// seasonKey is a plant key in this season's dress (env.Layer.SeasonKey).
func (r *Renderer) seasonKey(key string, w game.WorldView) string {
	return r.pipe().Env.SeasonKey(r.liveView(), key, w)
}

// envHost is the renderer as the env layer's Chunks, Stamp and Movers.
type envHost struct{ r *Renderer }

func (h envHost) ChunkView(cx, cy int) game.ChunkView { return h.r.wp.ChunkView(cx, cy) }
func (h envHost) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return h.r.wp.PlantsOn(plants, x0, y0, x1, y1, buf)
}

// FrameLists reports whether l is the world this frame's chunk lists
// index (r.view.W, synced by the pipeline's chunk pass): the same shared
// lists.
func (h envHost) FrameLists(l env.Lists) bool {
	r := h.r
	d := &r.view.W
	return r.game != nil && r.wp != nil && len(r.wp.LOD.Views) > 0 &&
		light.First(l.Buildings) == light.First(d.Buildings) && len(l.Buildings) == len(d.Buildings) &&
		light.First(l.Plants) == light.First(d.Plants) && len(l.Plants) == len(d.Plants) &&
		light.First(l.Items) == light.First(d.Items) && len(l.Items) == len(d.Items) &&
		light.First(l.Frames) == light.First(d.Frames) && len(l.Frames) == len(d.Frames)
}

func (h envHost) Current(bldgs []game.BuildingView) bool {
	cur := h.r.game.World()
	return light.First(cur.Buildings) == light.First(bldgs) && len(cur.Buildings) == len(bldgs)
}
func (h envHost) DrawBakedColor(screen, src *ebiten.Image, bx, by int, sx0, sy0, bpx float64, tx0, ty0, tx1, ty1 int, col [4]float32) {
	h.r.wp.Bk.DrawBakedColor(screen, src, bx, by, sx0, sy0, bpx, tx0, ty0, tx1, ty1, col)
}
func (h envHost) DrawChunkFlat(screen, src *ebiten.Image, srcX, srcY, dstX, dstY, span, srcSpan float64, col [4]float32) {
	h.r.wp.Bk.DrawChunkFlat(screen, src, srcX, srcY, dstX, dstY, span, srcSpan, col)
}
func (h envHost) DrawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs spritepkg.ColorScale) {
	h.r.wp.DrawTile(screen, key, wx, wy, tile, sw, sh, cs)
}
func (h envHost) DrawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps motionpkg.Pose) {
	h.r.wp.DrawSpritePose(screen, key, tint, wx, wy, tile, ps)
}
func (h envHost) InView(x, y, pad, padS int) bool { return h.r.wp.LOD.InView(x, y, pad, padS) }

// PawnMotion is the on-screen motion of a colonist or raider at the
// frame's animT.
func (h envHost) PawnMotion(id entity.EntityID) motionpkg.MoveState {
	m, _ := h.r.mo.MotionOf(id, h.r.view.AnimT)
	return m
}

// AnimalMotion is the on-screen motion of an animal at the frame's animT.
func (h envHost) AnimalMotion(id entity.EntityID) motionpkg.MoveState {
	m, _ := h.r.mo.MotionOf(id, h.r.view.AnimT)
	return m
}
