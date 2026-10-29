package ebiten

import (
	"image/color"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The ground cover and weather live in render/env (ARCH-004). These
// aliases and one-line wrappers keep the root's call sites unchanged while
// the layers move out one package at a time; each mover repoints its own
// files to env and the last one deletes this file (SOLID_GO.md §11).

// --- the layer ---------------------------------------------------------------

// envLayer is the renderer's environment layer, bound to its passes' state
// (the renderer still owns those fields: settings.go and draw_art.go reach
// them), its plant keys, its light layer and a painter over its live art.
// The lead's New gives the layer its own (env.New); until then it binds
// here on every call, a few pointer stores.
func (r *Renderer) envLayer() *env.Layer {
	e := &r.env.Layer
	if e.Water == nil {
		e.Water, e.SnowC, e.Trees, e.Grass, e.Crops = &r.water, &r.snowc, &r.treeFx, &r.lod.Grass, &r.lod.Crops
		e.Keys = worldpkg.PlantKeys.PlantKeyTable
	}
	r.env.paint = r.painter()
	e.P, e.Lit = &r.env.paint, r.litLayer()
	return e
}

// envView is the frame view the env layer reads (spriteView: the
// renderer's live camera, which a bake moves) with the map size, the
// options and Reduced Motion copied in: tests drive these entry points
// without Renderer.Draw filling the view. The caller puts frame back
// after the call, so the view keeps the frame's camera.
func (r *Renderer) envView() (v *render.View, frame camerapkg.Camera) {
	v, frame = r.spriteView()
	if r.game != nil {
		v.MapW, v.MapH = r.game.Width(), r.game.Height()
	}
	v.Fx, v.ReducedMotion, v.ReducedFlashing = r.fx, r.reducedMotion, r.reducedFlashing
	return v, frame
}

// envClock is the view for the per-plant entry points (PlantPose,
// TreeCanopyAlpha, SeasonKey): the art, the presentation frame and
// Reduced Motion, without the camera.
func (r *Renderer) envClock() *render.View {
	v := &r.view
	v.Art, v.Present, v.ReducedMotion = r.art, r.present, r.reducedMotion
	return v
}

// envHost is the renderer as the env layer's Chunks, Stamp and Movers.
type envHost struct{ r *Renderer }

func (h envHost) ChunkView(cx, cy int) game.ChunkView { return h.r.pipe().ChunkView(cx, cy) }
func (h envHost) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return h.r.pipe().PlantsOn(plants, x0, y0, x1, y1, buf)
}
func (h envHost) FrameLists(l env.Lists) bool { return h.r.frameLists(l) }
func (h envHost) Current(bldgs []game.BuildingView) bool {
	cur := h.r.game.World()
	return litFirst(cur.Buildings) == litFirst(bldgs) && len(cur.Buildings) == len(bldgs)
}
func (h envHost) DrawBakedColor(screen, src *ebiten.Image, bx, by int, sx0, sy0, bpx float64, tx0, ty0, tx1, ty1 int, col [4]float32) {
	h.r.pipe().Bk.DrawBakedColor(screen, src, bx, by, sx0, sy0, bpx, tx0, ty0, tx1, ty1, col)
}
func (h envHost) DrawChunkFlat(screen, src *ebiten.Image, srcX, srcY, dstX, dstY, span, srcSpan float64, col [4]float32) {
	h.r.pipe().Bk.DrawChunkFlat(screen, src, srcX, srcY, dstX, dstY, span, srcSpan, col)
}
func (h envHost) DrawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs spritepkg.ColorScale) {
	h.r.drawTile(screen, key, wx, wy, tile, sw, sh, cs)
}
func (h envHost) DrawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps motionpkg.Pose) {
	h.r.drawSpritePose(screen, key, tint, wx, wy, tile, ps)
}
func (h envHost) InView(x, y, pad, padS int) bool                     { return h.r.lod.InView(x, y, pad, padS) }
func (h envHost) PawnMotion(id entity.EntityID) motionpkg.MoveState   { return h.r.pawnMotion(id) }
func (h envHost) AnimalMotion(id entity.EntityID) motionpkg.MoveState { return h.r.animalMotion(id) }

// frameLists reports whether l is the world this frame's chunk lists
// index (r.view.W, synced by syncFrameChunks): the same shared lists.
func (r *Renderer) frameLists(l env.Lists) bool {
	d := &r.view.W
	return r.game != nil && len(r.lod.Views) > 0 &&
		litFirst(l.Buildings) == litFirst(d.Buildings) && len(l.Buildings) == len(d.Buildings) &&
		litFirst(l.Plants) == litFirst(d.Plants) && len(l.Plants) == len(d.Plants) &&
		litFirst(l.Items) == litFirst(d.Items) && len(l.Items) == len(d.Items) &&
		litFirst(l.Frames) == litFirst(d.Frames) && len(l.Frames) == len(d.Frames)
}

// frameViews is the frame's chunk lists when w is the world they index,
// else nil (env.Layer.DrawFurrows walks w then).
func (r *Renderer) frameViews(w *game.WorldView) []game.ChunkView {
	if !r.frameLists(env.ListsOf(w)) {
		return nil
	}
	return r.lod.Views
}

// stepEnvironment advances springs and weather particles once per frame
// (env.Layer.Step), drops the autumn leaves of the trees brushed and puffs
// the roof collapses' dust.
func (r *Renderer) stepEnvironment(w game.WorldView, pawns []game.PawnView) {
	v, frame := r.envView()
	e := r.envLayer()
	e.Step(v, w, pawns, r.mo.Stepped, envHost{r}, envHost{r})
	v.Cam = frame
	for _, t := range e.Leaves {
		r.dropLeaves(t[0], t[1])
	}
	r.stepCollapses(w)
}

func (r *Renderer) seasonKey(key string, w game.WorldView) string {
	return r.envLayer().SeasonKey(r.envClock(), key, w)
}

func (r *Renderer) setView(sw, sh int, tile float64) {
	v, frame := r.envView()
	r.envLayer().SetView(v, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) visibleTiles() (x0, y0, x1, y1 int) {
	v, frame := r.envView()
	x0, y0, x1, y1 = r.envLayer().VisibleTiles(v)
	v.Cam = frame
	return x0, y0, x1, y1
}

// SetTreeFocus lists the tiles of the hovered/selected entity or the
// Architect ghost; canopies over them turn see-through too. Replaces
// the previous list; call with nil to clear.
func (r *Renderer) SetTreeFocus(cells [][2]int) { r.envLayer().Trees.SetTreeFocus(cells) }
