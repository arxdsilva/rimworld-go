package ebiten

import (
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	envpkg "rimworld-go/internal/adapters/inbound/ebiten/render/env"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
)

func buildCropAtlas(c *artpkg.Catalog, species []string) *envpkg.CropAtlas {
	return env.BuildCropAtlas(c, worldpkg.PlantKeys.PlantKeyTable, species)
}

func (r *Renderer) windAt(x, y float64) (vx, vy float64) {
	return r.pipe().Env.WindAt(r.liveView(), x, y)
}

func (r *Renderer) plantPose(x, y int, kind envpkg.PlantKind, amount, scale float64) motionpkg.Pose {
	return r.pipe().Env.PlantPose(r.liveView(), x, y, kind, amount, scale)
}

func (r *Renderer) drawGroundCover(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	v.Date = r.game.Date()
	r.pipe().Env.DrawGroundCover(v, screen, w, sw, sh, tile, envHost{r}, envHost{r})
}

// drawWeather draws falling rain or snow and the storm's darkening, then
// the sim's lightning (fire_world_draw.go showStrikes) and its flash.
func (r *Renderer) drawWeather(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	e := r.pipe().Env
	e.DrawWeather(v, screen, w, sw, sh, tile)
	r.overlays().Fires.ShowStrikes(r.liveView(), w)
	e.DrawLightning(v, screen, sw, sh)
}

func (r *Renderer) drawGrassBatch(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh, limit int) {
	v := r.liveView()
	r.pipe().Env.DrawGrassBatch(v, dst, w, x0, y0, x1, y1, tile, sw, sh, limit, r.pipe().LOD.Baking, envHost{r})
}

// drawCropObj queues a crop tile's units (env.Layer.DrawCropObj); per
// plant, so it reads the plain sprite view.
func (r *Renderer) drawCropObj(dst *ebiten.Image, w *game.WorldView, p *game.PlantView, tile float64) {
	v := r.liveView()
	r.pipe().Env.DrawCropObj(v, dst, w, p, tile)
}

func (r *Renderer) drawFurrows(dst *ebiten.Image, w game.WorldView, tile float64) {
	v := r.liveView()
	var views []game.ChunkView // the frame's chunk lists when w is the world they index
	if (envHost{r}).FrameLists(env.ListsOf(&w)) {
		views = r.pipe().LOD.Views
	}
	r.pipe().Env.DrawFurrows(v, dst, w, tile, views)
}

func (r *Renderer) treeCanopyAlpha(x, y int) float32 {
	return r.pipe().Env.Trees.TreeCanopyAlpha(r.liveView(), x, y, r.pipe().LOD.Creatures)
}

func (r *Renderer) sweepTreeFades() { r.pipe().Env.Trees.SweepTreeFades(r.liveView()) }

func (r *Renderer) treePartKeys(key string) (trunk, canopy string, ok bool) {
	return env.TreePartKeys(r.liveView(), key)
}

func (r *Renderer) waterClock() float32 { return r.pipe().Env.Water.WaterClock(r.liveView()) }

func (r *Renderer) waterFrameKey(t string, x, y int) string {
	return r.pipe().Env.Water.WaterFrameKey(r.liveView(), t, x, y)
}

func (r *Renderer) drawWaterTile(screen *ebiten.Image, t string, x, y int, tile float64, sw, sh int, cs spritepkg.ColorScale) {
	v := r.liveView()
	r.pipe().Env.DrawWaterTile(v, screen, t, x, y, tile, sw, sh, cs, r.cast != nil, envHost{r})
}

func (r *Renderer) foamCS(x, y int) spritepkg.ColorScale {
	return r.pipe().Env.Water.FoamCS(r.liveView(), x, y)
}

func (r *Renderer) drawPuddleVeil(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	r.pipe().Env.DrawPuddleVeil(v, screen, w, sw, sh, tile, envHost{r}, envHost{r})
}

func (r *Renderer) bakeSnowChunk(c *envpkg.SnowChunk, w game.WorldView, mw, mh, cx, cy int) {
	r.pipe().Env.BakeSnowChunk(c, w, mw, mh, cx, cy, envHost{r})
}
