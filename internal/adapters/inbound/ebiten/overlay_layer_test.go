package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	creaturepkg "rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// torchFireAt is where torch b's flame stands (its sprite's fire attachment).
func torchFireAt(c *art.Catalog, b *game.BuildingView) (float64, float64, bool) {
	return overlay.TorchFireAt(c, world.BuildSprite(overlaypkg.TorchLampKind, "", 0, false, false), b)
}

func turretAimVec(i int) creaturepkg.Vec {
	p := overlay.TurretAimVec(i)
	return creaturepkg.Vec{X: p.X, Y: p.Y}
}

func (r *Renderer) roofOverlayOn() bool { return r.overlays().Roof.Shown() }

func (r *Renderer) powerShown() bool { return r.overlays().Power.Shown() }

func (r *Renderer) drawZoneRun(b *gfx.TriBatch, kind string, run overlaypkg.EdgeRun, sw, sh int, tile float64) {
	v := r.liveView()
	r.overlays().Zones.DrawZoneRun(v, b, kind, run, sw, sh, tile)
}

func (r *Renderer) drawFenceRun(b *gfx.TriBatch, run overlaypkg.EdgeRun, tile float64) {
	v := r.liveView()
	r.overlays().Zones.DrawFenceRun(v, b, run, tile)
}

func (r *Renderer) zoneLabelPlanFor(z game.ZoneView, g *overlaypkg.ZoneGeom, tile float64) (plan overlay.ZoneLabelPlan, ox, oy float32, bw, bh float64) {
	v := r.liveView()
	return r.overlays().Zones.ZoneLabelPlanFor(v, z, g, tile)
}

func (r *Renderer) drawEaves(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.pipe().Env.SetView(r.liveView(), sw, sh, tile)
	v := r.liveView()
	r.overlays().DrawEaves(v, screen, w, sw, sh, tile)
}

func (r *Renderer) drawHomeOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.pipe().Env.SetView(r.liveView(), sw, sh, tile)
	v := r.liveView()
	r.overlays().DrawHomeOverlay(v, screen, w, sw, sh, tile)
}

func (r *Renderer) drawPlanOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.pipe().Env.SetView(r.liveView(), sw, sh, tile)
	v := r.liveView()
	r.overlays().DrawPlanOverlay(v, screen, w, sw, sh, tile)
}

func (r *Renderer) drawSnowClearOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.pipe().Env.SetView(r.liveView(), sw, sh, tile)
	v := r.liveView()
	r.overlays().DrawSnowClearOverlay(v, screen, w, sw, sh, tile)
}

func (r *Renderer) drawAreaOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.pipe().Env.SetView(r.liveView(), sw, sh, tile)
	v := r.liveView()
	r.overlays().DrawAreaOverlay(v, screen, w, r.game.Areas, sw, sh, tile)
}

func (r *Renderer) drawAreas(screen *ebiten.Image, w game.WorldView, areas []game.AreaView, sw, sh int, tile float64) {
	r.pipe().Env.SetView(r.liveView(), sw, sh, tile)
	v := r.liveView()
	r.overlays().DrawAreas(v, screen, w, areas, sw, sh, tile)
}

func (r *Renderer) drawFog(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	r.overlays().DrawFog(v, screen, w, sw, sh, tile)
}

func (r *Renderer) drawFogLayer(screen *ebiten.Image, fog []uint8, ver uint64, mw, mh int, tile float64, sw, sh int) {
	v := r.liveView()
	r.overlays().DrawFogLayer(v, screen, fog, ver, mw, mh, tile, sw, sh)
}

func (r *Renderer) drawStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	r.overlays().DrawStains(v, r.pipe().Pass(), screen, w, sw, sh, tile)
}

func (r *Renderer) buildStains(b *overlaypkg.StainBatch, w game.WorldView, sw, sh int, tile float64) int {
	v := r.liveView()
	return r.overlays().BuildStains(v, b, w, sw, sh, tile)
}

func (r *Renderer) drawDirt(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	r.overlays().DrawDirt(v, r.pipe().Pass(), screen, w, sw, sh, tile)
}

func (r *Renderer) buildDirt(b *overlaypkg.StainBatch, w game.WorldView, sw, sh int, tile float64) int {
	v := r.liveView()
	return r.overlays().BuildDirt(v, b, w, sw, sh, tile)
}

func (r *Renderer) drawFoundation(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	ch := r.pipe().OverlayChunks()
	r.overlays().DrawFoundation(v, r.pipe().Pass(), &ch, screen, w, sw, sh, tile)
}

func (r *Renderer) drawBridgeCell(dst *ebiten.Image, w game.WorldView, set map[[2]int]bool, b *game.BuildingView, tile float64, sw, sh int) {
	v := r.liveView()
	r.overlays().DrawBridgeCell(v, r.pipe().Pass(), dst, w, set, b, tile, sw, sh)
}

func (r *Renderer) drawArrivals(screen *ebiten.Image, w game.WorldView, tile float64) {
	v := r.liveView()
	r.overlays().DrawArrivals(v, screen, w, tile)
}

func (r *Renderer) drawWorldFires(screen *ebiten.Image, w game.WorldView, tile float64) {
	v := r.liveView()
	r.overlays().Fires.DrawWorldFires(v, screen, w, tile)
}

func (r *Renderer) drawAshStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	r.overlays().Fires.DrawAshStains(v, r.pipe().Pass(), screen, w, sw, sh, tile)
}

func (r *Renderer) drawConduits(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v := r.liveView()
	ch := r.pipe().OverlayChunks()
	r.overlays().Wiring.DrawConduits(v, r.pipe().Pass(), &ch, r.pipe().Links, screen, w, sw, sh, tile)
}

func (r *Renderer) drawPowerMarkings(dst *ebiten.Image, w *game.WorldView, tile float64, sw, sh int, colour func(net int) color.NRGBA, hiddenOnly bool) {
	v := r.liveView()
	ch := r.pipe().OverlayChunks()
	r.overlays().Wiring.DrawPowerMarkings(v, &ch, r.pipe().Links, dst, w, tile, sw, sh, colour, hiddenOnly)
}

func (r *Renderer) drawNoPowerBadges(screen *ebiten.Image, w *game.WorldView, tile float64) {
	v := r.liveView()
	ch := r.pipe().OverlayChunks()
	r.overlays().Wiring.DrawNoPowerBadges(v, &ch, screen, w, tile)
}

func (r *Renderer) bladeAt(b *game.BuildingView, baseKey, bladeK string) (float64, float64, bool) {
	v := r.liveView()
	return r.overlays().Wiring.BladeAt(v, b, baseKey, bladeK)
}

// drawZones draws the zones in the live frame view (render/overlay Zones).
func (r *Renderer) drawZones(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	p := r.pipe()
	ch := p.OverlayChunks()
	p.Overlays.Zones.DrawZones(r.liveView(), p.Pass(), &ch, screen, w, sw, sh, tile)
}

// drawRoofOverlay sets the visible rectangle and draws the Roof overlay,
// as Renderer.Draw does after the world.
func (r *Renderer) drawRoofOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	p := r.pipe()
	p.Env.SetView(r.liveView(), sw, sh, tile)
	p.Overlays.DrawRoofOverlay(r.liveView(), screen, w, r.game.Areas, sw, sh, tile)
}

// drawPowerOverlay draws the power overlay, as Renderer.Draw does.
func (r *Renderer) drawPowerOverlay(screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	p := r.pipe()
	ch := p.OverlayChunks()
	p.Overlays.DrawPowerOverlay(r.liveView(), &ch, p.Links, screen, w, sw, sh, tile)
}
