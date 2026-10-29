package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	creaturepkg "rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// torchFireAt is where torch b's flame stands (its sprite's fire attachment).
func torchFireAt(c *art.Catalog, b *game.BuildingView) (float64, float64, bool) {
	return overlay.TorchFireAt(c, architect.BuildSprite(overlaypkg.TorchLampKind, "", 0, false, false), b)
}

func turretAimVec(i int) creaturepkg.Vec {
	p := overlay.TurretAimVec(i)
	return creaturepkg.Vec{X: p.X, Y: p.Y}
}

func (r *Renderer) roofOverlayOn() bool { return r.overlays().Roof.Shown() }

func (r *Renderer) powerShown() bool { return r.overlays().Power.Shown() }

func (r *Renderer) drawZoneRun(b *gfx.TriBatch, kind string, run overlaypkg.EdgeRun, sw, sh int, tile float64) {
	v, frame := r.partView()
	r.overlays().Zones.DrawZoneRun(v, b, kind, run, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawFenceRun(b *gfx.TriBatch, run overlaypkg.EdgeRun, tile float64) {
	v, frame := r.partView()
	r.overlays().Zones.DrawFenceRun(v, b, run, tile)
	v.Cam = frame
}

func (r *Renderer) zoneLabelPlanFor(z game.ZoneView, g *overlaypkg.ZoneGeom, tile float64) (plan overlay.ZoneLabelPlan, ox, oy float32, bw, bh float64) {
	v, frame := r.partView()
	defer func() { v.Cam = frame }()
	return r.overlays().Zones.ZoneLabelPlanFor(v, z, g, tile)
}

func (r *Renderer) drawEaves(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawEaves(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawHomeOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawHomeOverlay(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawPlanOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawPlanOverlay(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawSnowClearOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawSnowClearOverlay(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawAreaOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawAreaOverlay(v, screen, w, r.areasOf, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawAreas(screen *ebiten.Image, w game.WorldView, areas []game.AreaView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawAreas(v, screen, w, areas, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawFog(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawFog(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawFogLayer(screen *ebiten.Image, fog []uint8, ver uint64, mw, mh int, tile float64, sw, sh int) {
	v, frame := r.overlayView()
	r.overlays().DrawFogLayer(v, screen, fog, ver, mw, mh, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) drawStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawStains(v, r.spritePass(), screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) buildStains(b *overlaypkg.StainBatch, w game.WorldView, sw, sh int, tile float64) int {
	v, frame := r.overlayView()
	defer func() { v.Cam = frame }()
	return r.overlays().BuildStains(v, b, w, sw, sh, tile)
}

func (r *Renderer) drawDirt(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawDirt(v, r.spritePass(), screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) buildDirt(b *overlaypkg.StainBatch, w game.WorldView, sw, sh int, tile float64) int {
	v, frame := r.overlayView()
	defer func() { v.Cam = frame }()
	return r.overlays().BuildDirt(v, b, w, sw, sh, tile)
}

func (r *Renderer) drawFoundation(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().DrawFoundation(v, r.spritePass(), &ch, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawBridgeCell(dst *ebiten.Image, w game.WorldView, set map[[2]int]bool, b *game.BuildingView, tile float64, sw, sh int) {
	v, frame := r.partView()
	r.overlays().DrawBridgeCell(v, r.spritePass(), dst, w, set, b, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) drawArrivals(screen *ebiten.Image, w game.WorldView, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawArrivals(v, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) drawWorldFires(screen *ebiten.Image, w game.WorldView, tile float64) {
	v, frame := r.overlayView()
	r.overlays().Fires.DrawWorldFires(v, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) drawAshStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().Fires.DrawAshStains(v, r.spritePass(), screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawConduits(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Wiring.DrawConduits(v, r.spritePass(), &ch, &r.links, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawPowerMarkings(dst *ebiten.Image, w *game.WorldView, tile float64, sw, sh int, colour func(net int) color.NRGBA, hiddenOnly bool) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Wiring.DrawPowerMarkings(v, &ch, &r.links, dst, w, tile, sw, sh, colour, hiddenOnly)
	v.Cam = frame
}

func (r *Renderer) drawNoPowerBadges(screen *ebiten.Image, w *game.WorldView, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Wiring.DrawNoPowerBadges(v, &ch, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) bladeAt(b *game.BuildingView, baseKey, bladeK string) (float64, float64, bool) {
	v := r.lightView()
	return r.overlays().Wiring.BladeAt(v, b, baseKey, bladeK)
}
