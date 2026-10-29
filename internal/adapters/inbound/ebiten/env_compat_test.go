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
	return r.envLayer().WindAt(r.envClock(), x, y)
}

func (r *Renderer) plantPose(x, y int, kind envpkg.PlantKind, amount, scale float64) motionpkg.Pose {
	return r.envLayer().PlantPose(r.envClock(), x, y, kind, amount, scale)
}

func (r *Renderer) drawGroundCover(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.envView()
	v.Date = r.game.Date()
	r.envLayer().DrawGroundCover(v, screen, w, sw, sh, tile, envHost{r}, envHost{r})
	v.Cam = frame
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

func (r *Renderer) drawGrassBatch(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh, limit int) {
	v, frame := r.envView()
	r.envLayer().DrawGrassBatch(v, dst, w, x0, y0, x1, y1, tile, sw, sh, limit, r.lod.Baking, envHost{r})
	v.Cam = frame
}

// drawCropObj queues a crop tile's units (env.Layer.DrawCropObj); per
// plant, so it reads the plain sprite view.
func (r *Renderer) drawCropObj(dst *ebiten.Image, w *game.WorldView, p *game.PlantView, tile float64) {
	v, frame := r.spriteView()
	r.envLayer().DrawCropObj(v, dst, w, p, tile)
	v.Cam = frame
}

func (r *Renderer) drawFurrows(dst *ebiten.Image, w game.WorldView, tile float64) {
	v, frame := r.envView()
	r.envLayer().DrawFurrows(v, dst, w, tile, r.frameViews(&w))
	v.Cam = frame
}

func (r *Renderer) treeCanopyAlpha(x, y int) float32 {
	return r.envLayer().Trees.TreeCanopyAlpha(r.envClock(), x, y, r.lod.Creatures)
}

func (r *Renderer) sweepTreeFades() { r.envLayer().Trees.SweepTreeFades(r.envClock()) }

func (r *Renderer) treePartKeys(key string) (trunk, canopy string, ok bool) {
	return env.TreePartKeys(r.envClock(), key)
}

func (r *Renderer) waterClock() float32 { return r.envLayer().Water.WaterClock(r.envClock()) }

func (r *Renderer) waterFrameKey(t string, x, y int) string {
	return r.envLayer().Water.WaterFrameKey(r.envClock(), t, x, y)
}

func (r *Renderer) drawWaterTile(screen *ebiten.Image, t string, x, y int, tile float64, sw, sh int, cs spritepkg.ColorScale) {
	v, frame := r.envView()
	r.envLayer().DrawWaterTile(v, screen, t, x, y, tile, sw, sh, cs, r.cast != nil, envHost{r})
	v.Cam = frame
}

func (r *Renderer) foamCS(x, y int) spritepkg.ColorScale {
	return r.envLayer().Water.FoamCS(r.envClock(), x, y)
}

func (r *Renderer) drawPuddleVeil(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.envView()
	r.envLayer().DrawPuddleVeil(v, screen, w, sw, sh, tile, envHost{r}, envHost{r})
	v.Cam = frame
}

func (r *Renderer) bakeSnowChunk(c *envpkg.SnowChunk, w game.WorldView, mw, mh, cx, cy int) {
	r.envLayer().BakeSnowChunk(c, w, mw, mh, cx, cy, envHost{r})
}
