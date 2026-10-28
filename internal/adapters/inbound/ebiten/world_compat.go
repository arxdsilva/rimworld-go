package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
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

type (
	lodState      = world.LOD
	lodTier       = world.Tier
	artDraw       = world.ArtDraw
	stoneArtCache = world.StoneArt
	groundKeys    = world.GroundKeys
	// linkKinds is the renderer's sprite.Kinds (render/overlay).
	linkKinds = overlay.LinkKinds
)

const (
	lodLive           = world.Live
	lodChunk          = world.Chunk
	lodMap            = world.Map
	lodLiveMin        = world.LiveMin
	lodChunkMin       = world.ChunkMin
	decalMinTile      = world.DecalMinTile
	creatureMinTile   = world.CreatureMinTile
	visPad            = world.VisPad
	bakeBudget        = world.BakeBudget
	soilVariants      = world.SoilVariants
	grassEvery        = world.GrassEvery
	groundVariants    = world.GroundVariants
	stackCountMinTile = world.StackCountMinTile
	markerAboveHead   = world.MarkerAboveHead
	chatAboveHead     = world.ChatAboveHead
	raiderCloth       = world.RaiderCloth
	smoothMarkKey     = world.SmoothMarkKey
	haulMarkKey       = world.HaulMarkKey
)

var (
	terrainPriority  = world.TerrainPriority
	terrainSprite    = world.TerrainSprite
	terrainByPri     = world.TerrainByPri
	terrainNames     = world.TerrainNames
	terrainPriByCode = world.TerrainPriByCode
	isWaterCode      = world.IsWaterCode
	terrainEdgeKeys  = world.TerrainEdgeKeys
)

func artKey(family, name string, variant int) string { return env.ArtKey(family, name, variant) }
func twoDigits(n int) string                         { return sprite.TwoDigits(n) }
func terrainCode(w game.WorldView, x, y int) (uint8, bool) {
	return env.TerrainCode(w, x, y)
}
func itemSprite(kind string) string                     { return world.ItemSprite(kind) }
func plantSprite(p game.PlantView) (string, float64)    { return world.PlantSprite(p) }
func sizedItemKey(key string, qty int) string           { return world.SizedItemKey(key, qty) }
func caveMouthFacing(w game.WorldView, x, y int) string { return world.CaveMouthFacing(w, x, y) }
func clampStage(s int) int                              { return world.ClampStage(s) }
func fenceLabelAt(w game.WorldView, x, y int) string    { return world.FenceLabelAt(w, x, y) }
func interactionIconKey(kind string) string             { return world.InteractionIconKey(kind) }
func lodFor(tile float64) lodTier                       { return world.TierFor(tile) }
func lodTile(tile float64) float64                      { return world.LodTile(tile) }
func onFloor(kind string) bool                          { return world.OnFloor(kind) }
func pawnMarkerLift(c *artCatalog) float64              { return world.PawnMarkerLift(c) }
func roomLabelAt(w game.WorldView, x, y int) string     { return world.RoomLabelAt(w, x, y) }
func troughKey(b game.BuildingView) string              { return world.TroughKey(b) }
func creatureAt(wx, wy, tile, ct float64) (float64, float64) {
	return world.CreatureAt(wx, wy, tile, ct)
}
func interactionRecent(p game.PawnView, now clock.Tick) bool {
	return world.InteractionRecent(p, now)
}
func newPlantKey(p game.PlantView, h uint64) (string, float64, bool) {
	return world.NewPlantKey(p, h)
}
func plantMarkColor(p *game.PlantView) (color.RGBA, float64) { return world.PlantMarkColor(p) }

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

func (r *Renderer) drawArt(screen *ebiten.Image, sw, sh int, tile float64) {
	r.worldView()
	r.pipeline().DrawArt(screen, sw, sh, tile)
}

func (r *Renderer) drawFloorLayers(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.worldView()
	r.pipeline().DrawFloorLayers(screen, w, sw, sh, tile)
}

func (r *Renderer) drawBuildingObj(dst *ebiten.Image, b *game.BuildingView, tile float64, sw, sh int) {
	r.worldView()
	r.pipeline().Ob.DrawBuildingObj(dst, b, tile, sw, sh)
}

func (r *Renderer) drawPlantObj(dst *ebiten.Image, w *game.WorldView, p *game.PlantView, tile float64) {
	r.worldView()
	r.pipeline().Ob.DrawPlantObj(dst, w, p, tile)
}

func (r *Renderer) drawPawnArt(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, sw, sh int) {
	r.worldView()
	r.pipeline().Ob.DrawPawnArt(screen, p, wx, wy, tile, sw, sh)
}

func (r *Renderer) drawForbiddenMark(screen *ebiten.Image, wx, wy int, tile float64, sw, sh int) {
	r.worldView()
	r.pipeline().Ob.DrawForbiddenMark(screen, wx, wy, tile, sw, sh)
}

func (r *Renderer) drawHostileMarker(screen *ebiten.Image, p game.PawnView, wx, wy, lift, tile float64) {
	r.pipe().Lb.DrawHostileMarker(screen, p, wx, wy, lift, tile)
}

func (r *Renderer) drawChatBubbles(screen *ebiten.Image, tile float64) {
	r.worldView()
	r.pipeline().Lb.DrawChatBubbles(screen, tile)
}

func (r *Renderer) drawSteamPuffs(screen *ebiten.Image, w game.WorldView, tile float64) {
	r.worldView()
	r.pipeline().Lb.DrawSteamPuffs(screen, w, tile)
}

func (r *Renderer) drawPuff(screen *ebiten.Image, f int, sx, sy float32, tile, k float64, cs colorScale) bool {
	return r.pipe().Lb.DrawPuff(screen, f, sx, sy, tile, k, cs)
}

func (r *Renderer) steamHiss(now uint64, x, y int) { r.pipeline().Lb.SteamHiss(now, x, y) }

func (r *Renderer) drawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs colorScale) {
	r.lightPrep()
	r.pipe().DrawTile(screen, key, wx, wy, tile, sw, sh, cs)
}

func (r *Renderer) withBake(dst *ebiten.Image, px float64, x0, y0 int, ox, oy float64, fn func(sw, sh int, tile float64)) {
	r.lightPrep()
	r.pipe().Bk.WithBake(dst, px, x0, y0, ox, oy, fn)
}

func (r *Renderer) drawBaked(screen, src *ebiten.Image, bx, by int, sx0, sy0, bpx float64, tx0, ty0, tx1, ty1 int) {
	r.pipe().Bk.DrawBaked(screen, src, bx, by, sx0, sy0, bpx, tx0, ty0, tx1, ty1)
}

func (r *Renderer) drawBakedColor(screen, src *ebiten.Image, bx, by int, sx0, sy0, bpx float64, tx0, ty0, tx1, ty1 int, col [4]float32) {
	r.pipe().Bk.DrawBakedColor(screen, src, bx, by, sx0, sy0, bpx, tx0, ty0, tx1, ty1, col)
}

func (r *Renderer) drawChunkFlat(screen, src *ebiten.Image, srcX, srcY, dstX, dstY, span, srcSpan float64, col [4]float32) {
	r.pipe().Bk.DrawChunkFlat(screen, src, srcX, srcY, dstX, dstY, span, srcSpan, col)
}

func (r *Renderer) terrainTileKey(t string, v int) (string, colorScale) {
	return r.pipe().TerrainTileKey(t, v)
}

func (r *Renderer) plantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return r.pipe().PlantsOn(plants, x0, y0, x1, y1, buf)
}

func (r *Renderer) chunkView(cx, cy int) game.ChunkView { return r.pipe().ChunkView(cx, cy) }

func (r *Renderer) animalPads(tile float64) (pad, padS int) { return world.AnimalPadsOf(r.art, tile) }

func (r *Renderer) stoneItem(key, kind string, cs colorScale) (string, colorScale) {
	return r.pipe().StoneItem(key, kind, cs)
}

func (r *Renderer) colonyCloths() []string { return r.pipe().ColonyCloths() }

func (r *Renderer) buildKey(kind, stuff, facing string, mask int, ew, open bool) string {
	return r.pipe().BuildKey(kind, stuff, facing, mask, ew, open)
}
