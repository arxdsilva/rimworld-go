package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// The ground cover and weather live in render/env (ARCH-004). These
// aliases and one-line wrappers keep the root's call sites unchanged while
// the layers move out one package at a time; each mover repoints its own
// files to env and the last one deletes this file (SOLID_GO.md §11).

type (
	plantKind     = env.PlantKind
	envRect       = env.Rect
	envSpring     = env.Spring
	weatherDrop   = env.Drop
	dropKind      = env.DropKind
	grassBatch    = env.GrassBatch
	tuftCopy      = env.TuftCopy
	cropBatch     = env.CropBatch
	cropAtlas     = env.CropAtlas
	cropSprite    = env.CropSprite
	cropUnit      = env.CropUnit
	snowCache     = env.SnowCache
	snowChunk     = env.SnowChunk
	treeFx        = env.TreeFx
	waterRenderer = env.Water
	wetBake       = env.WetBake
)

const (
	kindGrass = env.KindGrass
	kindCrop  = env.KindCrop
	kindBush  = env.KindBush
	kindTree  = env.KindTree

	contactRadius  = env.ContactRadius
	contactViewPad = env.ContactViewPad

	cropVariants     = env.CropVariants
	cropWiltDamage   = env.CropWiltDamage
	cropMinShrink    = env.CropMinShrink
	cropJitterAcross = env.CropJitterAcross
	cropRootMax      = env.CropRootMax

	stSown        = env.StSown
	stSprout      = env.StSprout
	stMature      = env.StMature
	stHarvestable = env.StHarvestable
	stWilted      = env.StWilted
	numCropStages = env.NumCropStages

	dropRain   = env.DropRain
	dropSnow   = env.DropSnow
	dropSplash = env.DropSplash
	dropRipple = env.DropRipple

	duskGlowMax = env.DuskGlowMax

	grassCellJitter = env.GrassCellJitter
	grassEdgeSpill  = env.GrassEdgeSpill
	grassLiveMin    = env.GrassLiveMin
	grassMidCap     = env.GrassMidCap
	grassMarked     = env.GrassMarked

	lightningBucket = env.LightningBucket

	puddleBakeBudget = env.PuddleBakeBudget
	puddleRippleMin  = env.PuddleRippleMin
	wetBuckets       = env.WetBuckets

	snowChunkTiles  = env.SnowChunkTiles
	snowPx          = env.SnowPx
	snowPad         = env.SnowPad
	snowImgW        = env.SnowImgW
	snowEvictPasses = env.SnowEvictPasses

	steamVentKind     = env.SteamVentKind
	steamFrameTicks   = env.SteamFrameTicks
	steamFrames       = env.SteamFrames
	steamPeriodMin    = env.SteamPeriodMin
	steamPeriodSpread = env.SteamPeriodSpread

	treeTrunkSuffix  = env.TreeTrunkSuffix
	treeCanopySuffix = env.TreeCanopySuffix
	treeSeeThrough   = env.TreeSeeThrough

	waterFrameTicks = env.WaterFrameTicks
)

var (
	cropStageNames   = env.CropStageNames
	treeSpecies      = env.TreeSpecies
	grassShortKeys   = env.GrassShortKeys
	grassTallKeys    = env.GrassTallKeys
	grassDormantKeys = env.GrassDormantKeys
	grassGroundCode  = env.GrassGroundCode
	puddleGroundCode = env.PuddleGroundCode
	snowBankKeys     = env.SnowBankKeys
	waterKage        = env.WaterKage
)

// plantKeyTable is the plant sprite keys: the env layer's crops and trees
// (env.NewPlantKeys) and, in the same maps, the ground and biome plants
// (render/world biome_art.go); Ground is theirs.
type plantKeyTable = world.PlantKeyTable

var plantKeys = world.PlantKeys

func envTileKey(x, y int) uint32 { return env.TileKey(x, y) }

func cropStage(p game.PlantView) int       { return env.CropStage(p) }
func cropGrowthStage(p game.PlantView) int { return env.CropGrowthStage(p) }
func treeSpeciesAt(h uint64) int           { return env.TreeSpeciesAt(h) }

func cropFit(dy, above, below float64) (root, f float64) { return env.CropFit(dy, above, below) }
func cropUnitCount(species string) int                   { return env.CropUnitCount(species) }
func cropUnitLook(damage, rank, n int) (scale, alpha float64) {
	return env.CropUnitLook(damage, rank, n)
}
func cropUnits(x, y, n int, vertical bool, out []cropUnit) []cropUnit {
	return env.CropUnits(x, y, n, vertical, out)
}
func cropSwayAmount(species string) float64             { return env.CropSwayAmount(species) }
func cropSwayFactor(species string) float64             { return env.CropSwayFactor(species) }
func cropDamageTint(damage int, kind string) colorScale { return env.CropDamageTint(damage, kind) }
func cropMarkColor(p *game.PlantView) color.RGBA        { return env.CropMarkColor(p) }
func isCropPlant(p *game.PlantView) bool                { return env.IsCropPlant(p) }
func damageShadowFactor(p game.PlantView) float64       { return env.DamageShadowFactor(p) }
func plantDamageLook(p game.PlantView) (colorScale, float64) {
	return env.PlantDamageLook(p)
}
func plantDamageLine(p game.PlantView) string              { return env.PlantDamageLine(p) }
func plantDamageLabelAt(w game.WorldView, x, y int) string { return env.PlantDamageLabelAt(w, x, y) }

func buildCropAtlas(c *artCatalog, species []string) *cropAtlas {
	return env.BuildCropAtlas(c, plantKeys.PlantKeyTable, species)
}

func tuftCopies(growth uint8) int { return env.TuftCopies(growth) }
func grassCopies(x, y int, growth uint8, limit int, tallTile bool, out []tuftCopy) []tuftCopy {
	return env.GrassCopies(x, y, growth, limit, tallTile, out)
}
func grassTallAt(code uint8, x, y int) bool           { return env.GrassTallAt(code, x, y) }
func grassKeep(level string, x, y, n int) int         { return env.GrassKeep(level, x, y, n) }
func grassBakeKey(ver, sig uint64) uint64             { return env.GrassBakeKey(ver, sig) }
func grassMask(w game.WorldView, buf []uint8) []uint8 { return env.GrassMask(w, buf) }
func grassVisible(mask []uint8, w game.WorldView, t string, x, y int) bool {
	return env.GrassVisible(mask, w, t, x, y)
}

func snowCover(x, y int, depth float64) (string, float64) { return env.SnowCover(x, y, depth) }
func snowDrift(x, y int, depth float64) (string, float64) { return env.SnowDrift(x, y, depth) }
func valueNoise(fx, fy float64, salt uint64) float64      { return env.ValueNoise(fx, fy, salt) }
func frostStrength(tempTenths int, snow float64, d clock.Date) float64 {
	return env.FrostStrength(tempTenths, snow, d)
}
func lightningAt(kind string, intensity float64, b uint64) bool {
	return env.LightningAt(kind, intensity, b)
}
func weatherVeil(w game.WorldView) (veil, fog color.RGBA) { return env.WeatherVeil(w) }

func wetBucket(wet float64) uint8                     { return env.WetBucket(wet) }
func wetImgW(px int) int                              { return env.WetImgW(px) }
func wetPxFor(tile float64) int                       { return env.WetPxFor(tile) }
func puddleSalt(mw, mh int) uint64                    { return env.PuddleSalt(mw, mh) }
func puddleField(fx, fy float64, salt uint64) float64 { return env.PuddleField(fx, fy, salt) }
func puddleLevel(wet float64) float64                 { return env.PuddleLevel(wet) }
func puddleSky(l lightState) [4]float32               { return env.PuddleSky(l) }
func puddleShade(aw, dist, fade float64) (a, refl, cover float64) {
	return env.PuddleShade(aw, dist, fade)
}

func edgeMask(nbPri [4]int, match func(p int) bool) int { return env.EdgeMask(nbPri, match) }

func duskGlowStrength(p game.PlantView) float64 { return env.DuskGlowStrength(p) }
func plantLightLines(p game.PlantView) []string { return env.PlantLightLines(p) }

func treePartCoverageKeys() []string { return env.TreePartCoverageKeys() }

func steamPeriod(x, y int) int   { return env.SteamPeriod(x, y) }
func steamPhase(x, y int) uint64 { return env.SteamPhase(x, y) }
func steamPuff(now uint64, x, y int) (frame int, alpha float64, ok bool) {
	return env.SteamPuff(now, x, y)
}
func puffAt(t uint64) (frame int, alpha float64, ok bool) { return env.PuffAt(t) }
func ruinOverlayKey(x, y int) string                      { return env.RuinOverlayKey(x, y) }
func wantsRuinOverlay(b game.BuildingView) bool           { return env.WantsRuinOverlay(b) }
func ventLine(b game.BuildingView) string                 { return env.VentLine(b) }

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
		e.Keys = plantKeys.PlantKeyTable
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
func (r *Renderer) envView() (v *render.View, frame camera) {
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

func (h envHost) ChunkView(cx, cy int) game.ChunkView { return h.r.chunkView(cx, cy) }
func (h envHost) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return h.r.plantsOn(plants, x0, y0, x1, y1, buf)
}
func (h envHost) FrameLists(l env.Lists) bool { return h.r.frameLists(l) }
func (h envHost) Current(bldgs []game.BuildingView) bool {
	cur := h.r.game.World()
	return litFirst(cur.Buildings) == litFirst(bldgs) && len(cur.Buildings) == len(bldgs)
}
func (h envHost) DrawBakedColor(screen, src *ebiten.Image, bx, by int, sx0, sy0, bpx float64, tx0, ty0, tx1, ty1 int, col [4]float32) {
	h.r.drawBakedColor(screen, src, bx, by, sx0, sy0, bpx, tx0, ty0, tx1, ty1, col)
}
func (h envHost) DrawChunkFlat(screen, src *ebiten.Image, srcX, srcY, dstX, dstY, span, srcSpan float64, col [4]float32) {
	h.r.drawChunkFlat(screen, src, srcX, srcY, dstX, dstY, span, srcSpan, col)
}
func (h envHost) DrawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs colorScale) {
	h.r.drawTile(screen, key, wx, wy, tile, sw, sh, cs)
}
func (h envHost) DrawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps pose) {
	h.r.drawSpritePose(screen, key, tint, wx, wy, tile, ps)
}
func (h envHost) InView(x, y, pad, padS int) bool           { return h.r.lod.InView(x, y, pad, padS) }
func (h envHost) PawnMotion(id entity.EntityID) moveState   { return h.r.pawnMotion(id) }
func (h envHost) AnimalMotion(id entity.EntityID) moveState { return h.r.animalMotion(id) }

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

func (r *Renderer) windAt(x, y float64) (vx, vy float64) {
	return r.envLayer().WindAt(r.envClock(), x, y)
}

func (r *Renderer) plantPose(x, y int, kind plantKind, amount, scale float64) pose {
	return r.envLayer().PlantPose(r.envClock(), x, y, kind, amount, scale)
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

func (r *Renderer) drawGroundCover(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.envView()
	v.Date = r.game.Date()
	r.envLayer().DrawGroundCover(v, screen, w, sw, sh, tile, envHost{r}, envHost{r})
	v.Cam = frame
}

func (r *Renderer) groundOpen(w game.WorldView, x, y int) bool {
	return r.envLayer().GroundOpen(w, x, y)
}

// drawWeather draws falling rain or snow and the storm's darkening, then
// the sim's lightning (fire_world_draw.go showStrikes) and its flash.
func (r *Renderer) drawWeather(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.envView()
	e := r.envLayer()
	e.DrawWeather(v, screen, w, sw, sh, tile)
	r.showStrikes(w)
	e.DrawLightning(v, screen, sw, sh)
	v.Cam = frame
}

func (r *Renderer) grassFrameMask(w *game.WorldView, x0, y0, x1, y1 int) {
	r.envLayer().GrassFrameMask(w, x0, y0, x1, y1, envHost{r})
}

func (r *Renderer) drawGrassBatch(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh, limit int) {
	v, frame := r.envView()
	r.envLayer().DrawGrassBatch(v, dst, w, x0, y0, x1, y1, tile, sw, sh, limit, r.lod.Baking, envHost{r})
	v.Cam = frame
}

func (r *Renderer) grassReach(x0, y0, x1, y1 int) (int, int, int, int) {
	v, frame := r.envView()
	x0, y0, x1, y1 = r.envLayer().GrassReach(v, x0, y0, x1, y1)
	v.Cam = frame
	return x0, y0, x1, y1
}

func (r *Renderer) grassBatchOf(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh, limit int) {
	v, frame := r.envView()
	r.envLayer().GrassBatchOf(v, dst, w, x0, y0, x1, y1, tile, sw, sh, limit, r.lod.Baking)
	v.Cam = frame
}

func (r *Renderer) drawGrassCutMarks(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh int) {
	v, frame := r.envView()
	r.envLayer().DrawGrassCutMarks(v, r.spritePass(), dst, w, x0, y0, x1, y1, tile, sw, sh)
	v.Cam = frame
}

// grassBakesTufts reports the tier whose chunk images hold the tufts.
func (r *Renderer) grassBakesTufts() bool {
	return r.lod.Tier == lodChunk && r.tilePx() < grassLiveMin
}

func (r *Renderer) grassChunkSig(w *game.WorldView, cx, cy int) uint64 {
	v, frame := r.envView()
	out := r.envLayer().GrassChunkSig(v, w, cx, cy, envHost{r})
	v.Cam = frame
	return out
}

func (r *Renderer) grassLookSig(w *game.WorldView, h uint64) uint64 {
	return r.envLayer().GrassLookSig(w, h)
}

// bakeTufts draws the tufts reaching into tiles x0..x1-1 × y0..y1-1 into
// dst, a chunk bake at px per tile with tile (x0, y0) at (ox, oy): still,
// unlit, at most grassMidCap copies a tile.
func (r *Renderer) bakeTufts(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, px, ox, oy float64) {
	r.withBake(dst, px, x0, y0, ox, oy, func(sw, sh int, tile float64) {
		r.drawGrassBatch(dst, w, x0, y0, x1-1, y1-1, tile, sw, sh, grassMidCap)
	})
}

func (r *Renderer) grassMapSig(w *game.WorldView, cx, cy int, ver uint64) uint64 {
	v, frame := r.envView()
	out := r.envLayer().GrassMapSig(v, w, cx, cy, ver, r.lod.Chunks.Frame)
	v.Cam = frame
	return out
}

func (r *Renderer) bakeGrassTint(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64) {
	v, frame := r.envView()
	r.envLayer().BakeGrassTint(v, dst, w, x0, y0, x1, y1, tile, r.lod.WhiteImg())
	v.Cam = frame
}

// drawCropObj queues a crop tile's units (env.Layer.DrawCropObj); per
// plant, so it reads the plain sprite view.
func (r *Renderer) drawCropObj(dst *ebiten.Image, w *game.WorldView, p *game.PlantView, tile float64) {
	v, frame := r.spriteView()
	r.envLayer().DrawCropObj(v, dst, w, p, tile)
	v.Cam = frame
}

func (r *Renderer) flushCrops(dst *ebiten.Image) { r.envLayer().FlushCrops(dst) }

func (r *Renderer) drawLitBatch(dst *ebiten.Image, verts []ebiten.Vertex, idx []uint32, src *ebiten.Image) {
	r.envLayer().DrawLitBatch(dst, verts, idx, src)
}

func (r *Renderer) drawFurrows(dst *ebiten.Image, w game.WorldView, tile float64) {
	v, frame := r.envView()
	r.envLayer().DrawFurrows(v, dst, w, tile, r.frameViews(&w))
	v.Cam = frame
}

func (r *Renderer) drawDuskGlow(screen *ebiten.Image, w game.WorldView, tile float64) {
	v, frame := r.envView()
	r.envLayer().DrawDuskGlow(v, screen, w, tile, r.lod.Tier == lodLive, r.lod.Views)
	v.Cam = frame
}

// SetTreeFocus lists the tiles of the hovered/selected entity or the
// Architect ghost; canopies over them turn see-through too. Replaces
// the previous list; call with nil to clear.
func (r *Renderer) SetTreeFocus(cells [][2]int) { r.envLayer().Trees.SetTreeFocus(cells) }

func (r *Renderer) treeCanopyAlpha(x, y int) float32 {
	return r.envLayer().Trees.TreeCanopyAlpha(r.envClock(), x, y, r.lod.Creatures)
}

func (r *Renderer) sweepTreeFades() { r.envLayer().Trees.SweepTreeFades(r.envClock()) }

func (r *Renderer) treePartKeys(key string) (trunk, canopy string, ok bool) {
	return env.TreePartKeys(r.envClock(), key)
}

func (r *Renderer) waterShader() *ebiten.Shader { return r.envLayer().Water.WaterShader() }

func (r *Renderer) waterClock() float32 { return r.envLayer().Water.WaterClock(r.envClock()) }

func (r *Renderer) waterFrameKey(t string, x, y int) string {
	return r.envLayer().Water.WaterFrameKey(r.envClock(), t, x, y)
}

func (r *Renderer) drawWaterTile(screen *ebiten.Image, t string, x, y int, tile float64, sw, sh int, cs colorScale) {
	v, frame := r.envView()
	r.envLayer().DrawWaterTile(v, screen, t, x, y, tile, sw, sh, cs, r.cast != nil, envHost{r})
	v.Cam = frame
}

func (r *Renderer) drawWaterEdges(screen *ebiten.Image, t string, nbPri [4]int, x, y int, tile float64, sw, sh int) {
	v, frame := r.envView()
	r.envLayer().DrawWaterEdges(v, r.spritePass(), screen, t, nbPri, terrainPriority["shallow_water"], x, y, tile, sw, sh, envHost{r})
	v.Cam = frame
}

func (r *Renderer) foamCS(x, y int) colorScale { return r.envLayer().Water.FoamCS(r.envClock(), x, y) }

func (r *Renderer) drawPuddleVeil(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.envView()
	r.envLayer().DrawPuddleVeil(v, screen, w, sw, sh, tile, envHost{r}, envHost{r})
	v.Cam = frame
}

func (r *Renderer) puddleAt(fx, fy float64) float64 { return r.envLayer().SnowC.PuddleAt(fx, fy) }

func (r *Renderer) bakeSnowChunk(c *snowChunk, w game.WorldView, mw, mh, cx, cy int) {
	r.envLayer().BakeSnowChunk(c, w, mw, mh, cx, cy, envHost{r})
}

func (r *Renderer) onScreenTile(screen *ebiten.Image, x, y int, tile float64) bool {
	v, frame := r.envView()
	out := env.OnScreenTile(v, screen, x, y, tile)
	v.Cam = frame
	return out
}

func (r *Renderer) drawRuinOverlay(dst *ebiten.Image, b *game.BuildingView, tile float64, sw, sh int) {
	v, frame := r.envView()
	r.envLayer().DrawRuinOverlay(v, r.spritePass(), dst, b, tile, sw, sh)
	v.Cam = frame
}
